package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Templum/Spediteur/pkg/controller"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

func init() {
	log.SetLevel(log.DebugLevel)
	go func() {
		_ = http.ListenAndServe(":18080", nil) // TODO: Configurable
	}()
}

type server struct {
	HTTPServer *fasthttp.Server
}

func newServer() *server {
	handler := controller.NewForwardHandler()

	// TODO: Make most options configurable and allign the buffers specifically
	http := &fasthttp.Server{
		Handler: handler.HandleFastHTTP,
		ErrorHandler: func(ctx *fasthttp.RequestCtx, err error) {
		},
		DisableKeepalive:                   false,
		ReadBufferSize:                     16 * 1024,
		WriteBufferSize:                    16 * 1024,
		ReadTimeout:                        30 * time.Second,
		WriteTimeout:                       30 * time.Second,
		MaxConnsPerIP:                      0,
		MaxRequestsPerConn:                 0,
		MaxKeepaliveDuration:               0,
		TCPKeepalive:                       false,
		TCPKeepalivePeriod:                 0,
		MaxRequestBodySize:                 0,
		LogAllErrors:                       false,
		SleepWhenConcurrencyLimitsExceeded: 0,
		Logger:                             log.New(),
		KeepHijackedConns:                  false,
	}

	return &server{
		HTTPServer: http,
	}
}

func main() {
	// TODO: Get configuration via CLI

	server := newServer()
	ln, err := reuseport.Listen("tcp4", "localhost:8888") // TODO: Make configurable
	if err != nil {
		log.Warnf("error in reuseport listener: %s", err)
		os.Exit(1)
	}

	go func() {
		err := server.HTTPServer.Serve(ln)
		if err != nil {
			log.Warnf("error in serve: %s", err)
			os.Exit(1)
		}
	}()

	log.Infof("Server is listening under %s", ln.Addr())

	// Listening for relevant signals from os indicating shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs
	log.Infof("Shutdown signal %s received.", sig)

	if err := server.HTTPServer.Shutdown(); err != nil {
		log.Warnf("error during shutdown: %s", err)
	}

	log.Info("Server gracefully stopped.")
}
