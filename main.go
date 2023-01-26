package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	sd "github.com/sapcc/netappsd/pkg/netappsd"
)

var (
	query          string
	region         string
	namespace      string
	filepath       string
	templatePath   string
	netboxHost     string
	netboxToken    string
	logLevel       string
	logger         log.Logger
	updateInterval int64
)

func main() {
	nb, err := sd.NewNetboxClient(netboxHost, netboxToken)
	if err != nil {
		erro(err)
		os.Exit(1)
	}

	c, err := sd.NewFilerConfig(filepath, templatePath, nb)
	if err != nil {
		warn("msg", fmt.Sprintf("new config: %s", err))
	}

	// query netbox every 15 min and update configmap
	ticker := time.NewTicker(time.Duration(updateInterval) * time.Second)
	defer ticker.Stop()

	// stop ticker before exit
	quit := func(code int) int {
		defer ticker.Stop()
		return code
	}

	// create context
	ctx := cancelCtxOnSigterm(context.Background())

	for {
		found, err := updateConfig(c)
		if err != nil {
			erro(err)
		}
		if found {
			// exit with 1 if new filer found
			// pod will be restarted when container fails with exit code 1 ???
			os.Exit(quit(1))
		}

		debug("msg", fmt.Sprintf("No new filers found, continue in %d seconds..", updateInterval))

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return
		}
	}
}

func init() {
	flag.StringVar(&query, "query", "", "query")
	flag.StringVar(&region, "region", "", "region")
	flag.StringVar(&netboxHost, "netbox-host", "netbox.global.cloud.sap", "netbox host")
	flag.StringVar(&netboxToken, "netbox-api-token", "", "netbox token")
	flag.StringVar(&filepath, "output", "./filers.yaml", "output file path")
	flag.StringVar(&templatePath, "template", "", "template path")
	flag.StringVar(&logLevel, "log-level", "debug", "log level")
	flag.Int64Var(&updateInterval, "interval", 900, "update interval in seconds")
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

	level.Info(logger).Log("msg", fmt.Sprintf("output to local file %s", filepath))
}

// updateConfig fetches data and compare with old data
func updateConfig(c *sd.FilerConfig) (foundNew bool, err error) {
	filers, newFilers, err := c.Fetch(region, query)
	if err != nil {
		return
	}

	// compoare old and new filers
	for name, newf := range newFilers {
		if oldf, ok := filers[name]; !ok && oldf != newf {
			level.Debug(logger).Log("name", newf.Name, "host", newf.Host, "az", newf.AZ, "ip", newf.IP)
			foundNew = true
		}
	}

	if foundNew {
		if err = c.Write(); err != nil {
			return
		}
	}
	return
}

/* helpers */

func cancelCtxOnSigterm(ctx context.Context) context.Context {
	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-exitCh
		cancel()
	}()
	return ctx
}

func info(keyvals ...interface{}) {
	level.Info(logger).Log(keyvals...)
}
func erro(keyvals ...interface{}) {
	level.Error(logger).Log(keyvals...)
}
func warn(keyvals ...interface{}) {
	level.Warn(logger).Log(keyvals...)
}

func debug(keyvals ...interface{}) {
	level.Debug(logger).Log(keyvals...)
}
