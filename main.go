// Package OKProxy provides a simple proxy using httputil.NewSingleHostReverseProxy.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// OKProxy is the main struct and embeds a ProxyReverser.
type OKProxy struct {
	proxy ProxyReverser
}

// reverseProxy stores the proxy URL and access methods.
type reverseProxy struct {
	URL string `json:"proxyURL"`
}

type ProxyReverser interface {
	SetProxyURL(string)
	GetProxyURL() string
	serveReverseProxy(http.ResponseWriter, *http.Request, ErrorHandler)
	decodeURLFromBody(r *http.Request, errorHandler ErrorHandler) error
}

// ErrorHandler interface that can be passed into proxy handlers.
type ErrorHandler interface {
	ServerErrorHandler(http.ResponseWriter, *http.Request, error)
}

// New allocates a new OKProxy and reverseProxy with empty URL string.
func New(URL string) *OKProxy {
	return &OKProxy{&reverseProxy{URL}}
}

// PathRequestProxyHandler allows the creation of a proxy for the specified path.
// errorHandler interface must be passed for error handling.
// path is always trimmed from the actual r.URL.Path before proxing the request.
// e.g. on path=/forward, /forward/api -> /api
func (p *OKProxy) PathRequestProxyHandler(path string, errorHandler ErrorHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.proxy.GetProxyURL() == "" {
			errorHandler.ServerErrorHandler(w, r, errors.New("ProxyURL needs to be set for PathRequestProxyHandler"))
			return
		}
		// Remove the path from router before sending request to url
		r.URL.Path = strings.TrimPrefix(r.URL.Path, path)

		p.proxy.serveReverseProxy(w, r, errorHandler)
	})
}

// PaylodRequesProxyHandler allows the creation of a proxy from the value of the
// proxyURL field in a JSON body.
// errorHandler interface must be passed for error handling.
func (p *OKProxy) PayloadRequestProxyHandler(errorHandler ErrorHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := p.proxy.decodeURLFromBody(r, errorHandler)
		if err != nil {
			errorHandler.ServerErrorHandler(w, r, err)
			return
		}
		if p.proxy.GetProxyURL() == "" {
			errorHandler.ServerErrorHandler(w, r, errors.New("ProxyURL needs to be set in request body at proxyURL field"))
			return
		}
		p.proxy.serveReverseProxy(w, r, errorHandler)
	})
}

func (rp *reverseProxy) SetProxyURL(url string) {
	rp.URL = url
}

func (rp *reverseProxy) GetProxyURL() string {
	return rp.URL
}

// serveReverseProxy is the main function in charge of creating the
// reverse proxy from httputil.NewSingleHostReverseProxy and forwarding
// the request.
func (rp *reverseProxy) serveReverseProxy(w http.ResponseWriter, r *http.Request, errorHandler ErrorHandler) {
	url, err := url.Parse(rp.GetProxyURL())
	if err != nil {
		errorHandler.ServerErrorHandler(w, r, err)
		return
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(url)

	r.URL.Host = url.Host
	r.URL.Scheme = url.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = url.Host

	proxy.ServeHTTP(w, r)
}

// decodeURLFromBody reads the request body and unmarshals it into a rp.
// Resets r.Body so that it can be reads from other handlers.
// Errors when body is not valid JSON syntax.
func (rp *reverseProxy) decodeURLFromBody(r *http.Request, errorHandler ErrorHandler) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	err = json.Unmarshal(body, rp)
	if err != nil {
		return err
	}

	return nil
}
