package main

import (
	"flag"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sapcc/atlas/pkg/netbox"
	cmwriter "github.com/sapcc/atlas/pkg/writer"
)

var (
	query         string
	region        string
	namespace     string
	configmapName string
	filename      string
	netboxHost    string
	netboxToken   string
	logLevel      string
	local         bool
	cmUpdated     bool
	logger        log.Logger
)

func main() {
	var (
		cm     cmwriter.Writer
		err    error
		filers Filers
	)

	// create configmap writer
	if configmapName == "" {
		cm, err = cmwriter.NewFile(filename, logger)
	} else {
		cm, err = cmwriter.NewConfigMap(configmapName, namespace, logger)
	}
	if err != nil {
		level.Error(logger).Log("msg", err)
		os.Exit(1)
	}

	// create netbox client
	nb, err := netbox.New(netboxHost, netboxToken)
	if err != nil {
		level.Error(logger).Log(err)
		os.Exit(1)
	}

	// query netbox every 15 min and update configmap
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		go func() {
			// query netbox for filers
			newFilers, err := GetFilers(nb, region, query)
			if err != nil {
				level.Error(logger).Log("msg", err)
			}
			if newFilers == nil {
				level.Warn(logger).Log("msg", "no filers found")
				return
			}
			if reflect.DeepEqual(newFilers, filers) {
				level.Debug(logger).Log("msg", "no new filers")
				return
			}
			// write filers to configmap
			err = cm.Write(filename, newFilers.YamlString())
			if err != nil {
				level.Error(logger).Log("msg", err)
				return
			}
			filers = newFilers
			for _, filer := range filers {
				level.Info(logger).Log("name", filer.Name, "host", filer.Host, "az", filer.AZ)
			}
		}()

		select {
		case <-ticker.C:
		case sig := <-interrupt:
			logger.Log("%v signal received", sig)
			os.Exit(0)
		}
	}
}

func init() {
	flag.StringVar(&query, "query", "", "query")
	flag.StringVar(&region, "region", "", "region")
	flag.StringVar(&namespace, "namespace", "", "namespace")
	flag.StringVar(&configmapName, "configmap", "", "configmap name")
	flag.StringVar(&filename, "filename", "netapp-filers.yaml", "file name (used as key in configmap)")
	flag.StringVar(&netboxHost, "netbox-host", "netbox.global.cloud.sap", "netbox host")
	flag.StringVar(&netboxToken, "netbox-api-token", "", "netbox token")
	flag.StringVar(&logLevel, "log-level", "debug", "log level")
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

	if netboxToken == "" {
		level.Error(logger).Log("msg", "netbox token must be specified")
		os.Exit(1)
	}
	if region == "" {
		level.Error(logger).Log("msg", "region must be specified")
		os.Exit(1)
	}
	if configmapName == "" {
		level.Warn(logger).Log("msg", "configmap not specified, writting to local file")
	} else {
		if namespace == "" {
			level.Error(logger).Log("msg", "namespace must be specified when configmapName is specified")
			os.Exit(1)
		}
	}
}
