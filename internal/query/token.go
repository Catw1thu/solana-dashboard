package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

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
)

var (
	ErrTokenNotFound           = errors.New("token not found")
	ErrInvalidCandleResolution = errors.New("invalid candle resolution")
)

type tokenEventReader interface {
	ListServiceEventsByMint(ctx context.Context, mint string, limit int) ([]events.Envelope, error)
	FindLatestCreateEventByMint(ctx context.Context, mint string) (*events.Envelope, error)
}

type tokenReadModel interface {
	ListTokenSnapshots(ctx context.Context, limit int) ([]store.TokenSnapshotRecord, error)
	FindTokenSnapshotByMint(ctx context.Context, mint string) (*store.TokenSnapshotRecord, error)
	ListTokenMarketsByMint(ctx context.Context, mint string, limit int) ([]store.TokenMarketRecord, error)
	ListTradeEventsByMint(ctx context.Context, mint string, limit int) ([]store.TradeEventRecord, error)
	ListActivityEventsByMint(ctx context.Context, mint string, limit int) ([]store.ActivityEventRecord, error)
	LoadTradeSummaryByMint(ctx context.Context, mint string) (*store.TradeSummaryRecord, error)
	ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int) ([]store.TokenCandleRecord, error)
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

type TokenListItem struct {
	Mint              string           `json:"mint"`
	Name              *string          `json:"name,omitempty"`
	Symbol            *string          `json:"symbol,omitempty"`
	URI               *string          `json:"uri,omitempty"`
	Creator           *string          `json:"creator,omitempty"`
	BondingCurve      *string          `json:"bonding_curve,omitempty"`
	TokenProgram      *string          `json:"token_program,omitempty"`
	Decimals          *int32           `json:"decimals,omitempty"`
	QuoteMint         *string          `json:"quote_mint,omitempty"`
	QuoteDecimals     *int32           `json:"quote_decimals,omitempty"`
	AcceptedAt        int64            `json:"accepted_at"`
	CurrentStage      string           `json:"current_stage"`
	CurrentMarketType *string          `json:"current_market_type,omitempty"`
	CurrentMarketID   *string          `json:"current_market_id,omitempty"`
	MigratedAt        *int64           `json:"migrated_at,omitempty"`
	LatestPrice       *float64         `json:"latest_price,omitempty"`
	LatestEventUnixTS *int64           `json:"latest_event_unix_ts,omitempty"`
	Stats24h          *TokenTradeStats `json:"stats_24h,omitempty"`
}

type TokenDetail struct {
	Mint           string                    `json:"mint"`
	Name           *string                   `json:"name,omitempty"`
	Symbol         *string                   `json:"symbol,omitempty"`
	URI            *string                   `json:"uri,omitempty"`
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
	Quote          *TokenQuote               `json:"quote,omitempty"`
	MarketMetrics  *TokenMarketMetrics       `json:"market_metrics,omitempty"`
	PriceChanges   *TokenPriceChanges        `json:"price_changes,omitempty"`
	Stats24h       *TokenTradeStats          `json:"stats_24h,omitempty"`
}

type TokenCandle struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
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

func (s *TokenService) ListTokens(ctx context.Context, limit int) ([]TokenListItem, error) {
	if s.model == nil {
		return nil, fmt.Errorf("token read model not configured")
	}
	if limit <= 0 {
		limit = defaultTokenListLimit
	}

	snapshots, err := s.model.ListTokenSnapshots(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list token snapshots: %w", err)
	}

	items := make([]TokenListItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		summary, err := s.model.LoadTradeSummaryByMint(ctx, snapshot.Mint)
		if err != nil {
			return nil, fmt.Errorf("load trade summary for mint=%s: %w", snapshot.Mint, err)
		}

		item := TokenListItem{
			Mint:              snapshot.Mint,
			Name:              snapshot.Name,
			Symbol:            snapshot.Symbol,
			URI:               snapshot.URI,
			Creator:           snapshot.Creator,
			BondingCurve:      snapshot.BondingCurve,
			TokenProgram:      snapshot.TokenProgram,
			Decimals:          snapshot.Decimals,
			QuoteMint:         snapshot.QuoteMint,
			QuoteDecimals:     snapshot.QuoteDecimals,
			AcceptedAt:        snapshot.FirstSeenAt,
			CurrentStage:      snapshot.CurrentStage,
			CurrentMarketType: snapshot.ActiveMarketType,
			CurrentMarketID:   snapshot.ActiveMarketID,
			MigratedAt:        snapshot.MigratedAt,
		}
		if summary != nil {
			item.LatestPrice = summary.LatestPrice
			item.LatestEventUnixTS = summary.LatestEventUnix
			item.Stats24h = buildTradeStats(summary)
		}

		items = append(items, item)
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

func (s *TokenService) ListTimelineByMint(ctx context.Context, mint string, limit int) ([]TokenActivity, error) {
	return s.ListActivityByMint(ctx, mint, limit)
}

func (s *TokenService) ListActivityByMint(ctx context.Context, mint string, limit int) ([]TokenActivity, error) {
	snapshot, err := s.requireTokenSnapshot(ctx, mint)
	if err != nil {
		return nil, err
	}

	rows, err := s.model.ListActivityEventsByMint(ctx, mint, clampLimit(limit, defaultActivityLimit))
	if err != nil {
		return nil, fmt.Errorf("list activity events: %w", err)
	}

	items := make([]TokenActivity, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildTokenActivity(snapshot, row))
	}

	return items, nil
}

func (s *TokenService) ListCandlesByMint(ctx context.Context, mint string, resolution string, limit int) ([]TokenCandle, error) {
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

	rows, err := s.model.ListCandlesByMint(ctx, mint, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("list candles: %w", err)
	}

	candles := make([]TokenCandle, 0, len(rows))
	for _, row := range rows {
		candles = append(candles, TokenCandle{
			Time:   row.TimeUnix,
			Open:   row.Open,
			High:   row.High,
			Low:    row.Low,
			Close:  row.Close,
			Volume: row.Volume,
		})
	}

	return candles, nil
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
		H6:  pctChange(summary.LatestPrice, summary.Price6hAgo),
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
