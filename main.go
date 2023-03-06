package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/sapcc/netappsd/pkg/monitor"
	"github.com/sapcc/netappsd/pkg/monitor/netapp"
)

var (
	addr             string
	configpath       string
	logLevel         string
	metricsPrefix    string
	netboxHost       string
	netboxQuery      string
	netboxToken      string
	promLabel        string
	promQuery        string
	promUrl          string
	region           string
	q                *monitor.Monitor
	srv              *http.Server
	discoverInterval time.Duration
	observeInterval  time.Duration
	log              zerolog.Logger
)

func main() {
	ctx := cancelCtxOnSigterm(context.Background())

	m, err := netapp.NewNetappDiscoverer(netboxHost, netboxToken)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	q = monitor.NewMonitorQueue(m, metricsPrefix, &log)

	go q.DoObserve(ctx, observeInterval, promUrl, promQuery, promLabel)
	go q.DoDiscover(ctx, discoverInterval, region, netboxQuery)

	r := mux.NewRouter()
	// r.Methods("GET", "HEAD").Path("/next/name").HandlerFunc(handleNameRequest)
	r.Methods("POST").Path("/next/{templateName}.yaml").HandlerFunc(handleYamlRequest(configpath))
	q.AddMetricsHandler(r)

	go func() {
		srv = &http.Server{Handler: r, Addr: addr}
		log.Info().Msgf("starting server at address %s", addr)
		log.Fatal().Err(srv.ListenAndServe()).Send()
	}()

	<-ctx.Done()
	if err := srv.Shutdown(context.TODO()); err != nil {
		panic(err)
	}
}

func init() {
	flag.StringVar(&addr, "address", "0.0.0.0:8000", "server address")
	flag.StringVar(&netboxQuery, "query", "", "query")
	flag.StringVar(&region, "region", "", "region")
	flag.StringVar(&netboxHost, "netbox-host", "netbox.global.cloud.sap", "netbox host")
	flag.StringVar(&netboxToken, "netbox-api-token", "", "netbox token")
	flag.StringVar(&configpath, "config-dir", "./", "Directory where config and template files are located")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.DurationVar(&discoverInterval, "discover-interval", 5*time.Minute, "time interval between dicovering filers from netbox")
	flag.DurationVar(&observeInterval, "update-interval", 3*time.Minute, "time interval between state updates from prometheus")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log = zerolog.New(output).With().Timestamp().Logger()
	// log = zerolog.New(output).With().Caller().Timestamp().Logger()

	if region == "" {
		log.Fatal().Msg("region must be specified")
	}
	promUrl = os.Getenv("NETAPPSD_PROMETHEUS_URL")
	if promUrl == "" {
		log.Fatal().Msg("env variable NETAPPSD_PROMETHEUS_URL not set")
	}
	promQuery = os.Getenv("NETAPPSD_PROMETHEUS_OBSERVE_QUERY")
	if promQuery == "" {
		log.Fatal().Msg("env variable NETAPPSD_PROMETHEUS_OBSERVE_QUERY not set")
	}
	promLabel = os.Getenv("NETAPPSD_PROMETHEUS_OBSERVE_LABEL")
	if promLabel == "" {
		log.Fatal().Msg("env variable NETAPPSD_PROMETHEUS_OBSERVE_LABEL not set")
	}
	metricsPrefix = os.Getenv("NETAPPSD_METRICS_PREFIX")

	log.Info().Msgf("config and template dir: %s", configpath)
	log.Info().Msgf("observe metrics from %s", promUrl)
	log.Info().Msgf("observe metrics by query %s", promQuery)
}
