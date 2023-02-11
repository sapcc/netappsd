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
	logger = zerolog.New(output).With().Caller().Timestamp().Logger()
)

type MonitorQueue struct {
	Watcher
	mu     sync.Mutex
	ready  bool
	states map[string]State
	data   map[string]interface{}
	tplDir string
}

func NewMonitorQueue(w Watcher, tmplDir string) *MonitorQueue {
	states := make(map[string]State, 0)
	return &MonitorQueue{
		Watcher: w, ready: false, states: states, tplDir: tmplDir,
	}
}

func (q *MonitorQueue) DoObserve(ctx context.Context, interval time.Duration, promQ, labelName string) error {
	log.Printf("observe metrics every %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		func() {
			items, err := q.Observe(promQ, labelName)
			if err != nil {
				logger.Err(err).Send()
				return
			}
			q.setStatesAfterObserving(items)
		}()

		for n, s := range q.states {
			logger.Print(q.data[n], s)
		}
		logger.Printf("queue readiness = %t", q.ready)

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func (q *MonitorQueue) DoDiscover(ctx context.Context, interval time.Duration, region, query string) error {
	log.Printf("discover new objects every %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		func() {
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

func (q *MonitorQueue) NextName() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for f, s := range q.states {
		if s == newState {
			return f, true
		}
	}
	return "", false
}

func (q *MonitorQueue) NextItem() (interface{}, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for f, s := range q.states {
		if s == newState {
			return q.data[f], true
		}
	}
	return nil, false
}

func (q *MonitorQueue) setStatesAfterObserving(obs []string) {
	q.mu.Lock()
	defer func() {
		q.ready = true
		q.mu.Unlock()
	}()
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
			logger.Debug().Str("name", f).Int("state", int(scrapedState)).Msg("set state")
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
			q.states[f] += 1
			logger.Debug().Str("name", f).Int("state", int(s)).Msg("set state")
		}
	}

	// set queue readiness to true
	q.ready = true
}

func (q *MonitorQueue) setStatesAfterDiscovery(data map[string]interface{}) {
	if !q.ready {
		return
	}
	for f := range data {
		if _, found := q.states[f]; !found {
			q.states[f] = newState
			logger.Debug().Str("name", f).Int("state", int(newState)).Msg("add object to queue")
			// set readiness to false when new filer is found
			// queue should not serve any filer before next Observe is executed
			q.ready = false
		}
	}
	q.data = data
}
