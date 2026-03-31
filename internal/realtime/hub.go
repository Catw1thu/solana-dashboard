package realtime

import (
	"solana-dashboard-go/internal/events"
	"sync"
)

type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan events.Envelope]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[chan events.Envelope]struct{}),
	}
}

func (h *Hub) Subscribe(buffer int) chan events.Envelope {
	ch := make(chan events.Envelope, buffer)

	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	return ch
}

func (h *Hub) Unsubscribe(ch chan events.Envelope) {
	h.mu.Lock()
	if _, ok := h.subscribers[ch]; ok {
		delete(h.subscribers, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *Hub) Publish(event events.Envelope) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:

		}
	}
}
