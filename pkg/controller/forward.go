package controller

import (
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Templum/Spediteur/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

func NewForwardHandler(conf *config.ForwardProxyConfig) *ForwardHandler {
	var pool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, conf.Proxy.BufferSizes.Read)
			return buf
		},
	}

	return &ForwardHandler{pool: &pool, conf: conf}
}

type ForwardHandler struct {
	pool *sync.Pool
	conf *config.ForwardProxyConfig
}

func (h *ForwardHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	// TODO: Check against whitelist
	domain, lookup := getDomainName(ctx)
	log.Debugf("Domain Lookup yielded %s and %s", domain, lookup)

	// time.ParseDuration is already called during validation, hence an error is impossible at this location
	deadlineDuration, _ := time.ParseDuration(h.conf.Proxy.Timeouts.Write)
	deadline := time.Now().Add(deadlineDuration)

	if ctx.IsConnect() {
		log.Debugf("received connect for %s", domain)
		h.Tunnel(ctx, deadline)
	} else {
		log.Debugf("received proxy for %s", domain)
		h.Proxy(ctx, deadline)
	}
}

func (h *ForwardHandler) Tunnel(ctx *fasthttp.RequestCtx, deadline time.Time) {
	// time.ParseDuration is already called during validation, hence an error is impossible at this location
	t, _ := time.ParseDuration(h.conf.Proxy.Timeouts.Connect)
	dest, err := net.DialTimeout("tcp", string(ctx.Host()), t)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusServiceUnavailable)
		return
	}

	ctx.Hijack(func(origin net.Conn) {
		var wg sync.WaitGroup
		wg.Add(2)

		defer dest.Close()
		defer origin.Close()

		_ = dest.SetDeadline(deadline)
		_ = origin.SetDeadline(deadline)

		go h.transfer(dest, origin, &wg)
		go h.transfer(origin, dest, &wg)

		wg.Wait()
	})
}

func (h *ForwardHandler) Proxy(ctx *fasthttp.RequestCtx, deadline time.Time) {
	c := fasthttp.Client{}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := c.DoDeadline(&ctx.Request, resp, deadline)
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

func getDomainName(ctx *fasthttp.RequestCtx) (string, string) {
	host := string(ctx.Request.Host())

	// If present remove port
	dotIdx := strings.IndexRune(host, ':')
	if dotIdx > 0 {
		host = host[:dotIdx]
	}

	// If ParseIP return nil it is very likely a domain. Or worstcase an malformed IP that anyways would fail during lookup
	if net.ParseIP(host) == nil {
		return host, ""
	} else {
		domains, err := net.LookupAddr(host)
		if err != nil {
			log.Warnf("Reverse IP lookup failed with: %s", err)
			return host, ""
		}

		return host, domains[0]
	}
}
