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

	"netappsd"
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
		cm, err = netappsd.NewConfigMap(configmapName, namespace, logger)
	}
	logErrorAndExit(err)

	// create netbox client
	nb, err := netbox.New(netboxHost, netboxToken)
	logErrorAndExit(err)

	// query netbox periodically and update configmap
	filers := netappsd.NewFilers()
	tick := time.Tick(5 * time.Minute)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	for {
		updated, err := filers.QueryNetbox(nb, query, region)
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

// newFilers, err := queryNetappFilers(nb, query, region)
// if err != nil {
// 	level.Error(logger).Log("msg", err)
// } else {
// 	for _, f := range newFilers {
// 		level.Info(logger).Log("filer", f.Host)
// 	}
// 	ff, err := cm.GetData(configmapKey)
// 	if err != nil {
// 		level.Error(logger).Log(err)
// 	} else {
// 		level.Info(logger).Log(ff)
// 	}
// 	if compareFilers(filers, newFilers) {
// 		level.Info(logger).Log("msg", fmt.Sprintf("update configmap key: %s", configmapKey))
// 		err := cm.Write(configmapKey, newFilers.YamlString())
// 		if err != nil {
// 			level.Error(logger).Log(err)
// 		} else {
// 			filers = newFilers
// 		}
// 	}
// }
// // CompareFilers returns true when the lists are not equal
// func compareFilers(f, g netappsd.Filers) bool {
// 	if len(f) != len(g) {
// 		return true
// 	}
// 	diff := make(map[string]int)
// 	for _, ff := range f {
// 		diff[ff.Name]++
// 	}
// 	for _, gg := range g {
// 		diff[gg.Name]--
// 	}
// 	for _, v := range diff {
// 		if v != 0 {
// 			return true
// 		}
// 	}
// 	return false
// }
