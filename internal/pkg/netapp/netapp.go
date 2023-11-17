package netapp

import (
	"fmt"
	"time"

	"github.com/sapcc/netappsd/pkg/netbox"
)

type NetappDiscovererError struct {
	Host   string
	Reason string
	Err    error
}

func (r *NetappDiscovererError) Error() string {
	return fmt.Sprintf("%s: %v", r.Reason, r.Err)
}

type NetappDiscoverer struct {
	Netbox         netbox.Client
	netappPassword string
	netappUsername string
}

func NewNetappDiscoverer(netboxHost, netboxToken string, netappUsername, netappPassword string) (NetappDiscoverer, error) {
	nb, err := netbox.NewClient(netboxHost, netboxToken)
	if err != nil {
		return NetappDiscoverer{}, err
	}
	return NetappDiscoverer{
		Netbox:         nb,
		netappUsername: netappUsername,
		netappPassword: netappPassword,
	}, nil
}

func (n NetappDiscoverer) Discover(region, query string) (map[string]interface{}, []error, error) {
	filers, err := n.Netbox.GetFilers(region, query)
	if err != nil {
		return nil, nil, err
	}
	data := make(map[string]interface{}, 0)
	warns := make([]error, 0)
	for _, f := range filers {
		if f.IP == "" {
			continue
		}
		w := n.probe(f.Host, f.IP, n.netappUsername, n.netappPassword, 10*time.Second)
		if w != nil {
			warns = append(warns, w)
			continue
		}
		data[f.Name] = f
	}
	return data, warns, nil
}

func (n NetappDiscoverer) probe(host, addr, username, password string, timeout time.Duration) error {
	var reason string
	opts := ClientOptions{
		BasicAuthUser:     username,
		BasicAuthPassword: password,
		Timeout:           timeout,
	}
	addr = fmt.Sprintf("https://%s", addr)
	c := NewRestClient(addr, &opts)
	resp, err := c.DoRequest("/api/cluster")
	if err != nil {
		if resp == nil {
			reason = "connection error"
			return &NetappDiscovererError{host, reason, err}
		}
		switch resp.StatusCode {
		case 401:
			reason = "authentication error"
		default:
			reason = "other error"
		}
		return &NetappDiscovererError{host, reason, err}
	}
	return nil
}
