package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	klog "github.com/go-kit/kit/log"
	klogrus "github.com/go-kit/kit/log/logrus"
	"github.com/heptiolabs/healthcheck"
	"github.com/sapcc/atlas/pkg/netbox"
	cmwriter "github.com/sapcc/atlas/pkg/writer"
	"github.com/sirupsen/logrus"
)

var (
	logger        *logrus.Logger
	klogger       klog.Logger
	query         string
	region        string
	namespace     string
	configmapName string
	configmapKey  string
	netboxHost    string
	netboxToken   string
	local         bool
	cmUpdated     bool
)

func init() {
	flag.StringVar(&query, "query", "", "query")
	flag.StringVar(&region, "region", "", "region")
	flag.StringVar(&namespace, "namespace", "", "namespace")
	flag.StringVar(&configmapName, "configmap", "netapp-perf-etc", "configmap name")
	flag.StringVar(&configmapKey, "key", "netapp-filers.yaml", "configmap key")
	flag.StringVar(&netboxHost, "netbox-host", "", "netbox host")
	flag.StringVar(&netboxToken, "netbox-api-token", "", "netbox token")
	flag.BoolVar(&local, "local", false, "run program out of cluster")
	flag.Parse()

	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	if local {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	klogger = klogrus.NewLogrusLogger(logger)

	if namespace == "" {
		logger.Fatal("flag namespace must be specified")
	}
	if netboxHost == "" {
		logger.Fatal("netbox host must be specified")
	}
	if netboxToken == "" {
		logger.Fatal("netbox token must be specified")
	}
	if region == "" {
		logger.Fatal("region must be specified")
	}
}

func main() {
	var (
		cm     cmwriter.Writer
		err    error
		filers Filers
	)

	health := healthcheck.NewHandler()
	health.AddLivenessCheck("configmap-updated", ValueCheck("configmap updated", func() bool {
		// return false on updating configmap
		return !cmUpdated
	}))

	// serve health check at :8086/live and :8086/ready
	go http.ListenAndServe("0.0.0.0:8086", health)

	// create configmap writer
	logger.Infof("create writer to configmap: %s", configmapName)
	if local {
		filerName := namespace + "_" + configmapName + ".out"
		cm, err = cmwriter.NewFile(filerName, klogger)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
		cm, err = cmwriter.NewConfigMap(configmapName, namespace, klogger)
	}
	if err != nil {
		logger.Error(err)
		logger.Exit(1)
	}

	// create netbox client
	nb, err := netbox.New(netboxHost, netboxToken)
	if err != nil {
		logger.Error(err)
		logger.Exit(1)
	}

	// query netbox periodically and update configmap
	tick := time.Tick(5 * time.Minute)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	for {
		writeFiler := false

		// query netbox for filers
		newFilers, err := GetFilers(nb, region, query)
		if err != nil {
			logger.Error(err)
		}

		if newFilers == nil {
			logger.Warn("No filers found")
		} else {
			if reflect.DeepEqual(newFilers, filers) {
				logger.Debug("Filers are not changed")
			} else {
				writeFiler = true
			}
		}

		// write filers to configmap
		if writeFiler {
			err = cm.Write(configmapKey, newFilers.YamlString())
			if err != nil {
				logger.Error(err)
			} else {
				// don't set cmUpdated for liveness probe when the filers
				// are written to configmap for the first time
				if filers != nil {
					cmUpdated = true
				}

				filers = newFilers

				for _, filer := range filers {
					logger.Info(filer)
				}
			}
		}

		select {
		case <-tick:
		case sig := <-interrupt:
			logger.Errorf("%v signal received", sig)
			logger.Exit(1)
		}
	}
}
