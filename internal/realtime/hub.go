package realtime

import (
	"solana-dashboard-go/internal/events"
	"sync"
)

type Subscription struct {
	Events   chan events.Envelope
	Overflow chan struct{}

	mu         sync.Mutex
	closed     bool
	overflowed bool
}

func newSubscription(buffer int) *Subscription {
	return &Subscription{
		Events:   make(chan events.Envelope, buffer),
		Overflow: make(chan struct{}, 1),
	}
}

func (s *Subscription) markOverflow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.overflowed {
		return false
	}

	s.overflowed = true
	select {
	case s.Overflow <- struct{}{}:
	default:
	}
	close(s.Events)
	return true
}

func (s *Subscription) close() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.closed = true
	if !s.overflowed {
		close(s.Events)
	}
	close(s.Overflow)
	return true
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*Subscription]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[*Subscription]struct{}),
	}
}

func (h *Hub) Subscribe(topic string, buffer int) *Subscription {
	sub := newSubscription(buffer)

	h.mu.Lock()
	if _, ok := h.subscribers[topic]; !ok {
		h.subscribers[topic] = make(map[*Subscription]struct{})
	}
	h.subscribers[topic][sub] = struct{}{}
	h.mu.Unlock()

	return sub
}

// Unsubscribe removes a subscription from all topics it was subscribed to.
func (h *Hub) Unsubscribe(sub *Subscription) {
	if sub == nil {
		return
	}

	h.mu.Lock()
	for topic, subs := range h.subscribers {
		if _, ok := subs[sub]; ok {
			delete(subs, sub)
			if len(subs) == 0 {
				delete(h.subscribers, topic)
			}
		}
	}
	h.mu.Unlock()

	sub.close()
}

func (h *Hub) SubscriberCount(topic string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.subscribers[topic])
}

func (h *Hub) HasSubscribers(topic string) bool {
	return h.SubscriberCount(topic) > 0
}

func (h *Hub) TotalSubscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := 0
	for _, subs := range h.subscribers {
		total += len(subs)
	}
	return total
}

// Publish sends an event to all subscribers of the specified topic.
func (h *Hub) Publish(topic string, event events.Envelope) {
	h.mu.RLock()
	subs, ok := h.subscribers[topic]
	if !ok {
		h.mu.RUnlock()
		return
	}

	overflowed := make([]*Subscription, 0)
	for sub := range subs {
		select {
		case sub.Events <- event:
		default:
			overflowed = append(overflowed, sub)
		}
	}
	h.mu.RUnlock()

	if len(overflowed) == 0 {
		return
	}

	h.mu.Lock()
	for _, sub := range overflowed {
		if subs, ok := h.subscribers[topic]; ok {
			delete(subs, sub)
			if len(subs) == 0 {
				delete(h.subscribers, topic)
			}
		}
		sub.markOverflow()
	}
	h.mu.Unlock()
}
