package monitor

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (q *Monitor) InitMetrics(prefix string) {
	tname := "discovered_count"
	wname := "worker_count"
	if prefix != "" {
		tname = prefix + "_" + tname
		wname = prefix + "_" + wname
	}

	q.discoveredGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: tname,
			Help: "Total number of discovered items",
		},
	)
	q.workerGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: wname,
			Help: "Total number of workers",
		},
	)

}

func (q *Monitor) AddMetricsHandler(r *mux.Router) {
	r.Methods("GET").Path("/metrics").Handler(promhttp.Handler())
}
