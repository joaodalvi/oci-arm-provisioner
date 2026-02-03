package notifier

import (
	"sync"
	"time"
)

type Tracker struct {
	mu              sync.Mutex
	StartTime       time.Time
	TotalCycles     int
	CapacityErrors  int
	OtherErrors     int
	SuccessCount    int
	LastSuccessTime time.Time
}

func NewTracker() *Tracker {
	return &Tracker{
		StartTime: time.Now(),
	}
}

func (t *Tracker) IncCycle() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalCycles++
}

func (t *Tracker) IncCapacity() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CapacityErrors++
}

func (t *Tracker) IncError() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.OtherErrors++
}

func (t *Tracker) IncSuccess() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.SuccessCount++
	t.LastSuccessTime = time.Now()
}

func (t *Tracker) Snapshot() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Stats{
		StartTime:       t.StartTime,
		TotalCycles:     t.TotalCycles,
		CapacityErrors:  t.CapacityErrors,
		OtherErrors:     t.OtherErrors,
		SuccessCount:    t.SuccessCount,
		LastSuccessTime: t.LastSuccessTime,
	}
}
