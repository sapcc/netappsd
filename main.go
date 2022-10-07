package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	cmwriter "github.com/sapcc/atlas/pkg/writer"
)

var (
	query         string
	region        string
	namespace     string
	configmapName string
	filepath      string
	netboxHost    string
	netboxToken   string
	logLevel      string
	logger        log.Logger
)

func main() {
	var (
		cm  cmwriter.Writer
		err error
	)

	if configmapName != "" {
		cm, err = cmwriter.NewConfigMap(configmapName, namespace, logger)
		if err != nil {
			level.Error(logger).Log("msg", err)
			os.Exit(1)
		}
	}
	nb, err := NewNetboxClient(netboxHost, netboxToken)
	if err != nil {
		level.Error(logger).Log(err)
		os.Exit(1)
	}

	// query netbox every 15 min and update configmap
	filers := make(Filers)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		go func() {
			// query netbox for filers
			newFilerFound := false
			newFilers, err := GetFilers(nb, region, query)
			if err != nil {
				level.Error(logger).Log("msg", err)
			}
			if len(newFilers) == 0 {
				level.Warn(logger).Log("msg", "no filers found")
				return
			}
			for fname, fnew := range newFilers {
				if f, ok := filers[fname]; !ok && f != fnew {
					newFilerFound = true
					level.Debug(logger).Log("name", fnew.Name, "host", fnew.Host, "az", fnew.AZ, "ip", fnew.IP)
				}
			}
			// write filers to configmap
			if !newFilerFound {
				return
			}
			if cm == nil {
				err = ioutil.WriteFile(filepath, []byte(newFilers.YamlString()), 0)
				level.Info(logger).Log("msg", fmt.Sprintf("%d filers are written to %s", len(newFilers), filepath))
			} else {
				err = cm.Write(filepath, newFilers.YamlString())
			}
			if err != nil {
				level.Error(logger).Log("msg", err)
				return
			}
			filers = newFilers
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
	flag.StringVar(&filepath, "output-file-path", "filers.yaml", "output file path (also used as key in configmap)")
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

	if region == "" {
		level.Error(logger).Log("msg", "region must be specified")
		os.Exit(1)
	}
	if configmapName == "" {
		level.Warn(logger).Log("msg", fmt.Sprintf("configmap not specified, writting to local file: %s", filepath))
	} else {
		if namespace == "" {
			level.Error(logger).Log("msg", "namespace must be specified when configmapName is specified")
			os.Exit(1)
		}
	}
}
