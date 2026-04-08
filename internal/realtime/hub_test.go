package realtime

import (
	"testing"

	"solana-dashboard-go/internal/events"
)

func TestHubPublishDeliversEventToSubscriber(t *testing.T) {
	hub := NewHub()
	ch := hub.Subscribe("global", 1)
	defer hub.Unsubscribe(ch)

	event := events.Envelope{
		EventID:   "event_1",
		Protocol:  "pumpfun",
		EventType: "trade",
	}

	hub.Publish("global", event)

	select {
	case got := <-ch:
		if got.EventID != event.EventID {
			t.Fatalf("expected event_id=%s, got %s", event.EventID, got.EventID)
		}
	default:
		t.Fatal("expected published event to be delivered")
	}
}
