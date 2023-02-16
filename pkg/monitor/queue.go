package monitor

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type State int32

const (
	newState     State = 0
	scrapedState State = 1
	expiredState State = 2
	stagedState  State = 3
)

var (
	output = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	// logger = zerolog.New(output).With().Caller().Timestamp().Logger()
	logger = zerolog.New(output).With().Timestamp().Logger()
)

type MonitorQueue struct {
	Watcher
	mu       sync.Mutex
	wg       sync.WaitGroup
	liveness int
	tplDir   string
	data     map[string]interface{}
	states   map[string]State
}

func NewMonitorQueue(w Watcher, tmplDir string) *MonitorQueue {
	states := make(map[string]State, 0)
	q := MonitorQueue{
		Watcher: w, liveness: 0, states: states, tplDir: tmplDir,
	}
	q.wg.Add(1)
	return &q
}

func (q *MonitorQueue) DoObserve(ctx context.Context, interval time.Duration, promQ, labelName string) error {
	log.Printf("Observer runs every %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Starts Discoverer when Observer is ready
	ready := false

	for {
		func() {
			items, err := q.Observe(promQ, labelName)
			if err != nil {
				logger.Err(err).Send()
				q.liveness = q.liveness - 1
				return
			}
			if !ready {
				log.Debug().Msg("Observer is ready")
				ready = true
				q.wg.Done()
			}
			q.liveness = 1
			q.setStatesAfterObserving(items)
		}()

		if q.liveness < 0 {
			logger.Warn().Msg("Discoverer has problems with observing metrics")
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func (q *MonitorQueue) DoDiscover(ctx context.Context, interval time.Duration, region, query string) error {
	log.Printf("Discoverer runs every %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Wait for Observer ready
	q.wg.Wait()
	log.Debug().Msg("Discoverer starts")

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
				log.Err(err).Send()
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

func (q *MonitorQueue) setStatesAfterObserving(obs []string) {
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
			logger.Debug().Str("name", f).Int("state", int(scrapedState)).Msg("set state (scraped)")
		}
		q.states[f] = scrapedState
	}
	// third pass: now the expired objects are truly expired, remove them
	for f, s := range q.states {
		if s == expiredState {
			delete(q.states, f)
			logger.Debug().Str("name", f).Int("state", int(s)).Msg("remove expired object")
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
			logger.Debug().Str("name", f).Int("state", int(s)).Msg("remove staged object")
		} else if s >= stagedState {
			q.states[f] = s + 1
			logger.Debug().Str("name", f).Int("state", int(s+1)).Msg("set state")
		}
	}
	workedItems.Set(float64(len(obs)))
}

func (q *MonitorQueue) setStatesAfterDiscovery(data map[string]interface{}) {
	for f := range data {
		if _, found := q.states[f]; !found {
			q.states[f] = newState
			logger.Debug().Str("name", f).Int("state", int(newState)).Msg("set state (new)")
		}
	}
	q.data = data
	totalItems.Set(float64(len(q.data)))
}

func (q *MonitorQueue) NextName() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// Do not return any thing when queue is not ready
	if q.liveness >= 0 {
		for f, s := range q.states {
			if s == newState {
				q.states[f] = stagedState
				logger.Debug().Str("name", f).Int("state", int(stagedState)).Msg("set state (staged)")
				return f, true
			}
		}
	}
	return "", false
}

func (q *MonitorQueue) NextItem() (interface{}, bool) {
	f, ok := q.NextName()
	if ok {
		return q.data[f], true
	}
	return nil, false
}
