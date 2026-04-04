package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"solana-dashboard-go/internal/db"
	"solana-dashboard-go/internal/events"
	"time"

	"github.com/jackc/pgx/v5"
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

const listServiceEventsAfterLogIDSQL = `
select
    log_id,
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
    payload,
    created_at
from service_events
where log_id > $1
order by log_id asc
limit $2
`

const loadProjectionCheckpointSQL = `
select last_log_id
from projection_checkpoints
where projector_name = $1
`

const listServiceEventsByMintSQL = `
select
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
    payload,
    created_at
from service_events
where refs->>'mint' = $1
order by slot desc, tx_index desc, outer_index desc, inner_index desc nulls last
limit $2
`

const listServiceEventsByPoolSQL = `
select
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
    payload,
    created_at
from service_events
where refs->>'pool' = $1
order by slot desc, tx_index desc, outer_index desc, inner_index desc nulls last
limit $2
`

const listLatestCreateEventByMintSQL = `
select
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
    payload,
    created_at
from service_events
where refs->>'mint' = $1
  and event_type = 'create'
order by slot desc, tx_index desc, outer_index desc, inner_index desc nulls last
limit 1
`

const saveProjectionCheckpointSQL = `
insert into projection_checkpoints (projector_name, last_log_id, updated_at)
values ($1, $2, now())
on conflict (projector_name) do update set
    last_log_id = excluded.last_log_id,
    updated_at = now()
`

type ServiceEventStore struct {
	db *db.DB
}

type ServiceEventLogEntry struct {
	LogID     int64
	Event     events.Envelope
	CreatedAt time.Time
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

func (s *ServiceEventStore) ListServiceEventsAfterLogID(
	ctx context.Context,
	afterLogID int64,
	limit int,
) ([]ServiceEventLogEntry, error) {
	rows, err := s.db.Pool.Query(ctx, listServiceEventsAfterLogIDSQL, afterLogID, limit)
	if err != nil {
		return nil, fmt.Errorf("query service events after log_id=%d: %w", afterLogID, err)
	}
	defer rows.Close()

	entries := make([]ServiceEventLogEntry, 0, limit)
	for rows.Next() {
		var (
			entry          ServiceEventLogEntry
			slot           int64
			txIndex        int64
			outerIndex     int32
			innerIndex     *int32
			refsJSON       []byte
			payloadJSON    []byte
			source         string
			eventSource    string
			instructionRef events.InstructionPath
		)

		err := rows.Scan(
			&entry.LogID,
			&entry.Event.EventID,
			&entry.Event.SchemaVersion,
			&entry.Event.Chain,
			&entry.Event.Protocol,
			&entry.Event.EventType,
			&entry.Event.Commitment,
			&slot,
			&entry.Event.TxSignature,
			&txIndex,
			&source,
			&outerIndex,
			&innerIndex,
			&eventSource,
			&entry.Event.EventUnixTS,
			&refsJSON,
			&payloadJSON,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan service event log row: %w", err)
		}

		entry.Event.Slot = uint64(slot)
		entry.Event.TxIndex = uint64(txIndex)
		instructionRef.Source = source
		instructionRef.OuterIndex = int(outerIndex)
		if innerIndex != nil {
			value := int(*innerIndex)
			instructionRef.InnerIndex = &value
		}
		entry.Event.InstructionPath = instructionRef
		entry.Event.EventSource = eventSource
		entry.Event.Payload = payloadJSON
		if err := json.Unmarshal(refsJSON, &entry.Event.Refs); err != nil {
			return nil, fmt.Errorf("unmarshal service event refs for log_id=%d: %w", entry.LogID, err)
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service event log rows: %w", err)
	}

	return entries, nil
}

func (s *ServiceEventStore) LoadProjectionCheckpoint(
	ctx context.Context,
	projectorName string,
) (int64, error) {
	var lastLogID int64
	err := s.db.Pool.QueryRow(ctx, loadProjectionCheckpointSQL, projectorName).Scan(&lastLogID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("load projection checkpoint for %s: %w", projectorName, err)
	}

	return lastLogID, nil
}

func (s *ServiceEventStore) SaveProjectionCheckpoint(
	ctx context.Context,
	projectorName string,
	lastLogID int64,
) error {
	_, err := s.db.Pool.Exec(ctx, saveProjectionCheckpointSQL, projectorName, lastLogID)
	if err != nil {
		return fmt.Errorf("save projection checkpoint for %s: %w", projectorName, err)
	}

	return nil
}

func (s *ServiceEventStore) ListServiceEventsByMint(
	ctx context.Context,
	mint string,
	limit int,
) ([]events.Envelope, error) {
	rows, err := s.db.Pool.Query(ctx, listServiceEventsByMintSQL, mint, limit)
	if err != nil {
		return nil, fmt.Errorf("query service events by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	entries, err := scanServiceEvents(rows, limit)
	if err != nil {
		return nil, err
	}

	eventsList := make([]events.Envelope, 0, len(entries))
	for _, entry := range entries {
		eventsList = append(eventsList, entry.Event)
	}

	return eventsList, nil
}

func (s *ServiceEventStore) ListServiceEventsByPool(
	ctx context.Context,
	pool string,
	limit int,
) ([]events.Envelope, error) {
	rows, err := s.db.Pool.Query(ctx, listServiceEventsByPoolSQL, pool, limit)
	if err != nil {
		return nil, fmt.Errorf("query service events by pool=%s: %w", pool, err)
	}
	defer rows.Close()

	entries, err := scanServiceEvents(rows, limit)
	if err != nil {
		return nil, err
	}

	eventsList := make([]events.Envelope, 0, len(entries))
	for _, entry := range entries {
		eventsList = append(eventsList, entry.Event)
	}

	return eventsList, nil
}

func (s *ServiceEventStore) FindLatestCreateEventByMint(
	ctx context.Context,
	mint string,
) (*events.Envelope, error) {
	rows, err := s.db.Pool.Query(ctx, listLatestCreateEventByMintSQL, mint)
	if err != nil {
		return nil, fmt.Errorf("query latest create event by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	entries, err := scanServiceEvents(rows, 1)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	event := entries[0].Event
	return &event, nil
}

func scanServiceEvents(rows pgx.Rows, limit int) ([]ServiceEventLogEntry, error) {
	entries := make([]ServiceEventLogEntry, 0, limit)
	for rows.Next() {
		var (
			entry          ServiceEventLogEntry
			slot           int64
			txIndex        int64
			outerIndex     int32
			innerIndex     *int32
			refsJSON       []byte
			payloadJSON    []byte
			source         string
			eventSource    string
			instructionRef events.InstructionPath
		)

		err := rows.Scan(
			&entry.Event.EventID,
			&entry.Event.SchemaVersion,
			&entry.Event.Chain,
			&entry.Event.Protocol,
			&entry.Event.EventType,
			&entry.Event.Commitment,
			&slot,
			&entry.Event.TxSignature,
			&txIndex,
			&source,
			&outerIndex,
			&innerIndex,
			&eventSource,
			&entry.Event.EventUnixTS,
			&refsJSON,
			&payloadJSON,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan service event row: %w", err)
		}

		entry.Event.Slot = uint64(slot)
		entry.Event.TxIndex = uint64(txIndex)
		instructionRef.Source = source
		instructionRef.OuterIndex = int(outerIndex)
		if innerIndex != nil {
			value := int(*innerIndex)
			instructionRef.InnerIndex = &value
		}
		entry.Event.InstructionPath = instructionRef
		entry.Event.EventSource = eventSource
		entry.Event.Payload = payloadJSON
		if err := json.Unmarshal(refsJSON, &entry.Event.Refs); err != nil {
			return nil, fmt.Errorf("unmarshal service event refs: %w", err)
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service event rows: %w", err)
	}

	return entries, nil
}
