package netapp

import (
	"github.com/sapcc/netappsd/pkg/netbox"
)

type NetappDiscoverer struct {
	Netbox netbox.Client
}

func NewNetappDiscoverer(netboxHost, netboxToken string) (NetappDiscoverer, error) {
	nb, err := netbox.NewClient(netboxHost, netboxToken)
	if err != nil {
		return NetappDiscoverer{}, err
	}
	return NetappDiscoverer{
		Netbox: nb,
	}, nil
}

func (n NetappDiscoverer) Discover(region, query string) (map[string]interface{}, error) {
	filers, err := n.Netbox.GetFilers(region, query)
	if err != nil {
		return nil, err
	}
	data := make(map[string]interface{}, 0)
	for _, f := range filers {
		data[f.Name] = f
	}
	return data, nil
}
