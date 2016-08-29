package main

import (
    "testing"
)

var client = TlsConfig()

func TestGetUriSuccess(t* testing.T) {
    ret,_ := GetUri(client,"http://www.google.com")
    if !ret {
        t.Error("GetUri does not return for valid URI")
    }
}

func TestGetUriFail(t* testing.T) {
    ret,_ := GetUri(client,"www.google.com")
    if ret {
        t.Error("GetUri returns for invalid URI")
    }
}

func TestCrawlerStartFail(t* testing.T) {
    c := Crawler{}
    defer func() {
        if recover() == nil {
            t.Error("Crawler starts without required fields")
        }
    }()
    c.Start()
}

func TestCrawlerStop(t* testing.T) {
    c := Crawler{Sitemap: make(map[string]*Page), 
                        Deferred: make(map[string]string), 
                        Root: "http://www.tomblomfield.com", 
                        MaxRoutines: 500}
    c.Start()
    c.Running = false
    if len(c.Sem) != 500 {
        t.Error("Crawler not using all goroutines required/specified")
    }
}
