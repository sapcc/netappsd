package netappsd

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	discoveredFiler = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "netappsd_discovered_filer",
		Help: "Filer discovered from netbox.",
	}, []string{"filer", "host", "ip"})

	managedDeployments = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "netappsd_managed_deployments",
		Help: "Filer deployments managed by netappsd.",
	}, []string{"filer", "host", "ip"})

	probeFilerErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "netappsd_probe_filer_errors",
		Help: "Number of errors encountered while probing filer.",
	}, []string{"filer", "host", "ip"})
)

func init() {
	prometheus.MustRegister(discoveredFiler)
	prometheus.MustRegister(managedDeployments)
	prometheus.MustRegister(probeFilerErrors)
}
