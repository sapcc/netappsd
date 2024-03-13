package netappsd

import (
	"github.com/prometheus/client_golang/prometheus"
)

var discoveredFilers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "netappsd_discovered_filer",
	Help: "Filer discovered from netbox.",
}, []string{"filer_name", "filer_host"})

func init() {
	prometheus.MustRegister(discoveredFilers)
}
