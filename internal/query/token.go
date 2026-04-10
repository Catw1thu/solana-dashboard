package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"solana-dashboard-go/internal/broadcaster"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/store"
)

const (
	defaultDetailMarketLimit = 10
	defaultDetailTradeLimit  = 20
	defaultDetailEventLimit  = 20
	defaultActivityLimit     = 100
	defaultCandleLimit       = 200
	defaultTokenListLimit    = 100
	defaultTokenListView     = "hot"
	defaultTokenListWindow   = "24h"
	defaultTokenSearchLimit  = 8
)

var (
	ErrTokenNotFound           = errors.New("token not found")
	ErrInvalidCandleResolution = errors.New("invalid candle resolution")
	ErrInvalidTokenListWindow  = errors.New("invalid token list window")
)

type tokenEventReader interface {
	ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error)
	FindLatestCreateEventByMint(ctx context.Context, mint string) (*events.Envelope, error)
	FindLatestMigrateEventByMint(ctx context.Context, mint string) (*events.Envelope, error)
}

type tokenReadModel interface {
	ListTokenBoardRows(ctx context.Context, query store.TokenBoardQuery) ([]store.TokenBoardRecord, error)
	SearchTokenSnapshots(ctx context.Context, query string, limit int) ([]store.TokenSearchRecord, error)
	ListTokenSnapshots(ctx context.Context, limit int) ([]store.TokenSnapshotRecord, error)
	FindTokenSnapshotByMint(ctx context.Context, mint string) (*store.TokenSnapshotRecord, error)
	ListTokenMarketsByMint(ctx context.Context, mint string, limit int) ([]store.TokenMarketRecord, error)
	ListTradeEventsByMint(ctx context.Context, mint string, limit int) ([]store.TradeEventRecord, error)
	ListActivityEventsByMint(ctx context.Context, mint string, limit int) ([]store.ActivityEventRecord, error)
	ListActivityEventsPageByMint(ctx context.Context, mint string, limit int, cursor *store.ActivityEventCursor) (*store.ActivityEventPage, error)
	LoadTradeSummaryByMint(ctx context.Context, mint string) (*store.TradeSummaryRecord, error)
	ListTradeMetricsForStatsByMint(ctx context.Context, mint string) ([]store.TradeMetricPoint, error)
	ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int, beforeTime *int64) ([]store.TokenCandleRecord, error)
}

type TokenCreateSummary struct {
	EventID          string  `json:"event_id"`
	Protocol         string  `json:"protocol"`
	EventType        string  `json:"event_type"`
	EventUnixTS      int64   `json:"event_unix_ts"`
	Creator          *string `json:"creator,omitempty"`
	BondingCurve     *string `json:"bonding_curve,omitempty"`
	Name             string  `json:"name"`
	Symbol           string  `json:"symbol"`
	URI              string  `json:"uri"`
	TokenTotalSupply *string `json:"token_total_supply,omitempty"`
	TokenProgram     *string `json:"token_program,omitempty"`
	IsMayhemMode     *bool   `json:"is_mayhem_mode,omitempty"`
	Decimals         *int32  `json:"decimals,omitempty"`
}

type TokenQuote struct {
	Mint     string `json:"mint"`
	Decimals *int32 `json:"decimals,omitempty"`
	Symbol   string `json:"symbol"`
}

type TokenMarketMetrics struct {
	LatestPrice     *float64 `json:"latest_price,omitempty"`
	LatestEventUnix *int64   `json:"latest_event_unix_ts,omitempty"`
}

type TokenPriceChanges struct {
	M5  *float64 `json:"m5,omitempty"`
	H1  *float64 `json:"h1,omitempty"`
	H4  *float64 `json:"h4,omitempty"`
	H6  *float64 `json:"h6,omitempty"`
	H24 *float64 `json:"h24,omitempty"`
}

type TokenTradeStats struct {
	Txns       int64   `json:"txns"`
	Buys       int64   `json:"buys"`
	Sells      int64   `json:"sells"`
	Volume     float64 `json:"volume"`
	BuyVolume  float64 `json:"buy_volume"`
	SellVolume float64 `json:"sell_volume"`
	Makers     int64   `json:"makers"`
	Buyers     int64   `json:"buyers"`
	Sellers    int64   `json:"sellers"`
}

type TokenTrade struct {
	EventID        string   `json:"event_id"`
	Mint           string   `json:"mint"`
	MarketID       string   `json:"market_id"`
	MarketType     string   `json:"market_type"`
	Protocol       string   `json:"protocol"`
	Side           string   `json:"side"`
	IxName         string   `json:"ix_name"`
	UserAddress    string   `json:"user_address"`
	QuoteMint      string   `json:"quote_mint"`
	TokenAmountRaw string   `json:"token_amount_raw"`
	QuoteAmountRaw string   `json:"quote_amount_raw"`
	TokenAmount    string   `json:"token_amount"`
	QuoteAmount    string   `json:"quote_amount"`
	Price          *float64 `json:"price,omitempty"`
	TxSignature    string   `json:"tx_signature"`
	Slot           uint64   `json:"slot"`
	EventUnixTS    int64    `json:"event_unix_ts"`
	RawEventSource string   `json:"raw_event_source"`
}

type TokenActivity struct {
	EventID      string          `json:"event_id"`
	Mint         string          `json:"mint"`
	Protocol     string          `json:"protocol"`
	EventType    string          `json:"event_type"`
	ActivityType string          `json:"activity_type"`
	MarketID     *string         `json:"market_id,omitempty"`
	MarketType   *string         `json:"market_type,omitempty"`
	UserAddress  *string         `json:"user_address,omitempty"`
	Side         *string         `json:"side,omitempty"`
	Price        *float64        `json:"price,omitempty"`
	Quantity     *string         `json:"quantity,omitempty"`
	TotalQuote   *string         `json:"total_quote,omitempty"`
	QuoteMint    *string         `json:"quote_mint,omitempty"`
	TxSignature  string          `json:"tx_signature"`
	Slot         uint64          `json:"slot"`
	EventUnixTS  int64           `json:"event_unix_ts"`
	Details      json.RawMessage `json:"details"`
}

type TokenActivityCursor struct {
	EventUnixTS int64  `json:"event_unix_ts"`
	Slot        uint64 `json:"slot"`
	InsertSeq   int64  `json:"insert_seq"`
}

type TokenActivityPage struct {
	Activity   []TokenActivity      `json:"activity"`
	HasMore    bool                 `json:"has_more"`
	NextCursor *TokenActivityCursor `json:"next_cursor,omitempty"`
}

type TokenListItem struct {
	Mint              string           `json:"mint"`
	Name              *string          `json:"name,omitempty"`
	Symbol            *string          `json:"symbol,omitempty"`
	URI               *string          `json:"uri,omitempty"`
	ImageURI          *string          `json:"image_uri,omitempty"`
	Creator           *string          `json:"creator,omitempty"`
	BondingCurve      *string          `json:"bonding_curve,omitempty"`
	TokenProgram      *string          `json:"token_program,omitempty"`
	Decimals          *int32           `json:"decimals,omitempty"`
	QuoteMint         *string          `json:"quote_mint,omitempty"`
	QuoteDecimals     *int32           `json:"quote_decimals,omitempty"`
	AcceptedAt        int64            `json:"accepted_at"`
	ActiveSince       int64            `json:"active_since"`
	CurrentStage      string           `json:"current_stage"`
	CurrentMarketType *string          `json:"current_market_type,omitempty"`
	CurrentMarketID   *string          `json:"current_market_id,omitempty"`
	MigratedAt        *int64           `json:"migrated_at,omitempty"`
	LatestPrice       *float64         `json:"latest_price,omitempty"`
	LatestEventUnixTS *int64           `json:"latest_event_unix_ts,omitempty"`
	PriceChange       *float64         `json:"price_change,omitempty"`
	WindowVolume      float64          `json:"window_volume"`
	WindowTxns        int64            `json:"window_txns"`
	WindowBuys        int64            `json:"window_buys"`
	WindowSells       int64            `json:"window_sells"`
	LiquidityQuote    *float64         `json:"liquidity_quote,omitempty"`
	MarketCapQuote    *float64         `json:"market_cap_quote,omitempty"`
	Stats24h          *TokenTradeStats `json:"stats_24h,omitempty"`
}

type TokenListOptions struct {
	Limit  int
	View   string
	Window string
}

type TokenSearchItem struct {
	Mint        string   `json:"mint"`
	Name        *string  `json:"name,omitempty"`
	Symbol      *string  `json:"symbol,omitempty"`
	ImageURI    *string  `json:"image_uri,omitempty"`
	LatestPrice *float64 `json:"latest_price,omitempty"`
}

type TokenDetail struct {
	Mint           string                    `json:"mint"`
	Name           *string                   `json:"name,omitempty"`
	Symbol         *string                   `json:"symbol,omitempty"`
	URI            *string                   `json:"uri,omitempty"`
	ImageURI       *string                   `json:"image_uri,omitempty"`
	Creator        *string                   `json:"creator,omitempty"`
	BondingCurve   *string                   `json:"bonding_curve,omitempty"`
	TokenProgram   *string                   `json:"token_program,omitempty"`
	Decimals       *int32                    `json:"decimals,omitempty"`
	TotalSupplyRaw *string                   `json:"total_supply_raw,omitempty"`
	CurrentStage   string                    `json:"current_stage"`
	CreateEvent    *TokenCreateSummary       `json:"create_event,omitempty"`
	ActiveMarket   *store.TokenMarketRecord  `json:"active_market,omitempty"`
	Markets        []store.TokenMarketRecord `json:"markets"`
	RecentTrades   []TokenTrade              `json:"recent_trades"`
	RecentEvents   []events.Envelope         `json:"recent_events"`
	MigrateEvent   *events.Envelope          `json:"migrate_event,omitempty"`
	Quote          *TokenQuote               `json:"quote,omitempty"`
	MarketMetrics  *TokenMarketMetrics       `json:"market_metrics,omitempty"`
	PriceChanges   *TokenPriceChanges        `json:"price_changes,omitempty"`
	Stats24h       *TokenTradeStats          `json:"stats_24h,omitempty"`
}

type TokenCandle struct {
	Time      int64   `json:"time"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	IsGapFill bool    `json:"is_gapfill"`
}

type TokenService struct {
	events tokenEventReader
	model  tokenReadModel
}

func NewTokenService(events tokenEventReader, model tokenReadModel) *TokenService {
	return &TokenService{
		events: events,
		model:  model,
	}
}

func (s *TokenService) ListTokens(ctx context.Context, opts TokenListOptions) ([]TokenListItem, error) {
	if s.model == nil {
		return nil, fmt.Errorf("token read model not configured")
	}
	if opts.Limit <= 0 {
		opts.Limit = defaultTokenListLimit
	}

	view := normalizeTokenListView(opts.View)
	interval, err := normalizeStatsWindow(opts.Window)
	if err != nil {
		return nil, err
	}

	rows, err := s.model.ListTokenBoardRows(ctx, store.TokenBoardQuery{
		Limit:    opts.Limit,
		View:     view,
		Interval: interval,
	})
	if err != nil {
		return nil, fmt.Errorf("list token board rows: %w", err)
	}

	items := make([]TokenListItem, 0, len(rows))
	for _, row := range rows {
		item := TokenListItem{
			Mint:              row.Mint,
			Name:              row.Name,
			Symbol:            row.Symbol,
			URI:               row.URI,
			ImageURI:          row.ImageURI,
			Creator:           row.Creator,
			BondingCurve:      row.BondingCurve,
			TokenProgram:      row.TokenProgram,
			Decimals:          row.Decimals,
			QuoteMint:         row.QuoteMint,
			QuoteDecimals:     row.QuoteDecimals,
			AcceptedAt:        row.FirstSeenAt,
			ActiveSince:       row.ActiveSince,
			CurrentStage:      row.CurrentStage,
			CurrentMarketType: row.ActiveMarketType,
			CurrentMarketID:   row.ActiveMarketID,
			MigratedAt:        row.MigratedAt,
			LatestPrice:       row.LatestPrice,
			LatestEventUnixTS: row.LatestEventUnixTS,
			PriceChange:       row.PriceChange,
			WindowVolume:      row.WindowVolume,
			WindowTxns:        row.WindowTxns,
			WindowBuys:        row.WindowBuys,
			WindowSells:       row.WindowSells,
			LiquidityQuote:    row.LiquidityQuote,
			MarketCapQuote:    row.MarketCapQuote,
		}

		items = append(items, item)
	}

	return items, nil
}

func (s *TokenService) SearchTokens(ctx context.Context, rawQuery string, limit int) ([]TokenSearchItem, error) {
	if s.model == nil {
		return nil, fmt.Errorf("token read model not configured")
	}

	query := strings.TrimSpace(rawQuery)
	if query == "" {
		return []TokenSearchItem{}, nil
	}
	if limit <= 0 {
		limit = defaultTokenSearchLimit
	}

	rows, err := s.model.SearchTokenSnapshots(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search token snapshots: %w", err)
	}

	items := make([]TokenSearchItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, TokenSearchItem{
			Mint:        row.Mint,
			Name:        row.Name,
			Symbol:      row.Symbol,
			ImageURI:    row.ImageURI,
			LatestPrice: row.LatestPrice,
		})
	}

	return items, nil
}

func (s *TokenService) ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error) {
	if s.events == nil {
		return nil, fmt.Errorf("event reader not configured")
	}
	return s.events.ListServiceEventsByMint(ctx, mint, limit)
}

func (s *TokenService) ListTradesByMint(ctx context.Context, mint string, limit int) ([]TokenTrade, error) {
	snapshot, err := s.requireTokenSnapshot(ctx, mint)
	if err != nil {
		return nil, err
	}

	rows, err := s.model.ListTradeEventsByMint(ctx, mint, clampLimit(limit, defaultDetailTradeLimit))
	if err != nil {
		return nil, fmt.Errorf("list trade events: %w", err)
	}

	items := make([]TokenTrade, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildTokenTrade(snapshot, row))
	}

	return items, nil
}

func (s *TokenService) ListActivityByMint(ctx context.Context, mint string, limit int) ([]TokenActivity, error) {
	page, err := s.ListActivityPageByMint(ctx, mint, limit, nil)
	if err != nil {
		return nil, err
	}
	return page.Activity, nil
}

func (s *TokenService) ListActivityPageByMint(ctx context.Context, mint string, limit int, cursor *TokenActivityCursor) (*TokenActivityPage, error) {
	snapshot, err := s.requireTokenSnapshot(ctx, mint)
	if err != nil {
		return nil, err
	}

	var storeCursor *store.ActivityEventCursor
	if cursor != nil {
		storeCursor = &store.ActivityEventCursor{
			EventUnixTS: cursor.EventUnixTS,
			Slot:        cursor.Slot,
			InsertSeq:   cursor.InsertSeq,
		}
	}

	page, err := s.model.ListActivityEventsPageByMint(ctx, mint, clampLimit(limit, defaultActivityLimit), storeCursor)
	if err != nil {
		return nil, fmt.Errorf("list activity events: %w", err)
	}

	items := make([]TokenActivity, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, buildTokenActivity(snapshot, row))
	}

	var nextCursor *TokenActivityCursor
	if page.NextCursor != nil {
		nextCursor = &TokenActivityCursor{
			EventUnixTS: page.NextCursor.EventUnixTS,
			Slot:        page.NextCursor.Slot,
			InsertSeq:   page.NextCursor.InsertSeq,
		}
	}

	return &TokenActivityPage{
		Activity:   items,
		HasMore:    page.HasMore,
		NextCursor: nextCursor,
	}, nil
}

func (s *TokenService) ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int, beforeTime *int64) ([]TokenCandle, error) {
	if s.model == nil {
		return nil, fmt.Errorf("token read model not configured")
	}
	if limit <= 0 {
		limit = defaultCandleLimit
	}

	interval, err := normalizeResolution(resolution)
	if err != nil {
		return nil, err
	}

	rows, err := s.model.ListCandlesByMint(ctx, mint, interval, limit, beforeTime)
	if err != nil {
		return nil, fmt.Errorf("list candles: %w", err)
	}

	candles := make([]TokenCandle, 0, len(rows))
	for _, row := range rows {
		candles = append(candles, TokenCandle{
			Time:      row.TimeUnix,
			Open:      row.Open,
			High:      row.High,
			Low:       row.Low,
			Close:     row.Close,
			Volume:    row.Volume,
			IsGapFill: row.IsGapFill,
		})
	}

	return candles, nil
}

func (s *TokenService) BuildRealtimeStatsPayload(ctx context.Context, mint string, nowTs int64) (*broadcaster.TokenStatsPayload, error) {
	if s.model == nil {
		return nil, fmt.Errorf("token read model not configured")
	}

	snapshot, err := s.model.FindTokenSnapshotByMint(ctx, mint)
	if err != nil {
		return nil, fmt.Errorf("find token snapshot by mint=%s: %w", mint, err)
	}
	if snapshot == nil {
		return nil, ErrTokenNotFound
	}

	metrics, err := s.model.ListTradeMetricsForStatsByMint(ctx, mint)
	if err != nil {
		return nil, fmt.Errorf("list trade metrics for mint=%s: %w", mint, err)
	}

	payload, ok := broadcaster.BuildPayloadFromTradeMetrics(mint, metrics, nowTs)
	if !ok {
		return nil, nil
	}

	return &payload, nil
}

func (s *TokenService) GetTokenDetail(ctx context.Context, mint string) (TokenDetail, error) {
	snapshot, err := s.requireTokenSnapshot(ctx, mint)
	if err != nil {
		return TokenDetail{}, err
	}

	markets, err := s.model.ListTokenMarketsByMint(ctx, mint, defaultDetailMarketLimit)
	if err != nil {
		return TokenDetail{}, fmt.Errorf("list token markets: %w", err)
	}

	tradeRows, err := s.model.ListTradeEventsByMint(ctx, mint, defaultDetailTradeLimit)
	if err != nil {
		return TokenDetail{}, fmt.Errorf("list token trades: %w", err)
	}

	eventRows, err := s.events.ListServiceEventsByMint(ctx, mint, defaultDetailEventLimit)
	if err != nil {
		return TokenDetail{}, fmt.Errorf("list service events: %w", err)
	}

	summary, err := s.model.LoadTradeSummaryByMint(ctx, mint)
	if err != nil {
		return TokenDetail{}, fmt.Errorf("load trade summary: %w", err)
	}

	createSummary, err := s.loadCreateSummary(ctx, mint)
	if err != nil {
		return TokenDetail{}, err
	}

	trades := make([]TokenTrade, 0, len(tradeRows))
	for _, row := range tradeRows {
		trades = append(trades, buildTokenTrade(snapshot, row))
	}

	detail := TokenDetail{
		Mint:           snapshot.Mint,
		Name:           snapshot.Name,
		Symbol:         snapshot.Symbol,
		URI:            snapshot.URI,
		ImageURI:       snapshot.ImageURI,
		Creator:        snapshot.Creator,
		BondingCurve:   snapshot.BondingCurve,
		TokenProgram:   snapshot.TokenProgram,
		Decimals:       snapshot.Decimals,
		TotalSupplyRaw: snapshot.TotalSupplyRaw,
		CurrentStage:   snapshot.CurrentStage,
		CreateEvent:    createSummary,
		ActiveMarket:   findActiveMarket(markets, snapshot.ActiveMarketID),
		Markets:        markets,
		RecentTrades:   trades,
		RecentEvents:   eventRows,
	}
	if snapshot.MigratedAt != nil || snapshot.CurrentStage == "pool" {
		migrateEvent, err := s.events.FindLatestMigrateEventByMint(ctx, mint)
		if err != nil {
			return TokenDetail{}, fmt.Errorf("find latest migrate event by mint=%s: %w", mint, err)
		}
		detail.MigrateEvent = migrateEvent
	}

	if quote := buildQuote(snapshot, summary); quote != nil {
		detail.Quote = quote
	}
	if summary != nil {
		detail.MarketMetrics = &TokenMarketMetrics{
			LatestPrice:     summary.LatestPrice,
			LatestEventUnix: summary.LatestEventUnix,
		}
		detail.PriceChanges = buildPriceChanges(summary)
		detail.Stats24h = buildTradeStats(summary)
	}

	return detail, nil
}

func (s *TokenService) requireTokenSnapshot(ctx context.Context, mint string) (*store.TokenSnapshotRecord, error) {
	if s.model == nil {
		return nil, fmt.Errorf("token read model not configured")
	}

	snapshot, err := s.model.FindTokenSnapshotByMint(ctx, mint)
	if err != nil {
		return nil, fmt.Errorf("find token snapshot by mint=%s: %w", mint, err)
	}
	if snapshot == nil {
		return nil, ErrTokenNotFound
	}

	return snapshot, nil
}

func (s *TokenService) loadCreateSummary(ctx context.Context, mint string) (*TokenCreateSummary, error) {
	if s.events == nil {
		return nil, nil
	}

	createEvent, err := s.events.FindLatestCreateEventByMint(ctx, mint)
	if err != nil {
		return nil, fmt.Errorf("find latest create event by mint=%s: %w", mint, err)
	}
	if createEvent == nil {
		return nil, nil
	}

	payload, err := events.DecodePayload(*createEvent)
	if err != nil {
		return nil, fmt.Errorf("decode create event payload for mint=%s: %w", mint, err)
	}

	switch value := payload.(type) {
	case events.PumpfunCreatePayload:
		return &TokenCreateSummary{
			EventID:          createEvent.EventID,
			Protocol:         createEvent.Protocol,
			EventType:        createEvent.EventType,
			EventUnixTS:      createEvent.EventUnixTS,
			Creator:          ptrIfNotEmpty(value.Creator),
			BondingCurve:     ptrIfNotEmpty(value.BondingCurve),
			Name:             value.Name,
			Symbol:           value.Symbol,
			URI:              value.URI,
			TokenTotalSupply: ptrIfNotEmpty(value.TokenTotalSupply),
			TokenProgram:     ptrIfNotEmpty(value.TokenProgram),
			IsMayhemMode:     boolPtr(value.IsMayhemMode),
			Decimals:         optionalInt32FromUint32(value.MintDecimals),
		}, nil
	default:
		return nil, nil
	}
}

func buildQuote(snapshot *store.TokenSnapshotRecord, summary *store.TradeSummaryRecord) *TokenQuote {
	var quoteMint *string
	if summary != nil && summary.QuoteMint != "" {
		quoteMint = &summary.QuoteMint
	} else {
		quoteMint = snapshot.QuoteMint
	}
	if quoteMint == nil || *quoteMint == "" {
		return nil
	}

	return &TokenQuote{
		Mint:     *quoteMint,
		Decimals: resolveQuoteDecimals(snapshot, *quoteMint),
		Symbol:   quoteSymbol(*quoteMint),
	}
}

func buildPriceChanges(summary *store.TradeSummaryRecord) *TokenPriceChanges {
	if summary == nil || summary.LatestPrice == nil {
		return nil
	}

	return &TokenPriceChanges{
		M5:  pctChange(summary.LatestPrice, summary.Price5mAgo),
		H1:  pctChange(summary.LatestPrice, summary.Price1hAgo),
		H4:  pctChange(summary.LatestPrice, summary.Price4hAgo),
		H24: pctChange(summary.LatestPrice, summary.Price24hAgo),
	}
}

func buildTradeStats(summary *store.TradeSummaryRecord) *TokenTradeStats {
	if summary == nil {
		return nil
	}

	return &TokenTradeStats{
		Txns:       summary.Txns24h,
		Buys:       summary.Buys24h,
		Sells:      summary.Sells24h,
		Volume:     summary.Volume24h,
		BuyVolume:  summary.BuyVolume24h,
		SellVolume: summary.SellVolume24h,
		Makers:     summary.Makers24h,
		Buyers:     summary.Buyers24h,
		Sellers:    summary.Sellers24h,
	}
}

func buildTokenTrade(snapshot *store.TokenSnapshotRecord, row store.TradeEventRecord) TokenTrade {
	quoteDecimals := resolveQuoteDecimals(snapshot, row.QuoteMint)
	tokenAmount := formatAmount(row.TokenAmountRaw, snapshot.Decimals)
	quoteAmount := formatAmount(row.QuoteAmountRaw, quoteDecimals)

	return TokenTrade{
		EventID:        row.EventID,
		Mint:           row.Mint,
		MarketID:       row.MarketID,
		MarketType:     row.MarketType,
		Protocol:       row.Protocol,
		Side:           row.Side,
		IxName:         row.IxName,
		UserAddress:    row.UserAddress,
		QuoteMint:      row.QuoteMint,
		TokenAmountRaw: row.TokenAmountRaw,
		QuoteAmountRaw: row.QuoteAmountRaw,
		TokenAmount:    tokenAmount,
		QuoteAmount:    quoteAmount,
		Price:          computePrice(row.TokenAmountRaw, row.QuoteAmountRaw, snapshot.Decimals, quoteDecimals),
		TxSignature:    row.TxSignature,
		Slot:           row.Slot,
		EventUnixTS:    row.EventUnixTS,
		RawEventSource: row.RawEventSource,
	}
}

func buildTokenActivity(snapshot *store.TokenSnapshotRecord, row store.ActivityEventRecord) TokenActivity {
	var quantity *string
	if row.TokenAmountRaw != nil {
		value := formatAmount(*row.TokenAmountRaw, snapshot.Decimals)
		quantity = &value
	}

	var totalQuote *string
	quoteDecimals := resolveOptionalQuoteDecimals(snapshot, row.QuoteMint)
	if row.QuoteAmountRaw != nil {
		value := formatAmount(*row.QuoteAmountRaw, quoteDecimals)
		totalQuote = &value
	}

	return TokenActivity{
		EventID:      row.EventID,
		Mint:         row.Mint,
		Protocol:     row.Protocol,
		EventType:    row.EventType,
		ActivityType: row.ActivityType,
		MarketID:     row.MarketID,
		MarketType:   row.MarketType,
		UserAddress:  row.UserAddress,
		Side:         row.Side,
		Price:        computeOptionalPrice(row.TokenAmountRaw, row.QuoteAmountRaw, snapshot.Decimals, quoteDecimals),
		Quantity:     quantity,
		TotalQuote:   totalQuote,
		QuoteMint:    row.QuoteMint,
		TxSignature:  row.TxSignature,
		Slot:         row.Slot,
		EventUnixTS:  row.EventUnixTS,
		Details:      row.Details,
	}
}

func findActiveMarket(markets []store.TokenMarketRecord, activeMarketID *string) *store.TokenMarketRecord {
	if activeMarketID == nil {
		for _, market := range markets {
			if market.EndedAt == nil {
				copy := market
				return &copy
			}
		}
		return nil
	}

	for _, market := range markets {
		if market.MarketID == *activeMarketID {
			copy := market
			return &copy
		}
	}

	return nil
}

func resolveQuoteDecimals(snapshot *store.TokenSnapshotRecord, quoteMint string) *int32 {
	if quoteMint == store.SolMint {
		return int32Ptr(store.SolDecimals)
	}
	if snapshot == nil {
		return nil
	}
	if snapshot.QuoteMint != nil && *snapshot.QuoteMint == quoteMint {
		return snapshot.QuoteDecimals
	}
	return snapshot.QuoteDecimals
}

func resolveOptionalQuoteDecimals(snapshot *store.TokenSnapshotRecord, quoteMint *string) *int32 {
	if quoteMint == nil {
		return nil
	}
	return resolveQuoteDecimals(snapshot, *quoteMint)
}

func computeOptionalPrice(tokenAmountRaw *string, quoteAmountRaw *string, tokenDecimals *int32, quoteDecimals *int32) *float64 {
	if tokenAmountRaw == nil || quoteAmountRaw == nil {
		return nil
	}
	return computePrice(*tokenAmountRaw, *quoteAmountRaw, tokenDecimals, quoteDecimals)
}

func computePrice(tokenAmountRaw string, quoteAmountRaw string, tokenDecimals *int32, quoteDecimals *int32) *float64 {
	token := scaledRat(tokenAmountRaw, tokenDecimals)
	quote := scaledRat(quoteAmountRaw, quoteDecimals)
	if token == nil || quote == nil || token.Sign() == 0 {
		return nil
	}

	value, _ := new(big.Rat).Quo(quote, token).Float64()
	return &value
}

func scaledRat(raw string, decimals *int32) *big.Rat {
	if raw == "" {
		return nil
	}
	numerator := new(big.Int)
	if _, ok := numerator.SetString(raw, 10); !ok {
		return nil
	}

	ratio := new(big.Rat).SetInt(numerator)
	if decimals == nil || *decimals <= 0 {
		return ratio
	}

	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(*decimals)), nil)
	return ratio.Quo(ratio, new(big.Rat).SetInt(denominator))
}

func formatAmount(raw string, decimals *int32) string {
	if raw == "" {
		return ""
	}
	if decimals == nil || *decimals <= 0 {
		return raw
	}

	value := new(big.Int)
	if _, ok := value.SetString(raw, 10); !ok {
		return raw
	}

	negative := value.Sign() < 0
	if negative {
		value.Neg(value)
	}

	digits := value.String()
	scale := int(*decimals)
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}

	point := len(digits) - scale
	formatted := digits[:point] + "." + digits[point:]
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	if formatted == "" {
		formatted = "0"
	}
	if negative && formatted != "0" {
		formatted = "-" + formatted
	}

	return formatted
}

func pctChange(latest *float64, previous *float64) *float64 {
	if latest == nil || previous == nil || *previous == 0 {
		return nil
	}

	value := ((*latest - *previous) / *previous) * 100
	return &value
}

func normalizeResolution(resolution string) (string, error) {
	switch resolution {
	case "1m":
		return "1 minute", nil
	case "5m":
		return "5 minutes", nil
	case "15m":
		return "15 minutes", nil
	case "1h":
		return "1 hour", nil
	case "4h":
		return "4 hours", nil
	case "1d":
		return "1 day", nil
	default:
		return "", ErrInvalidCandleResolution
	}
}

func normalizeTokenListView(view string) string {
	switch strings.ToLower(strings.TrimSpace(view)) {
	case "new":
		return "new"
	case "hot", "":
		return defaultTokenListView
	default:
		return defaultTokenListView
	}
}

func normalizeStatsWindow(window string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "", "24h", "1d":
		return "24 hours", nil
	case "1m":
		return "1 minute", nil
	case "5m":
		return "5 minutes", nil
	case "1h":
		return "1 hour", nil
	case "4h":
		return "4 hours", nil
	default:
		return "", ErrInvalidTokenListWindow
	}
}

func quoteSymbol(mint string) string {
	if mint == store.SolMint {
		return "SOL"
	}
	return mint
}

func clampLimit(limit int, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	return limit
}

func ptrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func optionalInt32FromUint32(value *uint32) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}
