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
	configpath     string
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
	c, err := sd.NewFilerConfig(configpath, nb)
	if err != nil {
		erro(fmt.Sprintf("new config: %s", err))
		os.Exit(1)
	}
	_, fs, err := c.Fetch(region, query)
	if err != nil {
		erro(fmt.Sprintf("fetch filers: %s", err))
		os.Exit(1)
	}
	err = c.RenderAndWrite()
	if err != nil {
		erro(fmt.Sprintf("write rendered files: %s", err))
		os.Exit(1)
	}
	for _, f := range fs {
		info("new filer", "name", f.Name, "host", f.Host, "az", f.AvailabilityZone, "ip", f.IP)
	}

	// create context
	ctx := cancelCtxOnSigterm(context.Background())

	// set timer to refresh config
	ticker := time.NewTicker(time.Duration(updateInterval) * time.Second)
	defer ticker.Stop()

	// stop ticker before exit properly
	quit := func(code int) int {
		defer ticker.Stop()
		return code
	}

	for {
		// fetch filers and test if config are changed
		changed, err := doUpdate(c)
		if err != nil {
			erro(err)
			os.Exit(quit(1))
		}

		if changed {
			// exit with 1 if new filer found
			// pod will be restarted when container fails with exit code 1 ???
			os.Exit(quit(1))
		}

		debug(fmt.Sprintf("No new filers found, continue in %d seconds..", updateInterval))

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
	flag.StringVar(&configpath, "config-dir", "./", "Directory where config and template files are located")
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

	level.Info(logger).Log("msg", fmt.Sprintf("config and template dir: %s", configpath))
}

// doUpdate fetches data and compare with old data
func doUpdate(c *sd.FilerConfig) (changed bool, err error) {
	ff, nf, err := c.Fetch(region, query)
	if err != nil {
		return
	}
	filers := make(map[string]sd.Filer, len(ff))
	for _, f := range ff {
		filers[f.Name] = f
	}
	newFilers := make(map[string]sd.Filer, len(nf))
	for _, f := range nf {
		newFilers[f.Name] = f
	}

	// compoare old and new filers
	for name, newf := range newFilers {
		if oldf, ok := filers[name]; !ok && oldf != newf {
			info("new filer", "name", newf.Name, "host", newf.Host, "az", newf.AvailabilityZone, "ip", newf.IP)
			changed = true
		}
	}
	for name, oldf := range filers {
		if _, ok := newFilers[name]; !ok {
			warn("remove filer", "name", oldf.Name, "host", oldf.Host)
			changed = true
		}
	}

	// write
	err = c.RenderAndWrite()
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

func info(msg string, keyvals ...interface{}) {
	keyvals = append([]interface{}{"msg", msg}, keyvals...)
	level.Info(logger).Log(keyvals...)
}

func warn(msg string, keyvals ...interface{}) {
	keyvals = append([]interface{}{"msg", msg}, keyvals...)
	level.Warn(logger).Log(keyvals...)
}

func erro(err interface{}, keyvals ...interface{}) {
	keyvals = append([]interface{}{"error", err}, keyvals...)
	level.Error(logger).Log(keyvals...)
}

func debug(msg string, keyvals ...interface{}) {
	keyvals = append([]interface{}{"msg", msg}, keyvals...)
	level.Debug(logger).Log(keyvals...)
}
