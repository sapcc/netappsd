package monitor

import (
	"strings"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (q *Monitor) InitMetrics(prefix string) {
	q.discoveredGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: strings.TrimPrefix(prefix+"_discovered_count", "_"),
			Help: "Total number of discovered items",
		},
	)
	q.probeFailureGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: strings.TrimPrefix(prefix+"_probe_failure", "_"),
			Help: "Target probe has failed",
		},
		[]string{"host", "reason"},
	)
}

func (q *Monitor) AddMetricsHandler(r *mux.Router) {
	r.Methods("GET").Path("/metrics").Handler(promhttp.Handler())
}
