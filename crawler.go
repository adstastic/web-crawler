package main

import (
    "fmt"
    "strings"
    "flag"
    "os"
    "sync"
    "net/http"
    "net/url"
    "golang.org/x/net/html"
    "crypto/tls"
    "encoding/json"
    "strconv"
    "time"
    "bufio"
)

type Crawler struct {
    Sitemap map[string]*Page // indexed links and assets of every page crawled
    Deferred map[string]string // contains deferred url and parents, for when server starts blocking requests
    Mutex sync.Mutex // mutex for accessing maps
    Sem chan bool // semaphore channel to limit concurrent goroutines
    Root string
    MaxRoutines int
    wg sync.WaitGroup
    Running bool
}

type Page struct {
    Assets map[string]bool // links for assets
    Links map[string]bool // links to other pages on same domain
    Parent string // parent url from where this was accessed
}

func TlsConfig() (http.Client) {
    // create http client which skips tls verification for easy https parsing
    tlsConfig := &tls.Config {                 
        InsecureSkipVerify: true,  
    }                                                                  
    transport := &http.Transport{
        TLSClientConfig: tlsConfig,    
    }                                
    return http.Client{Transport: transport}
}

func GetUri(client http.Client, uri string) (bool, *http.Response) {
    time.Sleep(time.Millisecond * 300)
    // gets page from uri, returns boolean for success/failure 
    req, reqErr := http.NewRequest("GET", uri, nil)
    req.Close = true
    if reqErr != nil {
        fmt.Print("Request Error: ")
        fmt.Println(reqErr)
        return false, nil
    }
    req.Header.Add("Connection", "close")
    req.Header.Add("Accept-Encoding", "identity")

    res, resErr := client.Do(req)  
    if resErr != nil {
        fmt.Print("Request Error: ")
        fmt.Println(resErr)
        return false, nil
    }
    return true, res
}

func (c *Crawler) SanitizeUri(href string) (string) {
    // converts relative uri into absolute uri using base 
    uri, err := url.Parse(href)
    if err != nil {
        return ""
    }
    baseUri, err := url.Parse(c.Root)
    if err != nil {
        return ""
    }
    uri = baseUri.ResolveReference(uri)
    uriStr := uri.String()
    if strings.Index(uriStr, "?") != -1 {
        uriStr = strings.Split(uriStr, "?")[0]
    }
    if strings.Index(uriStr, "#") != -1 {
        uriStr = strings.Split(uriStr, "#")[0]
    }
    return uriStr
}

func (c *Crawler) SameDomain (link string) (bool) {
    // strips protocol from uri for comparison 
    // so both https and http version of domain are supported
    switch {
    case strings.HasPrefix(link, "http://www."):
        link = strings.SplitAfter(link, "http://www.")[1]
    case strings.HasPrefix(link, "https://www."):
        link = strings.SplitAfter(link, "https://www.")[1]
    case strings.HasPrefix(link, "http://"):
        link = strings.SplitAfter(link, "http://")[1]
    case strings.HasPrefix(link, "https://"):
        link = strings.SplitAfter(link, "https://")[1]
    case strings.HasPrefix(link, "www."):
        link = strings.SplitAfter(link, "www.")[1]
    }
    base := strings.SplitAfter(c.Root, "www.")[1]
    return strings.HasPrefix(link, base)
}

func (c *Crawler) ExtractLinks(res *http.Response) (map[string]bool, map[string]bool) {
    // extracts all static assets from the page
    // searches html tokens for fields "src" and "rel"
    assets := make(map[string]bool)
    links := make(map[string]bool)
    tokenizer := html.NewTokenizer(res.Body)
    defer res.Body.Close()
    for {
        switch tokenizer.Next() {
        case html.ErrorToken:
            return assets, links
        case html.StartTagToken:
            token := tokenizer.Token()
            for _, attribute := range token.Attr {
                key := attribute.Key
                if (key == "src" || key == "rel") {
                    uri := c.SanitizeUri(attribute.Val)
                    assets[uri] = true
                } else if (key == "href") {
                    uri := c.SanitizeUri(attribute.Val)
                    if c.SameDomain(uri) {
                        links[uri] = true
                    }
                }
            }
        }
    }     
} 

func (c *Crawler) Crawl(uri, parent string) {
    defer c.wg.Done() // signals waitgroup on termination
    <- c.Sem // wait for semaphore
    client := TlsConfig()
    ret, response := GetUri(client, uri)
    if ret && c.Running {
        fmt.Println("Crawling " + uri)
        assets, links := c.ExtractLinks(response)
        page := Page{Assets: assets, Links: links, Parent: parent}
        for link,_ := range page.Links {
            c.Mutex.Lock()
            if c.Sitemap[link] == nil {
                fmt.Println("Visiting " + link)
                c.wg.Add(1)
                go c.Crawl(link, uri)
                c.Sitemap[uri] = &page
            }
            c.Mutex.Unlock()
        }
        fmt.Println("Indexed " + uri)
    } else {
        // unable to crawl uri, save for later
        c.Running = false
        if c.Deferred[uri] == "" && c.Sitemap[uri] == nil {
            fmt.Println("Request failed, deferring crawl of " + uri)
            c.Deferred[uri] = parent
        }
    }
    c.Sem <- true // release semaphore
}

func (c *Crawler) Start() {
    c.Running = true
    if &c.wg == nil || &c.Root == nil || c.MaxRoutines == 0 {
        panic("Crawler fields not set.")
    }
    c.Sem = make(chan bool, c.MaxRoutines)
    // populate semaphore channel 
    for i := 0; i < c.MaxRoutines; i++ {
        c.Sem <- true
    }
    
    c.wg.Add(1)
    go c.Crawl(c.Root, "")
    c.wg.Wait() // wait until all goroutines have finished
}

func (c *Crawler) crawlDeferred() {
    c.Running = true
    fmt.Print("Deferred urls: ")
    fmt.Println(len(c.Deferred))
    if len(c.Deferred) > 0 {
        for url, parent := range c.Deferred {
            c.wg.Add(1)
            go c.Crawl(url, parent)
            c.wg.Wait()
        }
    }
}

func main() {
    flag.Parse()
    args := flag.Args()
    if len(args) < 2 {
        fmt.Println("Please both enter a URL and a maximum concurrency value.")
        os.Exit(1)
    }

    fmt.Println("Maximum concurrency: " + args[1])
    max,_ := strconv.Atoi(args[1])

    crawler := Crawler{Sitemap: make(map[string]*Page), 
                        Deferred: make(map[string]string), 
                        Root: args[0], 
                        MaxRoutines: max}
    crawler.Start()
    fmt.Println("Taking a break...")
    time.Sleep(time.Second * 5) // wait till (hopefully) server timeout has passed
    // resume crawling the deferred list of urls
    crawler.crawlDeferred()
    
    out, _ := json.MarshalIndent(crawler.Sitemap, "", "\t")
    // if save location specified
    if args[2] != "" {
        if _, err := os.Stat(args[2]); err == nil {
            fname := strings.SplitAfter(crawler.Root, "www.")[1]
            f, err := os.Create(args[2] + "/" + fname + ".json")
            w := bufio.NewWriter(f)
            nb, err := w.WriteString(string(out))
            if err != nil {
                fmt.Print("Write error: ")
                fmt.Println(err)
            }
            w.Flush()
            fmt.Println("wrote %d bytes\n", nb)
        } else {
            fmt.Println(args[2] + " doesn't exist.")
        }
    } else {
        fmt.Println("Sitemap")
        fmt.Println(string(out))
    }

}