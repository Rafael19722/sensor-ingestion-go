package main

import (
	"math"
	"sync"
	"time"
)

type sigState struct {
	temp float64
	t    time.Time
}

type significanceTracker struct {
	mu        sync.Mutex
	last      map[string]sigState
	heartbeat time.Duration
	deltaC    float64
}

func newSignificanceTracker(heartbeat time.Duration, deltaC float64) *significanceTracker {
	return &significanceTracker{
		last:      make(map[string]sigState),
		heartbeat: heartbeat,
		deltaC:    deltaC,
	}
}

func (s *significanceTracker) evaluate(mac string, temp float64, t time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev, seen := s.last[mac]
	significant := !seen ||
		t.Sub(prev.t) >= s.heartbeat ||
		math.Abs(temp-prev.temp) >= s.deltaC

	if significant {
		s.last[mac] = sigState{temp: temp, t: t}
	}
	return significant
}
