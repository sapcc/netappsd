package monitor

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	totalItems = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "total_items",
		Help: "Total number of discovered items",
	})

	workedItems = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "worked_items",
		Help: "Number of items being worked on",
	})
)

func (q *MonitorQueue) RegisterMetrics(r *mux.Router) {
	r.Methods("GET").Path("/metrics").Handler(promhttp.Handler())

}
