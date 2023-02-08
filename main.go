package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sapcc/go-bits/httpapi"
)

var (
	addr             string
	configpath       string
	logLevel         string
	namespace        string
	netboxHost       string
	netboxToken      string
	promUrl          string
	query            string
	region           string
	logger           log.Logger
	discoverInterval time.Duration
	updateInterval   time.Duration
)

func main() {
	ctx := cancelCtxOnSigterm(context.Background())
	fq, err := NewFilerQueue(promUrl)
	if err != nil {
		logFatal(err)
	}

	go func() {
		info(fmt.Sprintf("update filer states every %s", updateInterval))
		ticker := time.NewTicker(updateInterval)
		defer ticker.Stop()

		for {
			err = fq.UpdateState()
			if err != nil {
				logError(err)
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				os.Exit(0)
			}
		}
	}()

	go func() {
		info(fmt.Sprintf("discover new filers every %s", discoverInterval))
		ticker := time.NewTicker(discoverInterval)
		defer ticker.Stop()

		for {
			err = fq.DiscoverFilers(region, query)
			if err != nil {
				logError(err)
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				os.Exit(0)
			}
		}
	}()

	info(fmt.Sprintf("config and template dir: %s", configpath))
	info(fmt.Sprintf("starting server at address %s", addr))
	srv := &http.Server{
		Handler: httpapi.Compose(fq),
		Addr:    addr,
	}
	logFatal(srv.ListenAndServe())

}

func init() {
	flag.StringVar(&addr, "address", "0.0.0.0:8000", "server address")
	flag.StringVar(&query, "query", "", "query")
	flag.StringVar(&region, "region", "", "region")
	flag.StringVar(&netboxHost, "netbox-host", "netbox.global.cloud.sap", "netbox host")
	flag.StringVar(&netboxToken, "netbox-api-token", "", "netbox token")
	flag.StringVar(&configpath, "config-dir", "./", "Directory where config and template files are located")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.DurationVar(&discoverInterval, "discover-interval", 5*time.Minute, "time interval between dicovering filers from netbox")
	flag.DurationVar(&updateInterval, "update-interval", 3*time.Minute, "time interval between state updates from prometheus")
	flag.Parse()

	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)

	switch strings.ToLower(logLevel) {
	case "info":
		logger = level.NewFilter(logger, level.AllowInfo())
	case "debug":
		logger = level.NewFilter(logger, level.AllowDebug())
	case "warn":
		logger = level.NewFilter(logger, level.AllowWarn())
	case "error":
		logger = level.NewFilter(logger, level.AllowError())
	}

	if region == "" {
		level.Error(logger).Log("msg", "region must be specified")
		os.Exit(1)
	}
	promUrl = os.Getenv("NETAPPSD_PROMETHEUS_URL")
	if promUrl == "" {
		logFatal("env variable NETAPPSD_PROMETHEUS_URL not set")
	}
}
