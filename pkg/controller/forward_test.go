package controller

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
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

func TestForwardHandler_HandleFastHTTP(t *testing.T) {
	conf := config.ForwardProxyConfig{Proxy: config.Proxy{Timeouts: config.Timeouts{Connect: "30s", Write: "30s"}, BufferSizes: config.BufferSizes{Read: 1024, Write: 1024}}}
	proxyURL, _ := url.Parse("http://localhost:18080")

	t.Run("connect request to reachable endpoint returning html", func(t *testing.T) {
		h := NewForwardHandler(&conf)
		ln := fasthttputil.NewInmemoryListener()
		defer ln.Close()

		go func() {
			err := fasthttp.Serve(ln, h.HandleFastHTTP)
			assert.NoError(t, err, "should not throw err")
		}()

		client := &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return ln.Dial()
			},
		}}

		req, _ := http.NewRequest(http.MethodGet, "https://google.de", nil)
		resp, err := client.Do(req)

		assert.NoError(t, err, "should not throw err")
		assert.EqualValues(t, resp.StatusCode, 200, "should return 200")
	})

	t.Run("connect request to unreachable endpoint", func(t *testing.T) {
		h := NewForwardHandler(&conf)
		ln := fasthttputil.NewInmemoryListener()
		defer ln.Close()

		go func() {
			err := fasthttp.Serve(ln, h.HandleFastHTTP)
			assert.NoError(t, err, "should not throw err")
		}()

		client := &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return ln.Dial()
			},
		}}

		req, _ := http.NewRequest(http.MethodGet, "https://nonexisting-endpoint.de", nil)
		resp, err := client.Do(req)

		assert.Nil(t, resp, "should not return body")
		assert.Error(t, err, "should throw an error")
		assert.Contains(t, err.Error(), "Service Unavailable")
	})

	t.Run("forward call", func(t *testing.T) {

	})
}
