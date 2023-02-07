package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sapcc/go-bits/httpapi"
)

var (
	query            string
	region           string
	namespace        string
	configpath       string
	netboxHost       string
	netboxToken      string
	logLevel         string
	logger           log.Logger
	discoverInterval int64
)

func main() {
	promUrl := os.Getenv("NETAPPSD_PROMETHEUS_URL")
	if promUrl == "" {
		logFatal("env variable NETAPPSD_PROMETHEUS_URL not set")
	}
	fq, err := NewFilerQueue(promUrl)
	if err != nil {
		logFatal(err)
	}

	ctx := cancelCtxOnSigterm(context.Background())
	updateStateInterval := 300 * time.Second
	cnt := 0

	go func() {
		ticker := time.NewTicker(updateStateInterval)
		defer ticker.Stop()

		for {
			err = fq.UpdateState()
			if err != nil {
				logError(err)
			}
			select {
			case <-ticker.C:
				cnt += 1
			case <-ctx.Done():
				os.Exit(0)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(time.Duration(discoverInterval) * time.Second)
		stopTicker := func(code int) int {
			defer ticker.Stop()
			return code
		}
		defer ticker.Stop()

		for {
			err = fq.DiscoverFilers(region, query)
			if err != nil {
				logError(err)
				os.Exit(stopTicker(1))
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				os.Exit(0)
			}
		}
	}()

	srv := &http.Server{
		Handler: httpapi.Compose(fq),
		Addr:    "127.0.0.1:8000",
	}
	logFatal(srv.ListenAndServe())

}

func init() {
	flag.StringVar(&query, "query", "", "query")
	flag.StringVar(&region, "region", "", "region")
	flag.StringVar(&netboxHost, "netbox-host", "netbox.global.cloud.sap", "netbox host")
	flag.StringVar(&netboxToken, "netbox-api-token", "", "netbox token")
	flag.StringVar(&configpath, "config-dir", "./", "Directory where config and template files are located")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.Int64Var(&discoverInterval, "interval", 300, "discover interval in seconds")
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

	// level.Info(logger).Log("msg", fmt.Sprintf("config and template dir: %s", configpath))
}
