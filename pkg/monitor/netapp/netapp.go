package netapp

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sapcc/netappsd/pkg/netbox"
)

type NetappDiscoverer struct {
	Netbox         netbox.Client
	log            *zerolog.Logger
	netappPassword string
	netappUsername string
}

func NewNetappDiscoverer(netboxHost, netboxToken string, netappUsername, netappPassword string, log *zerolog.Logger) (NetappDiscoverer, error) {
	nb, err := netbox.NewClient(netboxHost, netboxToken)
	if err != nil {
		return NetappDiscoverer{}, err
	}
	return NetappDiscoverer{
		Netbox:         nb,
		log:            log,
		netappUsername: netappUsername,
		netappPassword: netappPassword,
	}, nil
}

func (n NetappDiscoverer) Discover(region, query string) (map[string]interface{}, error) {
	filers, err := n.Netbox.GetFilers(region, query)
	if err != nil {
		return nil, err
	}
	data := make(map[string]interface{}, 0)
	for _, f := range filers {
		err = n.probe(f.Host, n.netappUsername, n.netappPassword, 10*time.Second)
		if err != nil {
			n.log.Warn().Str("host", f.Host).Int("timeout", int(10*time.Second)).Msgf("probe failed: %v", err)
			continue
		}
		data[f.Name] = f
	}
	return data, nil
}

func (n NetappDiscoverer) probe(host, username, password string, timeout time.Duration) error {
	opts := ClientOptions{
		BasicAuthUser:     username,
		BasicAuthPassword: password,
		Timeout:           timeout,
	}
	host = fmt.Sprintf("https://%s", host)
	c := NewRestClient(host, &opts)
	return c.DoRequest("/api/cluster")
}
