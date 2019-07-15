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

type OKProxy struct {
	proxy ProxyReverser
}

type reverseProxy struct{
	URL string `json:"proxyURL"`
}

type ProxyReverser interface {
	SetProxyURL(string)
	GetProxyURL() string
	serveReverseProxy(http.ResponseWriter, *http.Request, ErrorHandler)
}

type ErrorHandler interface {
	ServerErrorHandler(http.ResponseWriter, *http.Request, error)
}

func New() *OKProxy {
	return &OKProxy{&reverseProxy{""}}
}

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

func (p *OKProxy) PayloadRequestProxyHandler(errorHandler ErrorHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := p.decodeURLFromBody(r, errorHandler)
		if err != nil {
            errorHandler.ServerErrorHandler(w, r, err)
            return
		}
		p.proxy.serveReverseProxy(w, r, errorHandler)
	})
}

func (rp *reverseProxy) SetProxyURL(url string)  {
	rp.URL = url
}

func (rp *reverseProxy) GetProxyURL() string {
	return rp.URL
}

func (pr *reverseProxy) serveReverseProxy(w http.ResponseWriter, r *http.Request, errorHandler ErrorHandler) {
	url, err := url.Parse(pr.GetProxyURL())
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

func (p *OKProxy) decodeURLFromBody(r *http.Request, errorHandler ErrorHandler) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	err = json.Unmarshal(body, p)
	if err != nil {
		return err
	}

	return nil
}

