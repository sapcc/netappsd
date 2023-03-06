package netapp

import (
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/rs/zerolog"
	"github.com/sapcc/netappsd/pkg/netbox"
)

type NetappDiscoverer struct {
	Netbox netbox.Client
	log    *zerolog.Logger
}

func NewNetappDiscoverer(netboxHost, netboxToken string, log *zerolog.Logger) (NetappDiscoverer, error) {
	nb, err := netbox.NewClient(netboxHost, netboxToken)
	if err != nil {
		return NetappDiscoverer{}, err
	}
	return NetappDiscoverer{
		Netbox: nb,
		log:    log,
	}, nil
}

func (n NetappDiscoverer) Discover(region, query string) (map[string]interface{}, error) {
	filers, err := n.Netbox.GetFilers(region, query)
	if err != nil {
		return nil, err
	}
	data := make(map[string]interface{}, 0)
	for _, f := range filers {
		err := probeHost(f.Host)
		if err != nil {
			n.log.Warn().Str("host", f.Host).Msg("probing host error")
			continue
		}
		n.log.Debug().Str("host", f.Host).Msg("probing host ok")
		data[f.Name] = f
	}
	return data, nil
}

func probeHost(host string) error {
	pinger, err := probing.NewPinger(host)
	if err != nil {
		return err
	}
	pinger.Count = 1
	pinger.Timeout = 500 * time.Millisecond
	err = pinger.Run() // Blocks until finished.
	if err != nil {
		return err
	}
	return nil
}
