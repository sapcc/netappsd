package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sapcc/go-bits/httpapi"
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
	promUrl := os.Getenv("NETAPPSD_PROMETHEUS_URL")
	if promUrl == "" {
		logFatal("env variable NETAPPSD_PROMETHEUS_URL not set")
	}
	fq, err := NewFilerQueue(promUrl)
	if err != nil {
		logFatal(err)
	}

	ctx := cancelCtxOnSigterm(context.Background())

	go func() {
		ticker := time.NewTicker(time.Duration(updateInterval) * time.Second)
		defer ticker.Stop()

		// stop ticker before exit properly
		stopTicker := func(code int) int {
			defer ticker.Stop()
			return code
		}

		for {
			err = fq.QueryFilersFromNetbox(region, query)
			if err != nil {
				logError(err)
				os.Exit(stopTicker(1))
			}
			for _, f := range fq.filers {
				debug("new filer", "name", f.Name, "host", f.Host, "az", f.AvailabilityZone, "ip", f.IP)
			}
			err = fq.QueryFilersFromPrometheus()
			if err != nil {
				logError(err)
				os.Exit(stopTicker(1))
			}
			for f, s := range fq.states {
				info("", "name", f, "state", s)
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				os.Exit(0)
			}
		}
	}()

	srv := &http.Server{
		Handler: httpapi.Compose(),
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

func (q *filerQueue) Export() {
	for {
		// // add filers to queue if not being scraped
		// filers := q.QueryFilersInNetbox()
		// for _, filer := range filers {
		// 	if ok := q.running[filer]; ok {
		// 		continue
		// 	}
		// 	if ok := q.staging[filer]; ok {
		// 		continue
		// 	}
		// 	if ok := q.queued[filer]; !ok {
		// 		add_queue(filer)
		// 	}
		// }
		//
		// //
		// for filer := range q.running {
		// 	if ok := running_filers[filer]; !ok {
		// 		remove_running(filer)
		// 		export_missing_filer(filer)
		// 	}
		// }
		//
		// // when staging filers are not removed in 10 runs
		// for filer := range q.staging {
		// 	count := add_staging_count(filer)
		// 	if count >= 10 {
		// 		remove_staging(filer)
		// 		export_missing_filer()
		// 	}
		// }

		// running_filers := compare_staging_files(q.staging, running_filers)
		// for filer := range running_filers {
		//     if filer := q.staging[filer] {
		//       move_staging_to_running(filer)
		//     }
		// }

	}
}
