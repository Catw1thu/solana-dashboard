package store

import (
	"context"
	"encoding/json"
	"fmt"
	"solana-dashboard-go/internal/db"
	"solana-dashboard-go/internal/events"
)

const insertServiceEventSQL = `
insert into service_events (
    event_id,
    schema_version,
    chain,
    protocol,
    event_type,
    commitment,
    slot,
    tx_signature,
    tx_index,
    instruction_source,
    outer_index,
    inner_index,
    event_source,
    event_unix_ts,
    refs,
    payload
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
on conflict (event_id) do nothing
`

type ServiceEventStore struct {
	db *db.DB
}

func NewServiceEventStore(db *db.DB) *ServiceEventStore {
	return &ServiceEventStore{db: db}
}

func (s *ServiceEventStore) InsertServiceEvent(ctx context.Context, event *events.Envelope) (bool, error) {
	refsJSON, err := json.Marshal(event.Refs)
	if err != nil {
		return false, fmt.Errorf("marshal refs: %w", err)
	}

	payloadJSON := []byte(event.Payload)
	tag, err := s.db.Pool.Exec(
		ctx, insertServiceEventSQL, event.EventID,
		event.SchemaVersion,
		event.Chain,
		event.Protocol,
		event.EventType,
		event.Commitment,
		int64(event.Slot),
		event.TxSignature,
		int64(event.TxIndex),
		event.InstructionPath.Source,
		event.InstructionPath.OuterIndex,
		event.InstructionPath.InnerIndex,
		event.EventSource,
		event.EventUnixTS,
		refsJSON,
		payloadJSON,
	)
	if err != nil {
		return false, fmt.Errorf("insert service event: %w", err)
	}

	return tag.RowsAffected() == 1, nil
}
