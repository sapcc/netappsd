package monitor

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (q *Monitor) InitMetrics(prefix string) {
	name := "discovered_count"
	if prefix != "" {
		name = prefix + "_" + name
	}
	q.discoveredGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: name,
			Help: "Total number of discovered items",
		},
	)
}

func (q *Monitor) AddMetricsHandler(r *mux.Router) {
	r.Methods("GET").Path("/metrics").Handler(promhttp.Handler())
}
