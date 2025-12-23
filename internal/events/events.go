package events

import (
	"sync"
	"time"
)

type Event struct {
	DeviceID string
	Topic    string
	Payload  []byte
	Time     time.Time
}

type Buffer interface {
	Push(e Event)
	Pull(after time.Time, max int) []Event
}

type ring struct {
	mu   sync.RWMutex
	data []Event
	size int
}

func NewRing(size int) Buffer {
	return &ring{data: make([]Event, 0, size), size: size}
}

func (r *ring) Push(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.data) == r.size {
		r.data = r.data[1:]
	}
	r.data = append(r.data, e)
}

func (r *ring) Pull(after time.Time, max int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Event, 0, max)
	for i := len(r.data) - 1; i >= 0 && len(out) < max; i-- {
		if r.data[i].Time.After(after) {
			out = append(out, r.data[i])
		}
	}
	// вернуть по возрастанию времени
	for l, rgt := 0, len(out)-1; l < rgt; l, rgt = l+1, rgt-1 {
		out[l], out[rgt] = out[rgt], out[l]
	}
	return out
}
