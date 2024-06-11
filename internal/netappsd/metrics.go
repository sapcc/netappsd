package netappsd

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	discoveredFiler = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "netappsd_discovered_filer",
		Help: "Filer discovered from netbox.",
	}, []string{"filer", "host", "ip"})

	enqueuedFiler = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "netappsd_enqueued_filer",
		Help: "Filer enqueued to work on.",
	}, []string{"filer", "host", "ip"})

	probeFilerErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "netappsd_probe_filer_errors",
		Help: "Number of errors encountered while probing filer.",
	}, []string{"filer", "host", "ip"})

	workerReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "netappsd_worker_replicas",
		Help: "Number of worker replicas.",
	}, []string{})
)

func init() {
	prometheus.MustRegister(discoveredFiler)
	prometheus.MustRegister(enqueuedFiler)
	prometheus.MustRegister(probeFilerErrors)
	prometheus.MustRegister(workerReplicas)
}
