package controller

import (
	"io"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

func NewForwardHandler() *ForwardHandler {
	var pool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 16*1024) // TODO: Configurable
			return buf
		},
	}

	return &ForwardHandler{pool: &pool}
}

type ForwardHandler struct {
	pool *sync.Pool
}

func (h *ForwardHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {

	host := ctx.Request.Host()
	// TODO: Check against whitelist

	if ctx.IsConnect() {
		log.Debugf("received connect for %s", host)
		h.Tunnel(ctx)
	} else {
		log.Debugf("received proxy for %s", host)
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
		var wg sync.WaitGroup
		wg.Add(2)

		defer dest.Close()
		defer origin.Close()

		go h.transfer(dest, origin, &wg)
		go h.transfer(origin, dest, &wg)

		wg.Wait()
	})
}

func (h *ForwardHandler) Proxy(ctx *fasthttp.RequestCtx) {
	c := fasthttp.Client{}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := c.Do(&ctx.Request, resp)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusServiceUnavailable)
		return
	}

	ctx.SetStatusCode(resp.StatusCode())
	ctx.SetBody(resp.Body())
}

func clearSlice(pool *sync.Pool, b []byte) {
	b = b[:cap(b)]

	//lint:ignore SA6002 as we are using slices here
	pool.Put(b)
}

func (h *ForwardHandler) transfer(destination io.Writer, source io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()

	buf := h.pool.Get().([]byte)
	defer clearSlice(h.pool, buf)

	_, _ = io.CopyBuffer(destination, source, buf)
}
