package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/sapcc/go-bits/promquery"
	"github.com/sapcc/netappsd/pkg/monitor/netapp"
)

type State int32

const (
	unknownState State = -1
	newState     State = 0
	scrapedState State = 2
	stagedState  State = 3
)

type Queue struct {
	states map[string]State
}

type Monitor struct {
	Discoverer
	queues            map[string]Queue
	discovered        map[string]interface{}
	discoveredGauge   prometheus.Gauge
	probeFailureGauge *prometheus.GaugeVec
	liveness          int
	log               *zerolog.Logger
	mu                sync.Mutex
}

func NewMonitorQueue(d Discoverer, metricsPrefix string, log *zerolog.Logger) *Monitor {
	q := Monitor{
		Discoverer: d, liveness: 0, log: log,
	}
	q.queues = make(map[string]Queue, 0)
	q.InitMetrics(metricsPrefix)
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

	for {
		func() {
			items, err := q.observe(promClient, promQ, promL)
			if err != nil {
				q.log.Err(err).Send()
				q.liveness = q.liveness - 1
				return
			}
			q.setStatesAfterObserving(items)

			// Observer with negative liveness is considered not live. Above,
			// liveness is not set to -1 immediately after observe fails with error.
			// Rather it is decreased by one to allow flappy prometheus connections.
			// Set liveness to a non-negative value when observe is done
			// successfully. The higher liveness is, more tolerance to prometheus
			// connection there is.
			q.liveness = 1
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

	for {
		func() {
			data, warns, err := q.Discover(region, query)
			if err != nil {
				q.log.Err(err).Send()
				return
			}
			q.probeFailureGauge.Reset()
			for _, e := range warns {
				ne, _ := e.(*netapp.NetappDiscovererError)
				q.log.Warn().Str("Host", ne.Host).Msg(ne.Error())
				q.probeFailureGauge.WithLabelValues(ne.Host, ne.Reason).Set(1)
			}
			q.setObservedObjects(data)
		}()

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

// updateQueueByDiscovered updates states based on the discovered items.
// Unknown items are set to newState, known items, that are not
// discovered, are set to unknownState.
func (q *Monitor) updateQueueByDiscovered() {
	if q.discovered == nil {
		return
	}
	// set new item to netState
	for n := range q.discovered {
		for _, qq := range q.queues {
			if _, found := qq.states[n]; !found {
				qq.states[n] = newState
			}
		}
	}
	for _, qq := range q.queues {
		for n := range qq.states {
			if _, found := q.discovered[n]; !found {
				qq.states[n] = unknownState
			}
		}
	}
}

func (q *Monitor) setStatesAfterObserving(obs map[string][]string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	log := q.log

	// debug obs
	for r := range obs {
		for _, n := range obs[r] {
			log.Trace().Str("replica", r).Str("name", n).Msg("observed items")
		}
	}

	// pass 0: set state based on discovered
	// initialize Q for replica
	for r := range obs {
		if _, found := q.queues[r]; !found {
			q.queues[r] = Queue{make(map[string]State)}
		}
	}
	// set first discovered item to newState
	// set lost items to unknownState
	q.updateQueueByDiscovered()

	// pass 1: give retention to scraped and staged
	// scrapedState ->  scrapedState-1 -> ... -> 0
	// stagedState -> stagedState+1 -> ... -> stagedState+3 -> newState
	for r, qq := range q.queues {
		for n, s := range qq.states {
			switch {
			case s > newState && s <= scrapedState:
				qq.states[n] = s - 1
				if s-1 == 0 {
					log.Debug().Str("replicaSet", r).Str("name", n).Int("oldState", int(s)).Int("state", int(s-1)).Msg("reset state (new)")
				}
			case s >= stagedState && s <= stagedState+2:
				qq.states[n] = s + 1
				log.Debug().Str("replicaSet", r).Str("name", n).Int("state", int(s+1)).Msg("set state")
			case s == stagedState+3:
				qq.states[n] = newState
				log.Debug().Str("replicaSet", r).Str("name", n).Int("oldState", int(s)).Int("state", int(newState)).Msg("reset state (new)")
			}
		}
	}

	// pass 2: set state based on observed
	// observed -> scrapedState
	for r, observed := range obs {
		for _, n := range observed {
			if s, found := q.queues[r].states[n]; !found {
				log.Debug().Str("replicaSet", r).Str("name", n).Int("state", int(scrapedState)).Msg("set state (scraped)")
			} else {
				if s == newState || s >= stagedState {
					log.Debug().Str("replicaSet", r).Str("name", n).Int("oldState", int(s)).Int("state", int(scrapedState)).Msg("set state (scraped)")
				}
			}
			q.queues[r].states[n] = scrapedState
		}
	}

	// debug queues
	for r, qq := range q.queues {
		for n, s := range qq.states {
			log.Trace().Str("replica", r).Str("name", n).Int("state", int(s)).Msg("queue")
		}
	}
}

func (q *Monitor) setObservedObjects(discovered map[string]interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// deep copy discovered data to Monitor
	q.discovered = discovered
	q.discoveredGauge.Set(float64(len(q.discovered)))
}

func (q *Monitor) NextName(r string) (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// initialize replicaSet if they are not in queue yet
	if _, found := q.queues[r]; !found {
		q.queues[r] = Queue{make(map[string]State)}
	}
	// when queue is not live, we are not sure about the states
	if q.liveness >= 0 {
		qq := q.queues[r]
		for f, s := range qq.states {
			if s == newState {
				qq.states[f] = stagedState
				q.log.Debug().Str("replicaSet", r).Str("name", f).Int("oldState", int(s)).Int("state", int(stagedState)).Msg("set state (staged)")
				return f, true
			}
		}
	}
	return "", false
}

func (q *Monitor) NextItem(r string) (interface{}, bool) {
	f, ok := q.NextName(r)
	if ok {
		return q.discovered[f], true
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
