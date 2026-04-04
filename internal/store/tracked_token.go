package store

import (
	"context"
	"fmt"

	"solana-dashboard-go/internal/db"
)

const listTrackedTokensSQL = `
select
    tt.mint,
    tt.creator,
    tt.bonding_curve,
    tt.token_program,
    tt.create_event_id,
    extract(epoch from tt.accepted_at)::bigint as accepted_at_unix,
    tt.current_stage,
    tt.current_market_type,
    tt.current_market_id,
    extract(epoch from tt.migrated_at)::bigint as migrated_at_unix,
    se.protocol,
    se.event_type,
    se.event_unix_ts,
    se.payload->>'name' as create_name,
    se.payload->>'symbol' as create_symbol,
    se.payload->>'uri' as create_uri,
    m.market_id,
    m.protocol,
    m.market_type,
    m.bonding_curve,
    m.pool,
    m.base_mint,
    m.quote_mint,
    m.lp_mint,
    extract(epoch from m.started_at)::bigint as market_started_at_unix,
    extract(epoch from m.ended_at)::bigint as market_ended_at_unix,
    m.create_event_id,
    lt.event_id,
    lt.market_id,
    lt.market_type,
    lt.protocol,
    lt.side,
    lt.ix_name,
    lt.user_address,
    lt.bonding_curve,
    lt.pool,
    lt.quote_mint,
    lt.token_amount::text,
    lt.quote_amount::text,
    lt.tx_signature,
    lt.slot,
    extract(epoch from lt.event_time)::bigint as latest_trade_event_unix_ts,
    lt.raw_event_source
from tracked_tokens tt
left join service_events se
    on se.event_id = tt.create_event_id
left join markets m
    on m.market_id = tt.current_market_id
left join lateral (
    select
        t.event_id,
        t.market_id,
        t.market_type,
        t.protocol,
        t.side,
        t.ix_name,
        t.user_address,
        t.bonding_curve,
        t.pool,
        t.quote_mint,
        t.token_amount,
        t.quote_amount,
        t.tx_signature,
        t.slot,
        t.event_time,
        t.raw_event_source
    from trades t
    where t.mint = tt.mint
    order by t.event_time desc, t.created_at desc
    limit 1
) lt on true
order by tt.accepted_at desc
limit $1
`

type TrackedTokenRecord struct {
	Mint              string        `json:"mint"`
	Creator           *string       `json:"creator,omitempty"`
	BondingCurve      *string       `json:"bonding_curve,omitempty"`
	TokenProgram      *string       `json:"token_program,omitempty"`
	CreateEventID     string        `json:"create_event_id"`
	AcceptedAt        int64         `json:"accepted_at"`
	CurrentStage      string        `json:"current_stage"`
	CurrentMarketType string        `json:"current_market_type"`
	CurrentMarketID   *string       `json:"current_market_id,omitempty"`
	MigratedAt        *int64        `json:"migrated_at,omitempty"`
	CreateProtocol    *string       `json:"-"`
	CreateEventType   *string       `json:"-"`
	CreateEventUnixTS *int64        `json:"-"`
	CreateName        *string       `json:"-"`
	CreateSymbol      *string       `json:"-"`
	CreateURI         *string       `json:"-"`
	CurrentMarket     *MarketRecord `json:"current_market,omitempty"`
	LatestTrade       *TradeRecord  `json:"latest_trade,omitempty"`
}

type TrackedTokenStore struct {
	db *db.DB
}

func NewTrackedTokenStore(database *db.DB) *TrackedTokenStore {
	return &TrackedTokenStore{db: database}
}

func (s *TrackedTokenStore) ListTrackedTokens(ctx context.Context, limit int) ([]TrackedTokenRecord, error) {
	rows, err := s.db.Pool.Query(ctx, listTrackedTokensSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("list tracked tokens: %w", err)
	}
	defer rows.Close()

	items := make([]TrackedTokenRecord, 0, limit)
	for rows.Next() {
		var (
			record              TrackedTokenRecord
			marketID            *string
			marketProtocol      *string
			marketType          *string
			marketBondingCurve  *string
			marketPool          *string
			marketBaseMint      *string
			marketQuoteMint     *string
			marketLPMint        *string
			marketStartedAt     *int64
			marketEndedAt       *int64
			marketCreateEventID *string
			tradeEventID        *string
			tradeMarketID       *string
			tradeMarketType     *string
			tradeProtocol       *string
			tradeSide           *string
			tradeIxName         *string
			tradeUserAddress    *string
			tradeBondingCurve   *string
			tradePool           *string
			tradeQuoteMint      *string
			tradeTokenAmount    *string
			tradeQuoteAmount    *string
			tradeTxSignature    *string
			tradeSlot           *int64
			tradeEventUnixTS    *int64
			tradeRawEventSource *string
		)

		if err := rows.Scan(
			&record.Mint,
			&record.Creator,
			&record.BondingCurve,
			&record.TokenProgram,
			&record.CreateEventID,
			&record.AcceptedAt,
			&record.CurrentStage,
			&record.CurrentMarketType,
			&record.CurrentMarketID,
			&record.MigratedAt,
			&record.CreateProtocol,
			&record.CreateEventType,
			&record.CreateEventUnixTS,
			&record.CreateName,
			&record.CreateSymbol,
			&record.CreateURI,
			&marketID,
			&marketProtocol,
			&marketType,
			&marketBondingCurve,
			&marketPool,
			&marketBaseMint,
			&marketQuoteMint,
			&marketLPMint,
			&marketStartedAt,
			&marketEndedAt,
			&marketCreateEventID,
			&tradeEventID,
			&tradeMarketID,
			&tradeMarketType,
			&tradeProtocol,
			&tradeSide,
			&tradeIxName,
			&tradeUserAddress,
			&tradeBondingCurve,
			&tradePool,
			&tradeQuoteMint,
			&tradeTokenAmount,
			&tradeQuoteAmount,
			&tradeTxSignature,
			&tradeSlot,
			&tradeEventUnixTS,
			&tradeRawEventSource,
		); err != nil {
			return nil, fmt.Errorf("scan tracked token row: %w", err)
		}

		if marketID != nil {
			record.CurrentMarket = &MarketRecord{
				MarketID:      *marketID,
				Mint:          record.Mint,
				Protocol:      stringValue(marketProtocol),
				MarketType:    stringValue(marketType),
				BondingCurve:  marketBondingCurve,
				Pool:          marketPool,
				BaseMint:      marketBaseMint,
				QuoteMint:     marketQuoteMint,
				LPMint:        marketLPMint,
				StartedAt:     int64Value(marketStartedAt),
				EndedAt:       marketEndedAt,
				CreateEventID: stringValue(marketCreateEventID),
			}
		}

		if tradeEventID != nil {
			record.LatestTrade = &TradeRecord{
				EventID:        *tradeEventID,
				Mint:           record.Mint,
				MarketID:       stringValue(tradeMarketID),
				MarketType:     stringValue(tradeMarketType),
				Protocol:       stringValue(tradeProtocol),
				Side:           stringValue(tradeSide),
				IxName:         stringValue(tradeIxName),
				UserAddress:    stringValue(tradeUserAddress),
				BondingCurve:   tradeBondingCurve,
				Pool:           tradePool,
				QuoteMint:      stringValue(tradeQuoteMint),
				TokenAmount:    stringValue(tradeTokenAmount),
				QuoteAmount:    stringValue(tradeQuoteAmount),
				TxSignature:    stringValue(tradeTxSignature),
				Slot:           uint64(int64Value(tradeSlot)),
				EventUnixTS:    int64Value(tradeEventUnixTS),
				RawEventSource: stringValue(tradeRawEventSource),
			}
		}

		items = append(items, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tracked token rows: %w", err)
	}

	return items, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
