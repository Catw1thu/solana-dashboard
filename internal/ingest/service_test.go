package ingest

import (
	"context"
	"encoding/json"
	"testing"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/realtime"
)

type mockStore struct {
	inserted bool
	err      error
}

func (m *mockStore) InsertServiceEvent(ctx context.Context, event *events.Envelope) (bool, error) {
	return m.inserted, m.err
}

func TestHandleEventAcceptsPumpAmmSwap(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	service := NewService(hub, store)
	defer service.Close()
	ch := hub.Subscribe(1)
	defer hub.Unsubscribe(ch)

	event := events.Envelope{
		EventID:   "solana:pumpamm:swap:testsig:outer:1",
		Protocol:  "pumpamm",
		EventType: "swap",
		Payload: json.RawMessage(`{
			"side":"sell",
			"ix_name":"sell",
			"pool":"pool_1",
			"user":"user_1",
			"base_mint":"base_1",
			"quote_mint":"quote_1",
			"coin_creator":"creator_1",
			"base_amount_in":"123",
			"base_amount_out":null,
			"quote_amount_in":null,
			"quote_amount_out":"456",
			"lp_fee":"3",
			"protocol_fee":"4",
			"coin_creator_fee":"5",
			"cashback":"0",
			"pool_base_token_reserves":"1000",
			"pool_quote_token_reserves":"2000",
			"instruction_args":{
				"base_amount_in":"123",
				"min_quote_amount_out":"450",
				"base_amount_out":null,
				"max_quote_amount_in":null,
				"spendable_quote_in":null,
				"min_base_amount_out":null
			}
		}`),
	}

	if err := service.HandleEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}

	select {
	case got := <-ch:
		if got.EventID != event.EventID {
			t.Fatalf("expected event_id=%s, got %s", event.EventID, got.EventID)
		}
	default:
		t.Fatal("expected handled event to be published to hub")
	}
}
