package realtime

import (
	"solana-dashboard-go/internal/events"
	"sync"
)

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan events.Envelope]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan events.Envelope]struct{}),
	}
}

func (h *Hub) Subscribe(topic string, buffer int) chan events.Envelope {
	ch := make(chan events.Envelope, buffer)

	h.mu.Lock()
	if _, ok := h.subscribers[topic]; !ok {
		h.subscribers[topic] = make(map[chan events.Envelope]struct{})
	}
	h.subscribers[topic][ch] = struct{}{}
	h.mu.Unlock()

	return ch
}

// Unsubscribe removes a channel from all topics it was subscribed to.
func (h *Hub) Unsubscribe(ch chan events.Envelope) {
	h.mu.Lock()
	closed := false
	for topic, subs := range h.subscribers {
		if _, ok := subs[ch]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(h.subscribers, topic)
			}
			if !closed {
				close(ch)
				closed = true
			}
		}
	}
	h.mu.Unlock()
}

// Publish sends an event to all subscribers of the specified topic
func (h *Hub) Publish(topic string, event events.Envelope) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subs, ok := h.subscribers[topic]
	if !ok {
		return
	}

	for ch := range subs {
		select {
		case ch <- event:
		default:
			// Buffer full, drop event
		}
	}
}
