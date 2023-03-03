package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/sapcc/go-bits/promquery"
)

type State int32

const (
	newState     State = 0
	scrapedState State = 1
	expiredState State = 2
	stagedState  State = 3
)

type Monitor struct {
	Discoverer
	data            map[string]interface{}
	states          map[string]State
	liveness        int
	log             *zerolog.Logger
	mu              sync.Mutex
	wg              sync.WaitGroup
	discoveredGauge prometheus.Gauge
	workerGauge     prometheus.Gauge
}

func NewMonitorQueue(m Discoverer, metricsPrefix string, log *zerolog.Logger) *Monitor {
	states := make(map[string]State, 0)
	q := Monitor{
		Discoverer: m, liveness: 0, states: states, log: log,
	}
	q.InitMetrics(metricsPrefix)
	q.wg.Add(1)
	return &q
}

func (q *Monitor) DoObserve(ctx context.Context, interval time.Duration, promUrl, promQ, promL string) error {
	q.log.Printf("Observer runs every %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	promClient, err := promquery.Config{
		ServerURL: promUrl,
	}.Connect()
	if err != nil {
		return err
	}

	// Starts Discoverer when Observer is ready
	ready := false

	for {
		func() {
			items, err := q.observe(promClient, promQ, promL)
			if err != nil {
				q.log.Err(err).Send()
				q.liveness = q.liveness - 1
				return
			}
			q.setStatesAfterObserving(items)

			// set observer liveness to a non-negative value
			// negative liveness values are considered as not live
			// we don't set liveness to negative immediately when observe fails in above
			// rather liveness is decreased by one to allow flappy prometheus connections
			// so the higher liveness is set in next line, the more tolerate to the prometheus connection quality
			q.liveness = 1

			// starts discovery right after observer is ready
			// if discoverer starts earlier, it might only start polling netbox in second loop
			// because it checks observer's liveness
			if !ready {
				q.log.Debug().Msg("Observer is ready")
				ready = true
				q.wg.Done()
			}
		}()

		if q.liveness < 0 {
			q.log.Warn().Msg("Discoverer has problems with observing metrics")
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func (q *Monitor) DoDiscover(ctx context.Context, interval time.Duration, region, query string) error {
	q.log.Printf("Discoverer runs every %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Wait for Observer ready
	q.wg.Wait()
	q.log.Debug().Msg("Discoverer starts")

	for {
		func() {
			// We don't know workers status when there are problems in observing
			// metrics from Prometheus. Should not add any new objects, since they
			// might already be picked by an unobserved worker.
			if q.liveness < 0 {
				return
			}
			data, err := q.Discover(region, query)
			if err != nil {
				q.log.Err(err).Send()
				return
			}
			q.setStatesAfterDiscovery(data)
		}()

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func (q *Monitor) setStatesAfterObserving(obs []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// first pass: set all scraped object to expired
	for f, s := range q.states {
		if s == scrapedState {
			q.states[f] = expiredState
		}
	}
	// second pass: set observed object to scraped
	for _, f := range obs {
		// log only for new and staged objects
		if q.states[f] == newState || q.states[f] >= stagedState {
			q.log.Debug().Str("name", f).Int("state", int(scrapedState)).Msg("set state (scraped)")
		}
		q.states[f] = scrapedState
	}
	// third pass: now the expired objects are truly expired, remove them
	for f, s := range q.states {
		if s == expiredState {
			delete(q.states, f)
			q.log.Debug().Str("name", f).Int("state", int(s)).Msg("remove expired object")
		}
	}
	// Staged objects are not candidates to serve, so they should be garbage
	// collected.
	// Always increase the state of staged objects by one in each call of this
	// function, and delete them if their states are larger than
	// stagedState+1. So they are not garbage collected on the next call after
	// they are staged, rather they are collected in two calls.
	for f, s := range q.states {
		if s > stagedState+State(1) {
			delete(q.states, f)
			q.log.Debug().Str("name", f).Int("state", int(s)).Msg("remove staged object")
		} else if s >= stagedState {
			q.states[f] = s + 1
			q.log.Debug().Str("name", f).Int("state", int(s+1)).Msg("set state")
		}
	}
	q.workerGauge.Set(float64(len(obs)))
}

func (q *Monitor) setStatesAfterDiscovery(data map[string]interface{}) {
	for f := range data {
		if _, found := q.states[f]; !found {
			q.states[f] = newState
			q.log.Debug().Str("name", f).Int("state", int(newState)).Msg("set state (new)")
		}
	}
	q.data = data
	q.discoveredGauge.Set(float64(len(q.data)))
}

func (q *Monitor) NextName() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// Do not return any thing when queue is not ready
	if q.liveness >= 0 {
		for f, s := range q.states {
			if s == newState {
				q.states[f] = stagedState
				q.log.Debug().Str("name", f).Int("state", int(stagedState)).Msg("set state (staged)")
				return f, true
			}
		}
	}
	return "", false
}

func (q *Monitor) NextItem() (interface{}, bool) {
	f, ok := q.NextName()
	if ok {
		return q.data[f], true
	}
	return nil, false
}

func (q *Monitor) observe(promClient promquery.Client, promQuery, promLabel string) (obs []string, err error) {
	resultVectors, err := promClient.GetVector(promQuery)
	if err != nil {
		return nil, err
	}
	for _, m := range resultVectors {
		v := m.Metric[model.LabelName(promLabel)]
		if v != "" {
			obs = append(obs, string(v))
		}
	}
	return obs, nil
}
