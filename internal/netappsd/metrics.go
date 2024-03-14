package netappsd

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	discoveredFiler = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "netappsd_discovered_filer",
		Help: "Filer discovered from netbox.",
	}, []string{"filer", "filer_host"})

	workerReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "netappsd_worker_replicas",
		Help: "Number of worker replicas.",
	}, []string{})

	probeFilerErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "netappsd_probe_filer_errors",
		Help: "Number of errors encountered while probing filer.",
	}, []string{"filer", "filer_host"})
)

func init() {
	prometheus.MustRegister(discoveredFiler)
	prometheus.MustRegister(workerReplicas)
	prometheus.MustRegister(probeFilerErrors)
}
