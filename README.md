# OKProxy

Simple proxy to be used as a handler with your favorite `http.Handler` compatible mux. Uses Go's httputil.NewSingleHostReverseProxy internally.

## Usage
OKProxy exposes methods `PathRequestProxyHandler` and `PayloadRequestProxyHandler`. The former allows for the creation of a router from a path and the latter is just a convenient way to forward a request to the specified URL in a JSON body.

#### PathRequestProxyHandler(path string, errorHandler ErrorHandler) http.Handler
```go
package main

import (
	"log"
	"net/http"
	"github.com/tomascaslo/ok-proxy"
)

type application struct{}

func (app *application) ServerErrorHandler(w http.ResponseWriter, r *http.Request, err error) { return }

func main() {
   app := &application{}
   proxy := okproxy.New("localhost:3000")
   http.Handle("/forward", proxy.PathRequestProxyHandler("/forward", app)) 
   
   log.Fatal(http.ListenAndServe(":8080", nil))
}
```

#### PayloadRequestProxyHandler(errorHandler ErrorHandler) http.Handler
```go
package main

import (
	"log"
	"net/http"
	"github.com/tomascaslo/ok-proxy"
)

type application struct{}

func (app *application) ServerErrorHandler(w http.ResponseWriter, r *http.Request, err error) { return }

func main() {
   app := &application{}
   proxy := okproxy.New("")
   http.Handle("/forward", proxy.PayloadRequestProxyHandler(app)) 
   
   log.Fatal(http.ListenAndServe(":8080", nil))
}
```

