package utils

import "time"

type TickTick struct {
	delay bool
}

func (t *TickTick) After(d time.Duration) <-chan time.Time {
	if !t.delay {
		t.delay = true
		return time.After(0)
	} else {
		return time.After(d)
	}
}
