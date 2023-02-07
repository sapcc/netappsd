package main

import (
	"github.com/sapcc/go-bits/promquery"
	sd "github.com/sapcc/netappsd/pkg/netappsd"
)

type filerQueue struct {
	filers     sd.Filers
	states     map[string]FilerState
	Netbox     *sd.Netbox
	Prometheus promquery.Client
}

type FilerState int32

const (
	newFiler     FilerState = 0
	scrapedFiler FilerState = 1
	expiredFiler FilerState = 2
)

func NewFilerQueue(prometheusUrl string) (*filerQueue, error) {
	nb, err := sd.NewNetboxClient(netboxHost, netboxToken)
	if err != nil {
		return nil, err
	}
	prometheus, err := promquery.Config{
		ServerURL: prometheusUrl,
	}.Connect()
	if err != nil {
		return nil, err
	}
	states := make(map[string]FilerState)
	return &filerQueue{Netbox: nb, Prometheus: prometheus, states: states}, nil

}

// QueryFilersFromPrometheus sets filers states to scrapedFiler if their
// metrics are found in prometheus. If a filer's metrics were fond before but
// not any more, it is removed from the states.
func (q *filerQueue) QueryFilersFromPrometheus() error {
	// query := "count by (cluster) (netapp_aggr_labels)"
	query := "count by (clusters) (netapp_aggr_labels)"
	resultVectors, err := q.Prometheus.GetVector(query)
	if err != nil {
		return err
	}
	for f, s := range q.states {
		if s == scrapedFiler {
			q.states[f] = expiredFiler
		}
	}
	for _, m := range resultVectors {
		if m.Metric["cluster"] != "" {
			q.states[string(m.Metric["cluster"])] = scrapedFiler
		}
	}
	for f, s := range q.states {
		if s == expiredFiler {
			delete(q.states, f)
		}
		if s > expiredFiler {
			q.states[f] += 1
		}
	}
	return nil
}

func (q *filerQueue) QueryFilersFromNetbox(region, query string) error {
	filers, err := sd.GetFilers(q.Netbox, region, query)
	if err != nil {
		return err
	}
	for _, f := range filers {
		if _, found := q.states[f.Name]; !found {
			q.states[f.Name] = newFiler
		}
	}
	q.filers = filers
	return nil
}
