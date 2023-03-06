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

type Queue struct {
	states map[string]State
}

type Monitor struct {
	Discoverer
	data            map[string]interface{}
	discoveredGauge prometheus.Gauge
	liveness        int
	log             *zerolog.Logger
	mu              sync.Mutex
	queues          map[string]Queue
	wg              sync.WaitGroup
}

func NewMonitorQueue(m Discoverer, metricsPrefix string, log *zerolog.Logger) *Monitor {
	queues := make(map[string]Queue, 0)
	q := Monitor{
		Discoverer: m, liveness: 0, queues: queues, log: log,
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

func (q *Monitor) setStatesAfterObserving(obs map[string][]string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for r := range obs {
		if _, found := q.queues[r]; !found {
			q.queues[r] = Queue{make(map[string]State)}
		}
		qq := q.queues[r]
		// first pass: set all scraped object to expired
		for f, s := range qq.states {
			if s == scrapedState {
				qq.states[f] = expiredState
			}
		}
		// second pass: set observed object to scraped
		for _, f := range obs[r] {
			// log only for new and staged objects
			if qq.states[f] == newState || qq.states[f] >= stagedState {
				q.log.Debug().Str("replicaSet", r).Str("name", f).Int("state", int(scrapedState)).Msg("set state (scraped)")
			}
			qq.states[f] = scrapedState
		}
		// third pass: now the expired objects are truly expired, remove them
		for f, s := range qq.states {
			if s == expiredState {
				delete(qq.states, f)
				q.log.Debug().Str("replicaSet", r).Str("name", f).Int("state", int(s)).Msg("remove expired object")
			}
		}
		// Staged objects are not candidates to serve, so they should be garbage
		// collected.
		// Always increase the state of staged objects by one in each call of this
		// function, and delete them if their states are larger than
		// stagedState+2. So they are not garbage collected on the next call after
		// they are staged, rather they are collected in three calls.
		for f, s := range qq.states {
			if s > stagedState+State(2) {
				delete(qq.states, f)
				q.log.Debug().Str("replicaSet", r).Str("name", f).Int("state", int(s)).Msg("remove staged object")
			} else if s >= stagedState {
				qq.states[f] = s + 1
				q.log.Debug().Str("replicaSet", r).Str("name", f).Int("state", int(s+1)).Msg("set state")
			}
		}
	}
}

func (q *Monitor) setStatesAfterDiscovery(data map[string]interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for f := range data {
		for r, qq := range q.queues {
			if _, found := qq.states[f]; !found {
				qq.states[f] = newState
				q.log.Debug().Str("replicaSet", r).Str("name", f).Int("state", int(newState)).Msg("set state (new)")
			}
		}
	}
	q.data = data
	q.discoveredGauge.Set(float64(len(q.data)))
}

func (q *Monitor) NextName(r string) (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// initialize Queue for new replicaSet with filers in q.data
	if _, found := q.queues[r]; !found {
		q.queues[r] = Queue{make(map[string]State)}
		for f := range q.data {
			q.queues[r].states[f] = newState
		}
	}
	// Do not return any thing when queue is not ready
	if q.liveness >= 0 {
		states := q.queues[r].states
		for f, s := range states {
			if s == newState {
				states[f] = stagedState
				q.log.Debug().Str("replicaSet", r).Str("name", f).Int("state", int(stagedState)).Msg("set state (staged)")
				return f, true
			}
		}
	}
	return "", false
}

func (q *Monitor) NextItem(r string) (interface{}, bool) {
	f, ok := q.NextName(r)
	if ok {
		return q.data[f], true
	}
	return nil, false
}

func (q *Monitor) observe(promClient promquery.Client, promQuery, promLabel string) (map[string][]string, error) {
	resultVectors, err := promClient.GetVector(promQuery)
	if err != nil {
		return nil, err
	}
	obs := make(map[string][]string, 0)
	for _, m := range resultVectors {
		r := string(m.Metric[model.LabelName("pod_template_hash")])
		if _, found := obs[r]; !found {
			obs[r] = make([]string, 0)
		}
		v := m.Metric[model.LabelName(promLabel)]
		if v != "" {
			obs[r] = append(obs[r], string(v))
		}
	}
	return obs, nil
}
