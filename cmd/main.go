package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	klog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"netappsd"
)

var (
	logger        klog.Logger
	namespace     string
	configmapName string
	configmapKey  string
	local         bool
)

func init() {
	logger = klog.NewLogfmtLogger(klog.NewSyncWriter(os.Stdout))
	flag.StringVar(&namespace, "namespace", "", "namespace")
	flag.StringVar(&configmapName, "configmap", "netapp-perf-etc", "configmap name")
	flag.StringVar(&configmapKey, "key", "netapp-filers.yaml", "configmap key")
	flag.BoolVar(&local, "local", false, "run program out of cluster")
	flag.Parse()

	if namespace == "" {
		log.Fatal("flag namespace must be specified")
	}
}

func main() {
	tick := time.Tick(5 * time.Minute)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	var cm *netappsd.ConfigMap
	var err error
	if local {
		cm, err = netappsd.NewConfigMapOutofCluster(configmapName, namespace, logger)
		logError(err)
	} else {
		cm, err = netappsd.NewConfigMap(configmapName, namespace, logger)
		logError(err)
	}

	var filers netappsd.Filers
	netboxClient := newNetboxClient()

	for {
		newFilers, err := netboxClient.QueryNetappFilers("bb", "qa-de-1")
		if err != nil {
			level.Error(logger).Log("msg", err)
		} else {
			for _, f := range newFilers {
				level.Info(logger).Log("filer", f.Host)
			}
			if compareFilers(filers, newFilers) {
				cm.Write(configmapKey, newFilers.YamlString())
				filers = newFilers
			}
		}

		select {
		case <-tick:
			log.Println("tick")
			continue
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

// CompareFilers returns true when the lists are not equal
func compareFilers(f, g netappsd.Filers) bool {
	if len(f) != len(g) {
		return true
	}
	diff := make(map[string]int)
	for _, ff := range f {
		diff[ff.Name]++
	}
	for _, gg := range g {
		diff[gg.Name]--
	}
	for _, v := range diff {
		if v != 0 {
			return true
		}
	}
	return false
}

func newNetboxClient() *netappsd.Netbox {
	c, err := netappsd.NewNetbox(`c1d40ae380689e55384c26f1e5303a36f618ca73`)
	logError(err)
	return c
}
