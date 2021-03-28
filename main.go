package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/Templum/Spediteur/pkg/config"
	"github.com/Templum/Spediteur/pkg/controller"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"

	_ "go.uber.org/automaxprocs"
)

var (
	confPath string
	logLevel uint
)

func init() {

	var defaultPath string
	dir, err := os.Getwd()
	if err != nil {
		defaultPath = ""
	} else {
		defaultPath = path.Join(dir, "hack", "default.yaml")
	}

	flag.StringVar(&confPath, "confPath", defaultPath, "used to specify which config should be used to configure the proxy")
	flag.UintVar(&logLevel, "logLevel", 1, "used to specify log level. Where 0=debug 1=info 2=warn 3=error 4=fatal 5=panic")
	flag.Parse()

	switch logLevel {
	case 0:
		log.SetLevel(log.DebugLevel)
	case 1:
		log.SetLevel(log.InfoLevel)
	case 2:
		log.SetLevel(log.WarnLevel)
	case 3:
		log.SetLevel(log.ErrorLevel)
	case 4:
		log.SetLevel(log.ErrorLevel)
	case 5:
		log.SetLevel(log.FatalLevel)
	}

	log.Infof("Spediteur will be using config at %s and Log Level is set to %d", confPath, logLevel)
}

type server struct {
	HTTPServer *fasthttp.Server
}

func newServer(conf *config.ForwardProxyConfig) *server {
	handler := controller.NewForwardHandler(conf)

	// time.ParseDuration is already called during validation, hence an error is impossible at this location
	readTimeout, _ := time.ParseDuration(conf.Proxy.Timeouts.Read)

	// time.ParseDuration is already called during validation, hence an error is impossible at this location
	writeTimeout, _ := time.ParseDuration(conf.Proxy.Timeouts.Write)

	http := &fasthttp.Server{
		Handler: handler.HandleFastHTTP,
		ErrorHandler: func(ctx *fasthttp.RequestCtx, err error) {
			log.Errorf("Following error %s was raised during parsing incoming request %s", err, ctx.Path())
		},
		DisableKeepalive:                   false,
		ReadBufferSize:                     conf.Proxy.BufferSizes.Read,
		WriteBufferSize:                    conf.Proxy.BufferSizes.Write,
		ReadTimeout:                        readTimeout,
		WriteTimeout:                       writeTimeout,
		MaxConnsPerIP:                      conf.Proxy.Limits.MaxConnsPerIP,
		IdleTimeout:                        readTimeout,
		MaxRequestBodySize:                 conf.Proxy.Limits.MaxBodySize,
		LogAllErrors:                       false,
		SleepWhenConcurrencyLimitsExceeded: 0,
		Logger:                             log.New(),
		KeepHijackedConns:                  false,
	}

	return &server{
		HTTPServer: http,
	}
}

func startMonitoringServer(conf *config.ForwardProxyConfig) {
	port := ":" + strconv.Itoa(int(conf.Monitoring.Port))
	err := http.ListenAndServe(port, nil)
	log.Warnf("Failed to start monitoring server under %s due to %s", strconv.Itoa(int(conf.Monitoring.Port)), err)
}

func startForwardProxyServer(server *server, conf *config.ForwardProxyConfig) {
	address := conf.Proxy.Server + ":" + strconv.Itoa(int(conf.Proxy.Port))
	ln, err := reuseport.Listen("tcp4", address)
	if err != nil {
		log.Fatalf("Error during creating listener for %s: %s", address, err)
	}

	err = server.HTTPServer.Serve(ln)
	if err != nil {
		log.Fatalf("Error during setup of http server: %s", err)
	}
}

func main() {
	file, err := os.Open(confPath)
	if err != nil {
		log.Fatalf("Failed reading config at %s due to %s", confPath, err)
	}

	conf, err := config.New(file)
	if err != nil {
		log.Fatalf("Failed while parsing provided config due to %s", err)
	}

	server := newServer(conf)

	go startMonitoringServer(conf)
	go startForwardProxyServer(server, conf)

	log.Infof("Spediteur started Metric server under :%d and Proxy Server under %s:%d", conf.Monitoring.Port, conf.Proxy.Server, conf.Proxy.Port)

	// Listening for relevant signals from os indicating shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Spediteur is now listening for SIGTERM & SIGINT signals to perform gracefully shutdown")

	sig := <-sigs
	log.Infof("Shutdown signal %s received.", sig)

	if err := server.HTTPServer.Shutdown(); err != nil {
		log.Warnf("Error during shutdown: %s", err)
	}

	log.Info("Server gracefully stopped.")
}
