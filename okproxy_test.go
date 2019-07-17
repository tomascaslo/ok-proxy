package okproxy

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type mockErrorHandler struct {
	called bool
}

func (meh *mockErrorHandler) ServerErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	meh.called = true
}

type mockReverseProxy struct {
	URL   string
	calls []string
}

func (mrp *mockReverseProxy) SetProxyURL(url string) {
	mrp.URL = url
}

func (mrp *mockReverseProxy) GetProxyURL() string {
	return mrp.URL
}

func (mrp *mockReverseProxy) serveReverseProxy(http.ResponseWriter, *http.Request, ErrorHandler) {
	mrp.calls = append(mrp.calls, "serveReverseProxy")
}

func (mrp *mockReverseProxy) decodeURLFromBody(r *http.Request, errorHandler ErrorHandler) error {
	mrp.calls = append(mrp.calls, "decodeURLFromBody")
	return nil
}

func TestPathRequestProxyHandler(t *testing.T) {
	tests := []struct {
		name                            string
		w                               *httptest.ResponseRecorder
		r                               *http.Request
		okProxy                         *OKProxy
		path                            string
		errorHandler                    *mockErrorHandler
		expectedURLPath                 string
		expectedErrorCalled             bool
		expectedServeReverseProxyCalled bool
	}{
		{
			"Trims path and forward request",
			httptest.NewRecorder(),
			httptest.NewRequest("GET", "/forward/api", nil),
			&OKProxy{&mockReverseProxy{"127.0.0.1:8080", []string{}}},
			"/forward",
			&mockErrorHandler{},
			"/api",
			false,
			true,
		},
		{
			"Errors on empty proxy URL",
			httptest.NewRecorder(),
			httptest.NewRequest("GET", "/forward/api", nil),
			&OKProxy{&mockReverseProxy{}},
			"",
			&mockErrorHandler{},
			"/forward/api",
			true,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathRequestProxyHandler := tt.okProxy.PathRequestProxyHandler(tt.path, tt.errorHandler)

			pathRequestProxyHandler.ServeHTTP(tt.w, tt.r)

			if tt.r.URL.Path != tt.expectedURLPath {
				t.Errorf("Expected %q got %q", tt.expectedURLPath, tt.r.URL.Path)
			}

			if tt.errorHandler.called != tt.expectedErrorCalled {
				t.Errorf("Expected %t got %t", tt.expectedErrorCalled, tt.errorHandler.called)
			}

			mpr, ok := tt.okProxy.proxy.(*mockReverseProxy)
			if !ok {
				t.Error("Error in *mockReverseProxy assert")
			}
			if stringInSlice("serveReverseProxy", mpr.calls) != tt.expectedServeReverseProxyCalled {
				t.Errorf("Expected %s to be in calls %v", "serveReverseProxy", mpr.calls)
			}
		})
	}
}

func TestPayloadRequestProxyHandler(t *testing.T) {
	tests := []struct {
		name                            string
		w                               *httptest.ResponseRecorder
		r                               *http.Request
		okProxy                         *OKProxy
		errorHandler                    *mockErrorHandler
		expectedURLPath                 string
		expectedErrorHandlerCalled      bool
		expectedServeReverseProxyCalled bool
	}{
		{
			"Sets proxy url and forwards request",
			httptest.NewRecorder(),
			httptest.NewRequest("GET", "/forward/api", bytes.NewReader([]byte(`{"proxyURL":"127.0.0.1:8080"}`))),
			&OKProxy{&mockReverseProxy{"127.0.0.1:8080", []string{}}},
			&mockErrorHandler{},
			"/forward/api",
			false,
			true,
		},
		{
			"Errors on decoding",
			httptest.NewRecorder(),
			httptest.NewRequest("GET", "/forward/api", bytes.NewReader([]byte(`{"proxyURL":"127.0.0.1:8080"`))),
			&OKProxy{&mockReverseProxy{}},
			&mockErrorHandler{},
			"/forward/api",
			true,
			false,
		},
		{
			"Errors on empty proxy",
			httptest.NewRecorder(),
			httptest.NewRequest("GET", "/forward/api", bytes.NewReader([]byte(`{"proxyURL":"127.0.0.1:8080"}`))),
			&OKProxy{&mockReverseProxy{}},
			&mockErrorHandler{},
			"/forward/api",
			true,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathRequestProxyHandler := tt.okProxy.PayloadRequestProxyHandler(tt.errorHandler)

			pathRequestProxyHandler.ServeHTTP(tt.w, tt.r)

			if tt.r.URL.Path != tt.expectedURLPath {
				t.Errorf("Expected %q got %q", tt.expectedURLPath, tt.r.URL.Path)
			}

			if tt.errorHandler.called != tt.expectedErrorHandlerCalled {
				t.Errorf("Expected %t got %t", tt.expectedErrorHandlerCalled, tt.errorHandler.called)
			}

			mpr, ok := tt.okProxy.proxy.(*mockReverseProxy)
			if !ok {
				t.Error("Error in *mockReverseProxy assert")
			}
			if stringInSlice("serveReverseProxy", mpr.calls) != tt.expectedServeReverseProxyCalled {
				t.Errorf("Expected %s to be in calls %v", "serveReverseProxy", mpr.calls)
			}
		})
	}
}

func TestServeReverseProxy(t *testing.T) {
	type url struct {
		host   string
		scheme string
	}
	type requestData struct {
		headerForwardedHost string
		host                string
	}

	createRequest := func(includeHost bool) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		if includeHost {
			r.Header.Set("Host", "127.0.0.1:8080")
		}
		return r
	}
	tests := []struct {
		name                       string
		w                          *httptest.ResponseRecorder
		r                          *http.Request
		proxy                      *reverseProxy
		errorHandler               *mockErrorHandler
		expectedURL                *url
		expectedRequestData        *requestData
		expectedErrorHandlerCalled bool
	}{
		{
			"Updates URL and request data",
			httptest.NewRecorder(),
			createRequest(true),
			&reverseProxy{"https://127.0.0.1:8080"},
			&mockErrorHandler{},
			&url{"127.0.0.1:8080", "https"},
			&requestData{"127.0.0.1:8080", "127.0.0.1:8080"},
			false,
		},
		{
			"Errors on url parse",
			httptest.NewRecorder(),
			createRequest(false),
			&reverseProxy{"http\ns://6876826^@30"},
			&mockErrorHandler{},
			&url{},
			&requestData{"", "example.com"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.proxy.serveReverseProxy(tt.w, tt.r, tt.errorHandler)

			actualURL := &url{tt.r.URL.Host, tt.r.URL.Scheme}
			if !reflect.DeepEqual(actualURL, tt.expectedURL) {
				t.Errorf("Expected %v got %v", tt.expectedURL, actualURL)
			}

			actualRequestData := &requestData{tt.r.Header.Get("X-Forwarded-Host"), tt.r.Host}
			if !reflect.DeepEqual(actualRequestData, tt.expectedRequestData) {
				t.Errorf("Expected %v got %v", tt.expectedRequestData, actualRequestData)
			}

			if tt.errorHandler.called != tt.expectedErrorHandlerCalled {
				t.Errorf("Expected %t got %t", tt.expectedErrorHandlerCalled, tt.errorHandler.called)
			}
		})
	}
}

func TestDecodeURLFromBody(t *testing.T) {
	tests := []struct {
		name          string
		r             *http.Request
		url           string
		errorHandler  *mockErrorHandler
		expectedProxy *OKProxy
		expectedBody  []byte
		expectedError error
	}{
		{
			"Unmarshals body",
			httptest.NewRequest("GET", "/", bytes.NewReader([]byte(`{"proxyURL":"127.0.0.1:8080"}`))),
			"127.0.0.1:8080",
			&mockErrorHandler{},
			&OKProxy{&reverseProxy{"127.0.0.1:8080"}},
			[]byte(`{"proxyURL":"127.0.0.1:8080"}`),
			nil,
		},
		{
			"Errors on json unmarshal",
			httptest.NewRequest("GET", "/", bytes.NewReader([]byte(`{"proxyURL":"127.0.0.1:8080}`))),
			"127.0.0.1:8080",
			&mockErrorHandler{},
			&OKProxy{&reverseProxy{"127.0.0.1:8080"}},
			[]byte(`{"proxyURL":"127.0.0.1:8080}`),
			errors.New("unexpected end of JSON input"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &OKProxy{&reverseProxy{tt.url}}
			err := p.proxy.decodeURLFromBody(tt.r, tt.errorHandler)

			if err != nil && err.Error() != tt.expectedError.Error() {
				t.Errorf("Expected %q got %q", tt.expectedError.Error(), err.Error())
			}

			if p.proxy.GetProxyURL() != tt.expectedProxy.proxy.GetProxyURL() {
				t.Errorf("Expected %q got %q", "127.0.0.1:8080", p.proxy.GetProxyURL())
			}

			body, err := ioutil.ReadAll(tt.r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(body, tt.expectedBody) {
				t.Errorf("Expected %v got %v", string(tt.expectedBody), string(body))
			}
		})
	}

}

func stringInSlice(s string, sl []string) bool {
	for _, el := range sl {
		if el == s {
			return true
		}
	}
	return false
}
