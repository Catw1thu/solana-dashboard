package projector

import (
	"context"
	"encoding/json"
	"fmt"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/store"
)

const (
	marketTypePumpfunCurve = "pumpfun_curve"
	marketTypePumpAmmPool  = "pumpamm_pool"
	protocolPumpfun        = "pumpfun"
	protocolPumpAmm        = "pumpamm"
	stageBondingCurve      = "bonding_curve"
	stagePool              = "pool"
	tokenSideBuy           = "buy"
	tokenSideSell          = "sell"
)

type readModelWriter interface {
	UpsertToken(ctx context.Context, record store.TokenRecord) error
	UpsertTokenMetadata(ctx context.Context, record store.TokenMetadataRecord) error
	UpsertTokenMarket(ctx context.Context, record store.TokenMarketRecord) error
	CloseTokenMarket(ctx context.Context, marketID string, endedAt int64) error
	InsertTradeEvent(ctx context.Context, trade store.TradeEventRecord) error
	InsertActivityEvent(ctx context.Context, activity store.ActivityEventRecord) error
}

type Projector struct {
	store readModelWriter
}

func New(readModel readModelWriter) *Projector {
	return &Projector{store: readModel}
}

func (p *Projector) Project(ctx context.Context, event *events.Envelope, payload any) error {
	if p.store == nil {
		return nil
	}

	switch value := payload.(type) {
	case events.PumpfunCreatePayload:
		if err := p.projectPumpfunCreate(ctx, event, value); err != nil {
			return err
		}
	case events.PumpfunTradePayload:
		if err := p.projectPumpfunTrade(ctx, event, value); err != nil {
			return err
		}
	case events.PumpfunMigratePayload:
		if err := p.projectPumpfunMigrate(ctx, event, value); err != nil {
			return err
		}
	case events.PumpAmmCreatePoolPayload:
		if err := p.projectPumpAmmCreatePool(ctx, event, value); err != nil {
			return err
		}
	case events.PumpAmmSwapPayload:
		if err := p.projectPumpAmmSwap(ctx, event, value); err != nil {
			return err
		}
	case events.PumpAmmLiquidityPayload:
		if err := p.projectPumpAmmLiquidity(ctx, event, value); err != nil {
			return err
		}
	}

	return nil
}

func (p *Projector) projectPumpfunCreate(ctx context.Context, event *events.Envelope, payload events.PumpfunCreatePayload) error {
	createEventID := event.EventID
	creator := ptrIfNotEmpty(payload.Creator)
	bondingCurve := ptrIfNotEmpty(payload.BondingCurve)
	tokenProgram := ptrIfNotEmpty(payload.TokenProgram)
	activeMarketID := ptrIfNotEmpty(payload.BondingCurve)
	activeMarketType := ptrIfNotEmpty(marketTypePumpfunCurve)

	if err := p.store.UpsertToken(ctx, store.TokenRecord{
		Mint:             payload.Mint,
		Creator:          creator,
		BondingCurve:     bondingCurve,
		TokenProgram:     tokenProgram,
		CreateEventID:    &createEventID,
		FirstSeenAt:      event.EventUnixTS,
		CurrentStage:     stageBondingCurve,
		ActiveMarketID:   activeMarketID,
		ActiveMarketType: activeMarketType,
	}); err != nil {
		return err
	}

	if err := p.store.UpsertTokenMetadata(ctx, store.TokenMetadataRecord{
		Mint:              payload.Mint,
		Name:              ptrIfNotEmpty(payload.Name),
		Symbol:            ptrIfNotEmpty(payload.Symbol),
		URI:               ptrIfNotEmpty(payload.URI),
		Decimals:          optionalInt32FromUint32(payload.MintDecimals),
		TotalSupplyRaw:    ptrIfNotEmpty(payload.TokenTotalSupply),
		QuoteMint:         ptrIfNotEmpty(store.SolMint),
		QuoteDecimals:     int32Ptr(store.SolDecimals),
		Creator:           creator,
		BondingCurve:      bondingCurve,
		TokenProgram:      tokenProgram,
		IsMayhemMode:      boolPtr(payload.IsMayhemMode),
		IsCashbackEnabled: boolPtr(payload.IsCashbackEnabled),
		SourceEventID:     &createEventID,
	}); err != nil {
		return err
	}

	if err := p.store.UpsertTokenMarket(ctx, store.TokenMarketRecord{
		MarketID:          payload.BondingCurve,
		Mint:              payload.Mint,
		Protocol:          protocolPumpfun,
		MarketType:        marketTypePumpfunCurve,
		BondingCurve:      bondingCurve,
		BaseMint:          ptrIfNotEmpty(payload.Mint),
		QuoteMint:         ptrIfNotEmpty(store.SolMint),
		BaseMintDecimals:  optionalInt32FromUint32(payload.MintDecimals),
		QuoteMintDecimals: int32Ptr(store.SolDecimals),
		StartedAt:         event.EventUnixTS,
		CreateEventID:     event.EventID,
	}); err != nil {
		return err
	}

	return p.insertActivity(ctx, event, payload.Mint, "create", store.ActivityEventRecord{
		EventID:        event.EventID,
		Mint:           payload.Mint,
		Protocol:       protocolPumpfun,
		EventType:      event.EventType,
		ActivityType:   "create",
		MarketID:       activeMarketID,
		MarketType:     activeMarketType,
		UserAddress:    ptrIfNotEmpty(payload.User),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
	})
}

func (p *Projector) projectPumpfunTrade(ctx context.Context, event *events.Envelope, payload events.PumpfunTradePayload) error {
	trade := store.TradeEventRecord{
		EventID:        event.EventID,
		Mint:           payload.Mint,
		MarketID:       payload.BondingCurve,
		MarketType:     marketTypePumpfunCurve,
		Protocol:       protocolPumpfun,
		Side:           payload.Side,
		IxName:         payload.IxName,
		UserAddress:    payload.User,
		QuoteMint:      store.SolMint,
		TokenAmountRaw: payload.TokenAmount,
		QuoteAmountRaw: payload.SolAmount,
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
	}
	if err := p.store.InsertTradeEvent(ctx, trade); err != nil {
		return err
	}

	return p.insertActivity(ctx, event, payload.Mint, "trade", store.ActivityEventRecord{
		EventID:        event.EventID,
		Mint:           payload.Mint,
		Protocol:       protocolPumpfun,
		EventType:      event.EventType,
		ActivityType:   "trade",
		MarketID:       ptrIfNotEmpty(payload.BondingCurve),
		MarketType:     ptrIfNotEmpty(marketTypePumpfunCurve),
		UserAddress:    ptrIfNotEmpty(payload.User),
		Side:           ptrIfNotEmpty(payload.Side),
		QuoteMint:      ptrIfNotEmpty(store.SolMint),
		TokenAmountRaw: ptrIfNotEmpty(payload.TokenAmount),
		QuoteAmountRaw: ptrIfNotEmpty(payload.SolAmount),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
	})
}

func (p *Projector) projectPumpfunMigrate(ctx context.Context, event *events.Envelope, payload events.PumpfunMigratePayload) error {
	if payload.BondingCurve != "" {
		if err := p.store.CloseTokenMarket(ctx, payload.BondingCurve, event.EventUnixTS); err != nil {
			return err
		}
	}

	migratedAt := event.EventUnixTS
	activeMarketID := ptrIfNotEmpty(payload.Pool)
	activeMarketType := ptrIfNotEmpty(marketTypePumpAmmPool)
	if err := p.store.UpsertToken(ctx, store.TokenRecord{
		Mint:             payload.Mint,
		Creator:          ptrIfNotEmpty(stringValue(event.Refs.Creator)),
		BondingCurve:     ptrIfNotEmpty(payload.BondingCurve),
		TokenProgram:     ptrIfNotEmpty(payload.TokenProgram),
		FirstSeenAt:      event.EventUnixTS,
		CurrentStage:     stagePool,
		ActiveMarketID:   activeMarketID,
		ActiveMarketType: activeMarketType,
		MigratedAt:       &migratedAt,
	}); err != nil {
		return err
	}

	if err := p.store.UpsertTokenMarket(ctx, store.TokenMarketRecord{
		MarketID:      payload.Pool,
		Mint:          payload.Mint,
		Protocol:      protocolPumpAmm,
		MarketType:    marketTypePumpAmmPool,
		Pool:          ptrIfNotEmpty(payload.Pool),
		BaseMint:      ptrIfNotEmpty(payload.Mint),
		QuoteMint:     ptrIfNotEmpty(store.SolMint),
		LPMint:        ptrIfNotEmpty(payload.LPMint),
		StartedAt:     event.EventUnixTS,
		CreateEventID: event.EventID,
	}); err != nil {
		return err
	}

	return p.insertActivity(ctx, event, payload.Mint, "migrate", store.ActivityEventRecord{
		EventID:        event.EventID,
		Mint:           payload.Mint,
		Protocol:       protocolPumpfun,
		EventType:      event.EventType,
		ActivityType:   "migrate",
		MarketID:       activeMarketID,
		MarketType:     activeMarketType,
		UserAddress:    ptrIfNotEmpty(payload.User),
		QuoteMint:      ptrIfNotEmpty(store.SolMint),
		TokenAmountRaw: ptrIfNotEmpty(payload.MintAmount),
		QuoteAmountRaw: ptrIfNotEmpty(payload.SolAmount),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
	})
}

func (p *Projector) projectPumpAmmCreatePool(ctx context.Context, event *events.Envelope, payload events.PumpAmmCreatePoolPayload) error {
	mint := trackedMint(event, payload.BaseMint, payload.QuoteMint)
	if mint == "" {
		return fmt.Errorf("tracked mint missing for pumpamm create_pool event %s", event.EventID)
	}

	metadata := tokenMetadataFromCreatePool(event.EventID, mint, payload)
	activeMarketID := ptrIfNotEmpty(payload.Pool)
	activeMarketType := ptrIfNotEmpty(marketTypePumpAmmPool)

	if err := p.store.UpsertToken(ctx, store.TokenRecord{
		Mint:             mint,
		Creator:          firstNonNil(ptrIfNotEmpty(payload.CoinCreator), ptrIfNotEmpty(payload.Creator)),
		TokenProgram:     nil,
		FirstSeenAt:      event.EventUnixTS,
		CurrentStage:     stagePool,
		ActiveMarketID:   activeMarketID,
		ActiveMarketType: activeMarketType,
	}); err != nil {
		return err
	}

	if err := p.store.UpsertTokenMetadata(ctx, metadata); err != nil {
		return err
	}

	baseMintDecimals := int32(payload.BaseMintDecimals)
	quoteMintDecimals := int32(payload.QuoteMintDecimals)
	if err := p.store.UpsertTokenMarket(ctx, store.TokenMarketRecord{
		MarketID:          payload.Pool,
		Mint:              mint,
		Protocol:          protocolPumpAmm,
		MarketType:        marketTypePumpAmmPool,
		Pool:              ptrIfNotEmpty(payload.Pool),
		BaseMint:          ptrIfNotEmpty(payload.BaseMint),
		QuoteMint:         ptrIfNotEmpty(payload.QuoteMint),
		BaseMintDecimals:  &baseMintDecimals,
		QuoteMintDecimals: &quoteMintDecimals,
		LPMint:            ptrIfNotEmpty(payload.LPMint),
		StartedAt:         event.EventUnixTS,
		CreateEventID:     event.EventID,
	}); err != nil {
		return err
	}

	tokenAmountRaw, quoteAmountRaw := trackedPairAmounts(
		mint,
		payload.BaseMint,
		payload.QuoteMint,
		payload.BaseAmountIn,
		payload.QuoteAmountIn,
	)

	return p.insertActivity(ctx, event, mint, "create_pool", store.ActivityEventRecord{
		EventID:        event.EventID,
		Mint:           mint,
		Protocol:       protocolPumpAmm,
		EventType:      event.EventType,
		ActivityType:   "create_pool",
		MarketID:       activeMarketID,
		MarketType:     activeMarketType,
		UserAddress:    ptrIfNotEmpty(payload.Creator),
		QuoteMint:      metadata.QuoteMint,
		TokenAmountRaw: ptrIfNotEmpty(tokenAmountRaw),
		QuoteAmountRaw: ptrIfNotEmpty(quoteAmountRaw),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
	})
}

func (p *Projector) projectPumpAmmSwap(ctx context.Context, event *events.Envelope, payload events.PumpAmmSwapPayload) error {
	trade, err := buildPumpAmmTrade(event, payload)
	if err != nil {
		return err
	}
	if err := p.store.InsertTradeEvent(ctx, trade); err != nil {
		return err
	}

	return p.insertActivity(ctx, event, trade.Mint, "trade", store.ActivityEventRecord{
		EventID:        trade.EventID,
		Mint:           trade.Mint,
		Protocol:       trade.Protocol,
		EventType:      event.EventType,
		ActivityType:   "trade",
		MarketID:       ptrIfNotEmpty(trade.MarketID),
		MarketType:     ptrIfNotEmpty(trade.MarketType),
		UserAddress:    ptrIfNotEmpty(trade.UserAddress),
		Side:           ptrIfNotEmpty(trade.Side),
		QuoteMint:      ptrIfNotEmpty(trade.QuoteMint),
		TokenAmountRaw: ptrIfNotEmpty(trade.TokenAmountRaw),
		QuoteAmountRaw: ptrIfNotEmpty(trade.QuoteAmountRaw),
		TxSignature:    trade.TxSignature,
		Slot:           trade.Slot,
		EventUnixTS:    trade.EventUnixTS,
		RawEventSource: trade.RawEventSource,
	})
}

func (p *Projector) projectPumpAmmLiquidity(ctx context.Context, event *events.Envelope, payload events.PumpAmmLiquidityPayload) error {
	mint := trackedMint(event, payload.BaseMint, payload.QuoteMint)
	if mint == "" {
		return fmt.Errorf("tracked mint missing for pumpamm liquidity event %s", event.EventID)
	}

	tokenAmountRaw, quoteAmountRaw := liquidityTrackedAmounts(mint, payload)
	return p.insertActivity(ctx, event, mint, payload.Action, store.ActivityEventRecord{
		EventID:        event.EventID,
		Mint:           mint,
		Protocol:       protocolPumpAmm,
		EventType:      event.EventType,
		ActivityType:   payload.Action,
		MarketID:       ptrIfNotEmpty(payload.Pool),
		MarketType:     ptrIfNotEmpty(marketTypePumpAmmPool),
		UserAddress:    ptrIfNotEmpty(payload.User),
		QuoteMint:      ptrIfNotEmpty(otherMint(mint, payload.BaseMint, payload.QuoteMint)),
		TokenAmountRaw: ptrIfNotEmpty(tokenAmountRaw),
		QuoteAmountRaw: ptrIfNotEmpty(quoteAmountRaw),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
	})
}

func (p *Projector) insertActivity(
	ctx context.Context,
	event *events.Envelope,
	mint string,
	activityType string,
	activity store.ActivityEventRecord,
) error {
	details, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal activity payload for event %s: %w", event.EventID, err)
	}

	activity.Mint = mint
	activity.ActivityType = activityType
	activity.Details = details

	return p.store.InsertActivityEvent(ctx, activity)
}

func tokenMetadataFromCreatePool(eventID string, mint string, payload events.PumpAmmCreatePoolPayload) store.TokenMetadataRecord {
	quoteMint := payload.QuoteMint
	tokenDecimals := int32(payload.BaseMintDecimals)
	quoteDecimals := int32(payload.QuoteMintDecimals)
	if mint == payload.QuoteMint {
		quoteMint = payload.BaseMint
		tokenDecimals = int32(payload.QuoteMintDecimals)
		quoteDecimals = int32(payload.BaseMintDecimals)
	}

	return store.TokenMetadataRecord{
		Mint:          mint,
		Decimals:      &tokenDecimals,
		QuoteMint:     ptrIfNotEmpty(quoteMint),
		QuoteDecimals: &quoteDecimals,
		Creator:       firstNonNil(ptrIfNotEmpty(payload.CoinCreator), ptrIfNotEmpty(payload.Creator)),
		SourceEventID: ptrIfNotEmpty(eventID),
	}
}

func buildPumpAmmTrade(event *events.Envelope, payload events.PumpAmmSwapPayload) (store.TradeEventRecord, error) {
	mint := trackedMint(event, payload.BaseMint, payload.QuoteMint)
	if mint == "" {
		return store.TradeEventRecord{}, fmt.Errorf("tracked mint missing for pumpamm swap event %s", event.EventID)
	}

	trade := store.TradeEventRecord{
		EventID:        event.EventID,
		Mint:           mint,
		MarketID:       payload.Pool,
		MarketType:     marketTypePumpAmmPool,
		Protocol:       protocolPumpAmm,
		IxName:         payload.IxName,
		UserAddress:    payload.User,
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
	}

	switch {
	case mint == payload.QuoteMint:
		trade.QuoteMint = payload.BaseMint
		switch payload.Side {
		case "sell":
			trade.Side = tokenSideBuy
			tokenAmount, err := requiredAmount(payload.QuoteAmountOut, "quote_amount_out", event.EventID)
			if err != nil {
				return store.TradeEventRecord{}, err
			}
			quoteAmount, err := requiredAmount(payload.BaseAmountIn, "base_amount_in", event.EventID)
			if err != nil {
				return store.TradeEventRecord{}, err
			}
			trade.TokenAmountRaw = tokenAmount
			trade.QuoteAmountRaw = quoteAmount
		case "buy", "buy_exact_quote_in":
			trade.Side = tokenSideSell
			tokenAmount, err := requiredAmount(payload.QuoteAmountIn, "quote_amount_in", event.EventID)
			if err != nil {
				return store.TradeEventRecord{}, err
			}
			quoteAmount, err := requiredAmount(payload.BaseAmountOut, "base_amount_out", event.EventID)
			if err != nil {
				return store.TradeEventRecord{}, err
			}
			trade.TokenAmountRaw = tokenAmount
			trade.QuoteAmountRaw = quoteAmount
		default:
			return store.TradeEventRecord{}, fmt.Errorf("unsupported pumpamm swap side %s for event %s", payload.Side, event.EventID)
		}
	default:
		trade.QuoteMint = payload.QuoteMint
		switch payload.Side {
		case "sell":
			trade.Side = tokenSideSell
			tokenAmount, err := requiredAmount(payload.BaseAmountIn, "base_amount_in", event.EventID)
			if err != nil {
				return store.TradeEventRecord{}, err
			}
			quoteAmount, err := requiredAmount(payload.QuoteAmountOut, "quote_amount_out", event.EventID)
			if err != nil {
				return store.TradeEventRecord{}, err
			}
			trade.TokenAmountRaw = tokenAmount
			trade.QuoteAmountRaw = quoteAmount
		case "buy", "buy_exact_quote_in":
			trade.Side = tokenSideBuy
			tokenAmount, err := requiredAmount(payload.BaseAmountOut, "base_amount_out", event.EventID)
			if err != nil {
				return store.TradeEventRecord{}, err
			}
			quoteAmount, err := requiredAmount(payload.QuoteAmountIn, "quote_amount_in", event.EventID)
			if err != nil {
				return store.TradeEventRecord{}, err
			}
			trade.TokenAmountRaw = tokenAmount
			trade.QuoteAmountRaw = quoteAmount
		default:
			return store.TradeEventRecord{}, fmt.Errorf("unsupported pumpamm swap side %s for event %s", payload.Side, event.EventID)
		}
	}

	return trade, nil
}

func trackedMint(event *events.Envelope, baseMint string, quoteMint string) string {
	if event.Refs.Mint != nil && *event.Refs.Mint != "" {
		return *event.Refs.Mint
	}

	switch {
	case baseMint == store.SolMint && quoteMint != "":
		return quoteMint
	case quoteMint == store.SolMint && baseMint != "":
		return baseMint
	default:
		return ""
	}
}

func trackedPairAmounts(mint string, baseMint string, quoteMint string, baseAmount string, quoteAmount string) (string, string) {
	if mint == quoteMint {
		return quoteAmount, baseAmount
	}
	return baseAmount, quoteAmount
}

func liquidityTrackedAmounts(mint string, payload events.PumpAmmLiquidityPayload) (string, string) {
	switch payload.Action {
	case "deposit":
		return trackedPairAmounts(
			mint,
			payload.BaseMint,
			payload.QuoteMint,
			stringValue(payload.BaseAmountIn),
			stringValue(payload.QuoteAmountIn),
		)
	case "withdraw":
		return trackedPairAmounts(
			mint,
			payload.BaseMint,
			payload.QuoteMint,
			stringValue(payload.BaseAmountOut),
			stringValue(payload.QuoteAmountOut),
		)
	default:
		return "", ""
	}
}

func otherMint(mint string, baseMint string, quoteMint string) string {
	if mint == baseMint {
		return quoteMint
	}
	if mint == quoteMint {
		return baseMint
	}
	return ""
}

func requiredAmount(value *string, field string, eventID string) (string, error) {
	if value == nil || *value == "" {
		return "", fmt.Errorf("missing %s for event %s", field, eventID)
	}

	return *value, nil
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

func firstNonNil(values ...*string) *string {
	for _, value := range values {
		if value != nil && *value != "" {
			return value
		}
	}
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
