package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"solana-dashboard-go/internal/ingest"
	"testing"
)

func TestIngestEventAcceptsValidPumpfunTrade(t *testing.T) {
	service := ingest.NewService()
	handler := NewHandler(service)

	body := []byte(`{
		"schema_version":1,
		"event_id":"solana:pumpfun:trade:testsig:outer:1",
		"chain":"solana",
		"protocol":"pumpfun",
		"event_type":"trade",
		"commitment":"processed",
		"slot":1,
		"tx_signature":"testsig",
		"tx_index":0,
		"instruction_path":{
			"source":"outer",
			"outer_index":1,
			"inner_index":null
		},
		"event_source":"logs",
		"event_unix_ts":1770000000,
		"refs":{
			"mint":"mint_1",
			"pool":null,
			"bonding_curve":"curve_1",
			"user":"user_1",
			"creator":"creator_1",
			"base_mint":null,
			"quote_mint":null,
			"lp_mint":null
		},
		"payload":{
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
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/internal/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.IngestEvent(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestIngestEventRejectsInvalidJSON(t *testing.T) {
	service := ingest.NewService()
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodPost, "/internal/events", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.IngestEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
