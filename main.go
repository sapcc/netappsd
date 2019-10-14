package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	klog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
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
	var cm cmwriter.Writer
	var err error

	// create configmap writer
	level.Info(logger).Log("msg", fmt.Sprintf("create writer to configmap: %s", configmapName))
	if local {
		// cm, err = netappsd.NewConfigMapOutofCluster(configmapName, namespace, logger)
		cm, err = cmwriter.NewFile(namespace+"_"+configmapName+".out", logger)
	} else {
		cm, err = cmwriter.NewConfigMap(configmapName, namespace, logger)
	}
	logErrorAndExit(err)

	// create netbox client
	nb, err := netbox.New(netboxHost, netboxToken)
	logErrorAndExit(err)

	// query netbox periodically and update configmap
	filers := NewFilers()
	tick := time.Tick(5 * time.Minute)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	for {
		updated, err := filers.QueryNetbox(nb, region, query)
		logError(err)
		// write filers when there are updates
		if updated {
			logError(cm.Write(configmapKey, filers.YamlString()))
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

func logError(err error) {
	if err != nil {
		level.Error(logger).Log("msg", err)
	}
}

func logErrorAndExit(err error) {
	if err != nil {
		level.Error(logger).Log("msg", err)
		os.Exit(1)
	}
}
