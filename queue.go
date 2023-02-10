package main

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gorilla/mux"
	"github.com/sapcc/go-bits/promquery"
	sd "github.com/sapcc/netappsd/pkg/netappsd"
)

type filerQueue struct {
	filers     sd.Filers
	states     map[string]FilerState
	Netbox     *sd.Netbox
	Prometheus promquery.Client
	mu         sync.Mutex
	ready      bool
}

type FilerState int32

const (
	newFiler     FilerState = 0
	scrapedFiler FilerState = 1
	expiredFiler FilerState = 2
	stagedFiler  FilerState = 3
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
	return &filerQueue{Netbox: nb, Prometheus: prometheus, states: states, ready: false}, nil
}

// FilerQueue serves candidate filer to a client, who starts a NetApp exporter
// for the filer. A filer in "new" state in the queue is selected and served
// when client requests at endpoint '/newfiler'. There is no particular order.
// Staged or scraped filers are not served. A filer is set to staged
// immediately after it is serverd, until it is set to scraped when the filer's
// metrics are in the prometheus. However a staged filer will be removed from
// queue after two calls of ObserveMetrics(), so that it can be served again to
// another client, assuming the first client has failed to start the NetApp
// exporter. Adjust the calling intervals to tune the time how long a filer can
// stay in staged state. The removed filer will be later added to the queue by
// DiscoverFilers().

// ObserveMetrics determins the filers' states by querying the metrics generated
// by the netapp exporters in prometheus. If filer's metrics are found, its
// state is set to scrapedFiler. If no metric found for a filer in
// "scrapedFiler" state, it is removed from queue's state. If no metric is
// found for a filer in stagingFiler (or greater) state, its state is increased
// by one.
func (q *filerQueue) ObserveMetrics(query string) error {
	// Do not serve any filer when prome query fails to avoid starting multiple
	// exporters for same filer.
	q.ready = false

	resultVectors, err := q.Prometheus.GetVector(query)
	if err != nil {
		return err
	}

	q.mu.Lock()
	defer func() {
		q.ready = true
		q.mu.Unlock()
	}()

	// Set all scraped filers to expired. The discovered filers will be reset to
	// scraped, and the unscraped filers are left in expired state.
	for f, s := range q.states {
		if s == scrapedFiler {
			q.states[f] = expiredFiler
		}
	}
	for _, m := range resultVectors {
		f := string(m.Metric["cluster"])
		if f != "" {
			if q.states[f] != expiredFiler {
				debug("set filer state", "state", scrapedFiler, "name", f)
			}
			q.states[f] = scrapedFiler
		}
	}
	// Remove the expired filers now, they are truly expired.
	for f, s := range q.states {
		if s == expiredFiler {
			delete(q.states, f)
			debug("remove expired filer from queue", "name", f)
		}
	}
	// Increase staged filers' states by one.
	for f, s := range q.states {
		if s > stagedFiler+FilerState(1) {
			delete(q.states, f)
			debug("remove staged filer queue", "state", s, "name", f)
		} else if s >= stagedFiler {
			q.states[f] += 1
			debug("set filer state", "state", q.states[f], "name", f)
		}
	}
	return nil
}

// DiscoverFilers fetches filers from netbox, and add new filers to the queue
// set their states to newFiler. Filers that are already in the queue are
// skipped.
func (q *filerQueue) DiscoverFilers(region, query string) error {
	filers, err := sd.GetFilers(q.Netbox, region, query)
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.filers = filers
	for _, f := range filers {
		if _, found := q.states[f.Name]; !found {
			q.states[f.Name] = newFiler
			debug("add new filer", "state", newFiler, "name", f.Name, "host", f.Host, "az", f.AvailabilityZone, "ip", f.IP)
		}
	}
	return nil
}

func (q *filerQueue) AddTo(r *mux.Router) {
	// r.Methods("GET", "HEAD").Path("/newfiler").HandlerFunc(q.HandleNewFilerRequddst)
	r.Methods("GET", "HEAD").Path("/harvest.yml").HandlerFunc(q.HandleHarvestYamlRequest)
}

// HandleNewFilerRequest serves a new filer in queue to requester
func (q *filerQueue) HandleNewFilerRequest(w http.ResponseWriter, r *http.Request) {
	q.mu.Lock()
	defer q.mu.Unlock()

	filer, found := q.findNewFiler()
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	jsonResp, err := json.Marshal(filer)
	if err != nil {
		logError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)
}

func (q *filerQueue) HandleHarvestYamlRequest(w http.ResponseWriter, r *http.Request) {
	q.mu.Lock()
	defer q.mu.Unlock()

	filer, found := q.findNewFiler()
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	err := q.parseHarvestYaml(w, filer)
	if err != nil {
		logError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (q *filerQueue) findNewFiler() (sd.Filer, bool) {
	if !q.ready {
		return sd.Filer{}, false
	}
	for filerName, s := range q.states {
		if s == newFiler {
			for _, filer := range q.filers {
				if filer.Name == filerName {
					debug("set filer state", "state", stagedFiler, "name", filer.Name)
					q.states[filer.Name] = stagedFiler
					return filer, true
				}
			}
		}
	}
	return sd.Filer{}, false
}

func (q *filerQueue) parseHarvestYaml(wr io.Writer, filer sd.Filer) error {
	t, err := template.ParseGlob(filepath.Join(configpath, "harvest.yml.tpl"))
	if err != nil {
		return err
	}
	return t.Execute(wr, sd.Filers{filer})
}
