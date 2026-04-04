package store

import (
	"context"
	"encoding/json"
	"fmt"

	"solana-dashboard-go/internal/db"
)

const insertTokenTimelineEventSQL = `
insert into token_timeline_events (
    event_id,
    mint,
    protocol,
    event_type,
    timeline_type,
    market_id,
    market_type,
    user_address,
    side,
    quote_mint,
    token_amount,
    quote_amount,
    tx_signature,
    slot,
    event_time,
    raw_event_source,
    details
) values (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::numeric, $12::numeric, $13, $14, to_timestamp($15), $16, $17
)
on conflict (event_id) do nothing
`

const listTimelineByMintSQL = `
select
    event_id,
    mint,
    protocol,
    event_type,
    timeline_type,
    market_id,
    market_type,
    user_address,
    side,
    quote_mint,
    token_amount::text,
    quote_amount::text,
    tx_signature,
    slot,
    extract(epoch from event_time)::bigint as event_unix_ts,
    raw_event_source,
    details
from token_timeline_events
where mint = $1
order by event_time desc, created_at desc
limit $2
`

type TokenTimelineRecord struct {
	EventID        string          `json:"event_id"`
	Mint           string          `json:"mint"`
	Protocol       string          `json:"protocol"`
	EventType      string          `json:"event_type"`
	TimelineType   string          `json:"timeline_type"`
	MarketID       *string         `json:"market_id,omitempty"`
	MarketType     *string         `json:"market_type,omitempty"`
	UserAddress    *string         `json:"user_address,omitempty"`
	Side           *string         `json:"side,omitempty"`
	QuoteMint      *string         `json:"quote_mint,omitempty"`
	TokenAmount    *string         `json:"token_amount,omitempty"`
	QuoteAmount    *string         `json:"quote_amount,omitempty"`
	TxSignature    string          `json:"tx_signature"`
	Slot           uint64          `json:"slot"`
	EventUnixTS    int64           `json:"event_unix_ts"`
	RawEventSource string          `json:"raw_event_source"`
	Details        json.RawMessage `json:"details"`
}

type TokenTimelineStore struct {
	db *db.DB
}

func NewTokenTimelineStore(database *db.DB) *TokenTimelineStore {
	return &TokenTimelineStore{db: database}
}

func (s *TokenTimelineStore) InsertTimelineEvent(ctx context.Context, item TokenTimelineRecord) error {
	details := item.Details
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}

	_, err := s.db.Pool.Exec(
		ctx,
		insertTokenTimelineEventSQL,
		item.EventID,
		item.Mint,
		item.Protocol,
		item.EventType,
		item.TimelineType,
		item.MarketID,
		item.MarketType,
		item.UserAddress,
		item.Side,
		item.QuoteMint,
		item.TokenAmount,
		item.QuoteAmount,
		item.TxSignature,
		int64(item.Slot),
		float64(item.EventUnixTS),
		item.RawEventSource,
		[]byte(details),
	)
	if err != nil {
		return fmt.Errorf("insert timeline event: %w", err)
	}

	return nil
}

func (s *TokenTimelineStore) ListTimelineByMint(ctx context.Context, mint string, limit int) ([]TokenTimelineRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTimelineByMintSQL, mint, limit)
	if err != nil {
		return nil, fmt.Errorf("list timeline by mint=%s: %w", mint, err)
	}
	defer rows.Close()

	items := make([]TokenTimelineRecord, 0, limit)
	for rows.Next() {
		var (
			item TokenTimelineRecord
			slot int64
		)
		if err := rows.Scan(
			&item.EventID,
			&item.Mint,
			&item.Protocol,
			&item.EventType,
			&item.TimelineType,
			&item.MarketID,
			&item.MarketType,
			&item.UserAddress,
			&item.Side,
			&item.QuoteMint,
			&item.TokenAmount,
			&item.QuoteAmount,
			&item.TxSignature,
			&slot,
			&item.EventUnixTS,
			&item.RawEventSource,
			&item.Details,
		); err != nil {
			return nil, fmt.Errorf("scan timeline row: %w", err)
		}
		item.Slot = uint64(slot)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeline rows: %w", err)
	}

	return items, nil
}
