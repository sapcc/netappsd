package netappsd

import (
	"github.com/prometheus/common/model"
	"github.com/sapcc/go-bits/promquery"
)

type NetappMonitor struct {
	Netbox     *Netbox
	Prometheus promquery.Client
}

func NewNetappMonitor(netboxHost, netboxToken, promUrl string) (NetappMonitor, error) {
	nb, err := NewNetboxClient(netboxHost, netboxToken)
	if err != nil {
		return NetappMonitor{}, err
	}
	prometheus, err := promquery.Config{
		ServerURL: promUrl,
	}.Connect()
	if err != nil {
		return NetappMonitor{}, err
	}
	return NetappMonitor{
		Netbox:     nb,
		Prometheus: prometheus,
	}, nil
}

func (n NetappMonitor) Observe(promQ, label string) (obs []string, err error) {
	resultVectors, err := n.Prometheus.GetVector(promQ)
	if err != nil {
		return nil, err
	}
	for _, m := range resultVectors {
		v := m.Metric[model.LabelName(label)]
		if v != "" {
			obs = append(obs, string(v))
		}
	}
	return
}

func (n NetappMonitor) Discover(region, query string) (map[string]interface{}, error) {
	filers, err := GetFilers(n.Netbox, region, query)
	if err != nil {
		return nil, err
	}
	data := make(map[string]interface{}, 0)
	for _, f := range filers {
		data[f.Name] = f
	}
	return data, nil
}
