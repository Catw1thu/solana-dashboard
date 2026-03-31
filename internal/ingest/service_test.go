package ingest

import (
	"context"
	"encoding/json"
	"testing"

	"solana-dashboard-go/internal/events"
)

func TestHandleEventAcceptsPumpAmmSwap(t *testing.T) {
	service := NewService()

	env := events.Envelope{
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

	if err := service.HandleEvent(context.Background(), env); err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}
}
