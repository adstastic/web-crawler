# Web Crawler

A web crawler written in Go. Only crawls pages with the same base URL as the one specified. Outputs site map as `json`.

## Usage

```bash
go run crawler.go [url] [num_goroutines] [save_dir]
```
`url` must be full, including protocol e.g. "http://google.com" or "http://www.google.com"
`num_goroutines` is the maximum number of concurrent network accesses the crawler will use. 
`save_dir` is the only optional argument of the above, if specified the sitemap is written to file at the location

Tests: `go test`

## Further Info

The script is verbose so will output every URL it crawls, visits, and defers.

Deferring a URL (not used in the same context as `defer` in Go) is implemented because it was found that having many threads concurrently accessing the same webserver eventually triggers a timeout (which manifests as EOF or host not found or various other errors in the HTTP Get request). When this happens, all goroutines that haven't crawled their URL yet add it to a deferred list and stop bombarding the server with requests. After all the goroutines finish, there's a timeout (currently 5s), then the deferred list is picked up again for crawling.

One step to increase concurrency is running `sudo ulimit -n 1000` so your local machine stops being part of the bottleneck. Beyond this, its up to the server.