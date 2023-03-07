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
	netappUsername   string
	netappPassword   string
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

	m, err := netapp.NewNetappDiscoverer(netboxHost, netboxToken, netappUsername, netappPassword, &log)
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
	log = NewLogger()

	netappUsername = MustGetenv("NETAPP_USERNAME", log)
	netappPassword = MustGetenv("NETAPP_PASSWORD", log)
	promUrl = MustGetenv("NETAPPSD_PROMETHEUS_OBSERVE_URL", log)
	promQuery = MustGetenv("NETAPPSD_PROMETHEUS_OBSERVE_QUERY", log)
	promLabel = MustGetenv("NETAPPSD_PROMETHEUS_OBSERVE_LABEL", log)
	metricsPrefix = os.Getenv("NETAPPSD_METRICS_PREFIX")

	flag.StringVar(&addr, "address", "0.0.0.0:8000", "server address")
	flag.StringVar(&netboxQuery, "query", "", "query")
	flag.StringVar(&region, "region", "", "region")
	flag.StringVar(&netboxHost, "netbox-host", "netbox.global.cloud.sap", "netbox host")
	flag.StringVar(&netboxToken, "netbox-api-token", "", "netbox token")
	flag.StringVar(&configpath, "config-dir", "./", "Directory where config and template files are located")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.DurationVar(&discoverInterval, "discover-interval", 5*time.Minute, "time interval between dicovering filers from netbox")
	flag.DurationVar(&observeInterval, "update-interval", 1*time.Minute, "time interval between state updates from prometheus")
	flag.Parse()

	if region == "" {
		log.Fatal().Msg("region must be specified")
	}

	log.Info().Msgf("config and template dir: %s", configpath)
	log.Info().Msgf("observe metrics from %s", promUrl)
	log.Info().Msgf("observe metrics by query %s", promQuery)
}

func MustGetenv(k string, log zerolog.Logger) string {
	val := os.Getenv(k)
	if val == "" {
		log.Fatal().Msgf("env %s not set", k)
	}
	return val
}

func NewLogger() zerolog.Logger {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	return zerolog.New(output).With().Timestamp().Logger()
	// log = zerolog.New(output).With().Caller().Timestamp().Logger()
}
