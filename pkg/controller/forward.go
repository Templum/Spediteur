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
			return &buf
		},
	}
	// time.ParseDuration is already called during validation, hence an error is impossible at this location
	d, _ := time.ParseDuration(conf.Proxy.Timeouts.Write)
	t, _ := time.ParseDuration(conf.Proxy.Timeouts.Connect)

	return &ForwardHandler{pool: &pool, conf: conf, connectTimeout: t, deadlineDuration: d}
}

type ForwardHandler struct {
	pool *sync.Pool
	conf *config.ForwardProxyConfig

	connectTimeout time.Duration
	deadlineDuration time.Duration
}

func (h *ForwardHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	// TODO: Check against whitelist
	domain, lookup := getDomainName(ctx)
	log.Debugf("Domain Lookup yielded %s and %s", domain, lookup)

	deadline := time.Now().Add(h.deadlineDuration)

	if ctx.IsConnect() {
		log.Debugf("received connect for %s", domain)
		h.Tunnel(ctx, deadline)
	} else {
		log.Debugf("received proxy for %s", domain)
		h.Proxy(ctx, deadline)
	}
}

func (h *ForwardHandler) Tunnel(ctx *fasthttp.RequestCtx, deadline time.Time) {
	
	dest, err := fasthttp.DialTimeout(string(ctx.Host()), h.connectTimeout)
	if err != nil {
		log.Errorf("tunnel: failed to reach target host %s due to %s", ctx.Host(), err)
		ctx.Error("could not reach upstream server", fasthttp.StatusServiceUnavailable)
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
	// Eventually would make sense to have a pool of fasthttp clients, although the target upstream are unlikely always the same
	c := fasthttp.Client{}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := c.DoDeadline(&ctx.Request, resp, deadline)
	if err != nil {
		log.Warnf("Received %s during forwarding", err)
		ctx.Error("could not reach upstream server", fasthttp.StatusServiceUnavailable)
		return
	}

	ctx.SetStatusCode(resp.StatusCode())
	ctx.SetBody(resp.Body())
}

func clearSlice(pool *sync.Pool, b *[]byte) {
	// CLearing slice while protecting length
	*b = (*b)[:cap(*b)]

	pool.Put(b)
}

func (h *ForwardHandler) transfer(destination io.Writer, source io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()

	buf := h.pool.Get().(*[]byte)
	defer clearSlice(h.pool, buf)

	_, err := io.CopyBuffer(destination, source, *buf)
	if err != nil {
		log.Warnf("Received %s during proxying", err)
	}
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
