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
)

func init() {
	prometheus.MustRegister(discoveredFiler)
	prometheus.MustRegister(workerReplicas)
}
