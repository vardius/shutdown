⏲️ shutdown
================
[![Build Status](https://travis-ci.org/vardius/shutdown.svg?branch=master)](https://travis-ci.org/vardius/shutdown)
[![Go Report Card](https://goreportcard.com/badge/github.com/vardius/shutdown)](https://goreportcard.com/report/github.com/vardius/shutdown)
[![codecov](https://codecov.io/gh/vardius/shutdown/branch/master/graph/badge.svg)](https://codecov.io/gh/vardius/shutdown)
[![](https://godoc.org/github.com/vardius/shutdown?status.svg)](http://godoc.org/github.com/vardius/shutdown)
[![license](https://img.shields.io/github/license/mashape/apistatus.svg)](https://github.com/vardius/shutdown/blob/master/LICENSE.md)

<img align="right" height="180px" src="https://github.com/vardius/gorouter/blob/master/website/src/static/img/logo.png?raw=true" alt="logo" />

shutdown - Simple shutdown signals handler with callback

📖 ABOUT
==================================================
Contributors:

* [Rafał Lorenz](http://rafallorenz.com)

Want to contribute ? Feel free to send pull requests!

Have problems, bugs, feature ideas?
We are using the github [issue tracker](https://github.com/vardius/shutdown/issues) to manage them.

## 📚 Documentation

For __examples__ **visit [godoc#pkg-examples](http://godoc.org/github.com/vardius/shutdown#pkg-examples)**

For **GoDoc** reference, **visit [godoc.org](http://godoc.org/github.com/vardius/shutdown)**

🚏 HOW TO USE
==================================================

## 🏫 Basic example
```go
package main

import (
    "context"
    "net/http"
    "os"
    "time"
    "log"

    "github.com/vardius/shutdown"
)

func main() {
    ctx := context.Background()
    
    http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
        io.WriteString(w, "Hello!\n")
    })

    stop := func() {
        ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()

        log.Printf("shutting down...\n")

        if err := srv.Shutdown(ctx); err != nil {
            log.Printf("shutdown error: %v\n", err)
        } else {
            log.Printf("gracefully stopped\n")
        }
    }

    go func() {
        log.Printf("%v\n", http.ListenAndServe(":8080", nil))
        stop()
        os.Exit(1)
    }()
}
```

📜 [License](LICENSE.md)
-------

This package is released under the MIT license. See the complete license in the package
