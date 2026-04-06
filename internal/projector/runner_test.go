package projector

import (
	"context"
	"encoding/json"
	"testing"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/store"
)

type replayLogReader struct {
	checkpoint int64
	entries    []store.ServiceEventLogEntry
	saved      []int64
}

func (r *replayLogReader) ListServiceEventsAfterLogID(ctx context.Context, afterLogID int64, limit int) ([]store.ServiceEventLogEntry, error) {
	filtered := make([]store.ServiceEventLogEntry, 0, limit)
	for _, entry := range r.entries {
		if entry.LogID <= afterLogID {
			continue
		}
		filtered = append(filtered, entry)
		if len(filtered) == limit {
			break
		}
	}
	return filtered, nil
}

func (r *replayLogReader) LoadProjectionCheckpoint(ctx context.Context, projectorName string) (int64, error) {
	return r.checkpoint, nil
}

func (r *replayLogReader) SaveProjectionCheckpoint(ctx context.Context, projectorName string, lastLogID int64) error {
	r.saved = append(r.saved, lastLogID)
	r.checkpoint = lastLogID
	return nil
}

type replayReadModelWriter struct {
	tokens     []store.TokenRecord
	metadata   []store.TokenMetadataRecord
	markets    []store.TokenMarketRecord
	trades     []store.TradeEventRecord
	activities []store.ActivityEventRecord
}

func (w *replayReadModelWriter) UpsertToken(ctx context.Context, record store.TokenRecord) error {
	w.tokens = append(w.tokens, record)
	return nil
}

func (w *replayReadModelWriter) UpsertTokenMetadata(ctx context.Context, record store.TokenMetadataRecord) error {
	w.metadata = append(w.metadata, record)
	return nil
}

func (w *replayReadModelWriter) UpsertTokenMarket(ctx context.Context, record store.TokenMarketRecord) error {
	w.markets = append(w.markets, record)
	return nil
}

func (w *replayReadModelWriter) CloseTokenMarket(ctx context.Context, marketID string, endedAt int64) error {
	return nil
}

func (w *replayReadModelWriter) InsertTradeEvent(ctx context.Context, trade store.TradeEventRecord) error {
	w.trades = append(w.trades, trade)
	return nil
}

func (w *replayReadModelWriter) InsertActivityEvent(ctx context.Context, activity store.ActivityEventRecord) error {
	w.activities = append(w.activities, activity)
	return nil
}

func TestRunnerReplayBatchProjectsAndSavesCheckpoint(t *testing.T) {
	reader := &replayLogReader{
		entries: []store.ServiceEventLogEntry{
			{
				LogID: 7,
				Event: events.Envelope{
					EventID:     "solana:pumpfun:create:testsig:outer:1",
					Protocol:    "pumpfun",
					EventType:   "create",
					EventUnixTS: 1770000000,
					TxSignature: "testsig",
					InstructionPath: events.InstructionPath{
						Source:     "outer",
						OuterIndex: 1,
					},
					Refs: events.EventRefs{
						Mint:         stringPtr("mint_1"),
						BondingCurve: stringPtr("curve_1"),
						Creator:      stringPtr("creator_1"),
					},
					Payload: json.RawMessage(`{
						"ix_name":"create",
						"mint":"mint_1",
						"bonding_curve":"curve_1",
						"user":"user_1",
						"creator":"creator_1",
						"name":"Token",
						"symbol":"TKN",
						"uri":"https://example.com/meta.json",
						"token_program":"token_program_1",
						"virtual_token_reserves":"1000",
						"virtual_sol_reserves":"2000",
						"real_token_reserves":"3000",
						"token_total_supply":"4000",
						"is_mayhem_mode":false,
						"is_cashback_enabled":false
					}`),
				},
			},
		},
	}
	readModel := &replayReadModelWriter{}
	runner := NewRunner("token_read_model", reader, New(readModel))

	nextLogID, processed, err := runner.ReplayBatch(context.Background(), 0)
	if err != nil {
		t.Fatalf("ReplayBatch returned error: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected processed=1, got %d", processed)
	}
	if nextLogID != 7 {
		t.Fatalf("expected nextLogID=7, got %d", nextLogID)
	}
	if len(readModel.tokens) != 1 {
		t.Fatalf("expected 1 token upsert, got %d", len(readModel.tokens))
	}
	if len(readModel.metadata) != 1 {
		t.Fatalf("expected 1 metadata upsert, got %d", len(readModel.metadata))
	}
	if len(readModel.markets) != 1 {
		t.Fatalf("expected 1 market upsert, got %d", len(readModel.markets))
	}
	if len(readModel.activities) != 1 {
		t.Fatalf("expected 1 activity insert, got %d", len(readModel.activities))
	}
	if len(reader.saved) != 1 || reader.saved[0] != 7 {
		t.Fatalf("expected checkpoint save [7], got %#v", reader.saved)
	}
}

func stringPtr(v string) *string {
	return &v
}
