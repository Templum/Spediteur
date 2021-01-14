package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Templum/Spediteur/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

func startHTTPSTestEndpoint(handler http.Handler) (*httptest.Server, *x509.CertPool) {
	srv := httptest.NewTLSServer(handler)

	certpool := x509.NewCertPool()
	certpool.AddCert(srv.Certificate())

	return srv, certpool
}

func startHTTPTestEndpoint(handler http.Handler) *httptest.Server {
	srv := httptest.NewServer(handler)
	return srv
}

func TestForwardHandler_HandleFastHTTP(t *testing.T) {
	conf := config.ForwardProxyConfig{Proxy: config.Proxy{Timeouts: config.Timeouts{Connect: "30s", Write: "30s"}, BufferSizes: config.BufferSizes{Read: 1024, Write: 1024}}}
	proxyURL, _ := url.Parse("http://mysuperproxy:18080")

	connectTests := []struct {
		name string

		method   string
		body     []byte
		upstream func(w http.ResponseWriter, r *http.Request)

		expectErr     bool
		wantedMessage string

		wantedStatusCode int
		wantedBody       []byte
	}{
		{
			name: "[connect request] getting from successful returning endpoint with small body", method: http.MethodGet, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = io.WriteString(w, "<html><body>Hello World!</body></html>")
			},
			expectErr:        false,
			wantedStatusCode: 200,
			wantedBody:       []byte("<html><body>Hello World!</body></html>"),
		},
		{
			name: "[connect request] deleting from unsuccessful returning endpoint with no body", method: http.MethodDelete, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
				_, _ = w.Write(nil)
			},
			expectErr:        false,
			wantedStatusCode: 404,
			wantedBody:       []byte{},
		},
		{
			name: "[connect request] posting from successful returning endpoint with small body", method: http.MethodPost, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(201)
				_, _ = io.WriteString(w, "<html><body>Hello World!</body></html>")
			},
			expectErr:        false,
			wantedStatusCode: 201,
			wantedBody:       []byte("<html><body>Hello World!</body></html>"),
		},
		{
			name: "[connect request] putting from successful returning endpoint with small body", method: http.MethodPut, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(202)
				_, _ = io.WriteString(w, "<html><body>Hello World!</body></html>")
			},
			expectErr:        false,
			wantedStatusCode: 202,
			wantedBody:       []byte("<html><body>Hello World!</body></html>"),
		},
		{
			name: "[connect request] patching from unsuccessful returning endpoint with no body", method: http.MethodPut, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				_, _ = w.Write(nil)
			},
			expectErr:        false,
			wantedStatusCode: 500,
			wantedBody:       []byte{},
		},
	}

	for _, tt := range connectTests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewForwardHandler(&conf)
			ln := fasthttputil.NewInmemoryListener()
			defer ln.Close()

			go func() {
				err := fasthttp.Serve(ln, h.HandleFastHTTP)
				assert.NoError(t, err, "should not throw err")
			}()

			srv, certpool := startHTTPSTestEndpoint(http.HandlerFunc(tt.upstream))
			defer srv.Close()

			client := &http.Client{Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return ln.Dial()
				},
				TLSClientConfig: &tls.Config{RootCAs: certpool}, // This ensures our client works with test server
			}}

			req, _ := http.NewRequest(tt.method, srv.URL, bytes.NewReader(tt.body))
			resp, err := client.Do(req)

			if resp != nil {
				defer resp.Body.Close()
			}

			if tt.expectErr {
				assert.Error(t, err, "should throw error")
				assert.Contains(t, err.Error(), tt.wantedMessage)
			} else {
				actualBody, bodyReadErr := ioutil.ReadAll(resp.Body)

				assert.NoError(t, bodyReadErr, "should not fail reading body")
				assert.NoError(t, err, "should not throw error")
				assert.EqualValues(t, tt.wantedStatusCode, resp.StatusCode)
				assert.EqualValues(t, tt.wantedBody, actualBody)
			}

		})
	}

	forwardTests := []struct {
		name string

		method   string
		body     []byte
		upstream func(w http.ResponseWriter, r *http.Request)

		expectErr     bool
		wantedMessage string

		wantedStatusCode int
		wantedBody       []byte
	}{
		{
			name: "[forwarding] getting from successful returning endpoint with small body", method: http.MethodGet, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = io.WriteString(w, "<html><body>Hello World!</body></html>")
			},
			expectErr:        false,
			wantedStatusCode: 200,
			wantedBody:       []byte("<html><body>Hello World!</body></html>"),
		},
		{
			name: "[forwarding] deleting from unsuccessful returning endpoint with no body", method: http.MethodDelete, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
				_, _ = w.Write(nil)
			},
			expectErr:        false,
			wantedStatusCode: 404,
			wantedBody:       []byte{},
		},
		{
			name: "[forwarding] posting from successful returning endpoint with small body", method: http.MethodPost, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(201)
				_, _ = io.WriteString(w, "<html><body>Hello World!</body></html>")
			},
			expectErr:        false,
			wantedStatusCode: 201,
			wantedBody:       []byte("<html><body>Hello World!</body></html>"),
		},
		{
			name: "[forwarding] putting from successful returning endpoint with small body", method: http.MethodPut, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(202)
				_, _ = io.WriteString(w, "<html><body>Hello World!</body></html>")
			},
			expectErr:        false,
			wantedStatusCode: 202,
			wantedBody:       []byte("<html><body>Hello World!</body></html>"),
		},
		{
			name: "[forwarding] patching from unsuccessful returning endpoint with no body", method: http.MethodPut, body: nil,
			upstream: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				_, _ = w.Write(nil)
			},
			expectErr:        false,
			wantedStatusCode: 500,
			wantedBody:       []byte{},
		},
	}

	for _, tt := range forwardTests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewForwardHandler(&conf)
			ln := fasthttputil.NewInmemoryListener()
			defer ln.Close()

			go func() {
				err := fasthttp.Serve(ln, h.HandleFastHTTP)
				assert.NoError(t, err, "should not throw err")
			}()

			srv := startHTTPTestEndpoint(http.HandlerFunc(tt.upstream))
			defer srv.Close()

			client := &http.Client{Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return ln.Dial()
				},
			}}

			req, _ := http.NewRequest(tt.method, srv.URL, bytes.NewReader(tt.body))
			resp, err := client.Do(req)

			if resp != nil {
				defer resp.Body.Close()
			}

			if tt.expectErr {
				assert.Error(t, err, "should throw error")
				assert.Contains(t, err.Error(), tt.wantedMessage)
			} else {
				actualBody, bodyReadErr := ioutil.ReadAll(resp.Body)

				assert.NoError(t, bodyReadErr, "should not fail reading body")
				assert.NoError(t, err, "should not throw error")
				assert.EqualValues(t, tt.wantedStatusCode, resp.StatusCode)
				assert.EqualValues(t, tt.wantedBody, actualBody)
			}

		})
	}
}
