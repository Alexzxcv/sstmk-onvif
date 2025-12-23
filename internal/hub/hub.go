package hub

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type Command struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type deviceState struct {
	q    chan Command
	last time.Time
}

type Hub struct {
	mu  sync.RWMutex
	dev map[string]*deviceState
}

func New() *Hub { return &Hub{dev: map[string]*deviceState{}} }

func (h *Hub) get(id string) *deviceState {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.dev[id]
	if !ok {
		s = &deviceState{q: make(chan Command, 64)}
		h.dev[id] = s
	}
	s.last = time.Now()
	return s
}

func (h *Hub) Enqueue(id string, c Command) {
	s := h.get(id)
	select {
	case s.q <- c:
	default: /* очередь заполнена — можно залогировать */
	}
}

func (h *Hub) LongPoll(ctx context.Context, id string) []Command {
	s := h.get(id)
	select {
	case c := <-s.q:
		cmds := []Command{c}
		for i := 0; i < 31; i++ {
			select {
			case c2 := <-s.q:
				cmds = append(cmds, c2)
			default:
				return cmds
			}
		}
		return cmds
	case <-ctx.Done():
		return nil
	}
}

func (h *Hub) LastSeen(id string) time.Time { return h.get(id).last }
