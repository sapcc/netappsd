package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	klog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/heptiolabs/healthcheck"
	"github.com/sapcc/atlas/pkg/netbox"
	cmwriter "github.com/sapcc/atlas/pkg/writer"
)

var (
	logger        klog.Logger
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
	logger = klog.NewLogfmtLogger(klog.NewSyncWriter(os.Stdout))
	flag.StringVar(&query, "query", "", "query")
	flag.StringVar(&region, "region", "", "region")
	flag.StringVar(&namespace, "namespace", "", "namespace")
	flag.StringVar(&configmapName, "configmap", "netapp-perf-etc", "configmap name")
	flag.StringVar(&configmapKey, "key", "netapp-filers.yaml", "configmap key")
	flag.StringVar(&netboxHost, "netbox-host", "", "netbox host")
	flag.StringVar(&netboxToken, "netbox-api-token", "", "netbox token")
	flag.BoolVar(&local, "local", false, "run program out of cluster")
	flag.Parse()

	if namespace == "" {
		log.Fatal("flag namespace must be specified")
	}
	if netboxHost == "" {
		log.Fatal("netbox host must be specified")
	}
	if netboxToken == "" {
		log.Fatal("netbox token must be specified")
	}
	if region == "" {
		log.Fatal("region must be specified")
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
	_ = level.Info(logger).Log("msg", fmt.Sprintf("create writer to configmap: %s", configmapName))
	if local {
		logger = level.NewFilter(logger, level.AllowAll())
		filerName := namespace + "_" + configmapName + ".out"
		cm, err = cmwriter.NewFile(filerName, logger)
	} else {
		logger = level.NewFilter(logger, level.AllowInfo())
		cm, err = cmwriter.NewConfigMap(configmapName, namespace, logger)
	}
	logErrorAndExit(err)

	// create netbox client
	nb, err := netbox.New(netboxHost, netboxToken)
	logErrorAndExit(err)

	// query netbox periodically and update configmap
	tick := time.Tick(5 * time.Minute)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	for {
		writeFiler := false

		// query netbox for filers
		newFilers, err := GetFilers(nb, region, query)
		if err != nil {
			_ = level.Error(logger).Log("msg", err)
		}

		if newFilers == nil {
			_ = level.Warn(logger).Log("msg", "No filers found")
		} else {
			if reflect.DeepEqual(newFilers, filers) {
				_ = level.Info(logger).Log("msg", "Filers are not changed")
			} else {
				writeFiler = true
			}
		}

		// write filers to configmap
		if writeFiler {
			err = cm.Write(configmapKey, newFilers.YamlString())
			if err != nil {
				_ = level.Error(logger).Log("error", err)
			} else {
				// don't set cmUpdated for liveness probe when the filers
				// are written to configmap for the first time
				if filers != nil {
					cmUpdated = true
				}

				filers = newFilers
				_ = level.Info(logger).Log("msg", filers.JsonString())
			}
		}

		select {
		case <-tick:
			log.Println("tick")
		case sig := <-interrupt:
			log.Println(sig, "signal received")
			os.Exit(1)
		}
	}
}

func logErrorAndExit(err error) {
	if err != nil {
		_ = level.Error(logger).Log("msg", err)
		os.Exit(1)
	}
}
