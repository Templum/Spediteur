package controller

import (
	"bytes"
	"io"
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

func NewForwardHandler() *ForwardHandler {
	return &ForwardHandler{}
}

type ForwardHandler struct {
}

func (h *ForwardHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {

	if bytes.Equal(ctx.Method(), []byte("CONNECT")) {
		h.Proxy(ctx)
	} else {
		h.Proxy(ctx)
	}
}

func (h *ForwardHandler) Tunnel(ctx *fasthttp.RequestCtx) {
	dest, err := net.DialTimeout("tcp", string(ctx.Host()), 10*time.Second) // TODO: Configurable
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusServiceUnavailable)
		return
	}

	ctx.Hijack(func(origin net.Conn) {
		go transfer(dest, origin)
		go transfer(origin, dest)
	})
}

func (h *ForwardHandler) Proxy(ctx *fasthttp.RequestCtx) {
	c := fasthttp.Client{}

	resp := fasthttp.AcquireResponse()
	err := c.Do(&ctx.Request, resp)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusServiceUnavailable)
		return
	}

	ctx.SetStatusCode(resp.StatusCode())
	ctx.SetBody(resp.Body())
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	_, _ = io.Copy(destination, source)
}
