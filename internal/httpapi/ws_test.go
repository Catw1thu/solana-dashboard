package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/ingest"
	"solana-dashboard-go/internal/realtime"
)

type wsMockProjector struct {
	err error
}

func (m *wsMockProjector) Project(ctx context.Context, event *events.Envelope, payload any) error {
	return m.err
}

func TestServeWSPublishesRealtimeEvent(t *testing.T) {
	hub := realtime.NewHub()
	store := &mockStore{inserted: true}
	projector := &wsMockProjector{}
	service := ingest.NewService(hub, store, projector)
	handler := NewHandler(service)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handler.ServeWS)

	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test complete")

	event := events.Envelope{
		SchemaVersion: 1,
		EventID:       "solana:pumpfun:trade:testsig:outer:1",
		Chain:         "solana",
		Protocol:      "pumpfun",
		EventType:     "trade",
		Commitment:    "processed",
		Slot:          1,
		TxSignature:   "testsig",
		TxIndex:       0,
		InstructionPath: events.InstructionPath{
			Source:     "outer",
			OuterIndex: 1,
			InnerIndex: nil,
		},
		EventSource: "logs",
		EventUnixTS: 1770000000,
		Refs: events.EventRefs{
			Mint:         stringPtr("mint_1"),
			Pool:         nil,
			BondingCurve: stringPtr("curve_1"),
			User:         stringPtr("user_1"),
			Creator:      stringPtr("creator_1"),
			BaseMint:     nil,
			QuoteMint:    nil,
			LPMint:       nil,
		},
		Payload: json.RawMessage(`{
			"side":"buy",
			"ix_name":"buy",
			"mint":"mint_1",
			"user":"user_1",
			"bonding_curve":"curve_1",
			"creator":"creator_1",
			"creator_vault":"vault_1",
			"token_program":"token_program_1",
			"sol_amount":"100",
			"token_amount":"200",
			"fee":"1",
			"creator_fee":"2",
			"virtual_sol_reserves":"300",
			"virtual_token_reserves":"400",
			"real_sol_reserves":"500",
			"real_token_reserves":"600",
			"track_volume":true,
			"mayhem_mode":false,
			"cashback":"0",
			"instruction_args":{
				"amount":"1000",
				"max_sol_cost":"2000",
				"min_sol_output":null,
				"spendable_sol_in":null,
				"min_tokens_out":null
			}
		}`),
	}

	if err := service.HandleEvent(ctx, event); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}

	var got events.Envelope
	if err := wsjson.Read(ctx, conn, &got); err != nil {
		t.Fatalf("failed to read websocket event: %v", err)
	}

	if got.EventID != event.EventID {
		t.Fatalf("expected event_id=%s, got %s", event.EventID, got.EventID)
	}

	if got.Protocol != "pumpfun" {
		t.Fatalf("expected protocol=pumpfun, got %s", got.Protocol)
	}
}

func stringPtr(v string) *string {
	return &v
}
