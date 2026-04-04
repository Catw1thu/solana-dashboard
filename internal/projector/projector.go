package projector

import (
	"context"
	"encoding/json"
	"fmt"
	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/store"
)

const (
	solMint                = "So11111111111111111111111111111111111111112"
	marketTypePumpfunCurve = "pumpfun_curve"
	marketTypePumpAmmPool  = "pumpamm_pool"
	protocolPumpfun        = "pumpfun"
	protocolPumpAmm        = "pumpamm"
	tokenSideBuy           = "buy"
	tokenSideSell          = "sell"
)

type marketWriter interface {
	UpsertMarket(ctx context.Context, market store.MarketRecord) error
	CloseMarket(ctx context.Context, marketID string, endedAt int64) error
}

type tradeWriter interface {
	InsertTrade(ctx context.Context, trade store.TradeRecord) error
}

type timelineWriter interface {
	InsertTimelineEvent(ctx context.Context, item store.TokenTimelineRecord) error
}

type Projector struct {
	markets  marketWriter
	trades   tradeWriter
	timeline timelineWriter
}

func New(markets marketWriter, trades tradeWriter, timeline timelineWriter) *Projector {
	return &Projector{
		markets:  markets,
		trades:   trades,
		timeline: timeline,
	}
}

func (p *Projector) Project(ctx context.Context, event *events.Envelope, payload any) error {
	switch value := payload.(type) {
	case events.PumpfunCreatePayload:
		if err := p.markets.UpsertMarket(ctx, buildPumpfunCreateMarket(event, value)); err != nil {
			return err
		}
		timeline, err := buildPumpfunCreateTimeline(event, value)
		if err != nil {
			return err
		}
		return p.insertTimeline(ctx, timeline)
	case events.PumpAmmCreatePoolPayload:
		if err := p.markets.UpsertMarket(ctx, buildPumpAmmCreatePoolMarket(event, value)); err != nil {
			return err
		}
		timeline, err := buildPumpAmmCreatePoolTimeline(event, value)
		if err != nil {
			return err
		}
		return p.insertTimeline(ctx, timeline)
	case events.PumpfunMigratePayload:
		if err := p.markets.CloseMarket(ctx, value.BondingCurve, event.EventUnixTS); err != nil {
			return fmt.Errorf("close pumpfun curve market: %w", err)
		}
		if err := p.markets.UpsertMarket(ctx, buildPumpfunMigratePoolMarket(event, value)); err != nil {
			return err
		}
		timeline, err := buildPumpfunMigrateTimeline(event, value)
		if err != nil {
			return err
		}
		return p.insertTimeline(ctx, timeline)
	case events.PumpfunTradePayload:
		trade := buildPumpfunTrade(event, value)
		if err := p.trades.InsertTrade(ctx, trade); err != nil {
			return err
		}
		timeline, err := buildTradeTimeline(event, trade)
		if err != nil {
			return err
		}
		return p.insertTimeline(ctx, timeline)
	case events.PumpAmmSwapPayload:
		trade, err := buildPumpAmmTrade(event, value)
		if err != nil {
			return err
		}
		if err := p.trades.InsertTrade(ctx, trade); err != nil {
			return err
		}
		timeline, err := buildTradeTimeline(event, trade)
		if err != nil {
			return err
		}
		return p.insertTimeline(ctx, timeline)
	case events.PumpAmmLiquidityPayload:
		timeline, err := buildPumpAmmLiquidityTimeline(event, value)
		if err != nil {
			return err
		}
		return p.insertTimeline(ctx, timeline)
	default:
		return nil
	}
}

func (p *Projector) insertTimeline(ctx context.Context, item store.TokenTimelineRecord) error {
	if p.timeline == nil {
		return nil
	}
	return p.timeline.InsertTimelineEvent(ctx, item)
}

func buildPumpfunCreateMarket(event *events.Envelope, payload events.PumpfunCreatePayload) store.MarketRecord {
	bondingCurve := payload.BondingCurve
	quoteMint := solMint

	return store.MarketRecord{
		MarketID:      bondingCurve,
		Mint:          payload.Mint,
		Protocol:      protocolPumpfun,
		MarketType:    marketTypePumpfunCurve,
		BondingCurve:  &bondingCurve,
		QuoteMint:     &quoteMint,
		StartedAt:     event.EventUnixTS,
		CreateEventID: event.EventID,
	}
}

func buildPumpAmmCreatePoolMarket(event *events.Envelope, payload events.PumpAmmCreatePoolPayload) store.MarketRecord {
	pool := payload.Pool
	baseMint := payload.BaseMint
	quoteMint := payload.QuoteMint
	lpMint := payload.LPMint

	return store.MarketRecord{
		MarketID:      payload.Pool,
		Mint:          trackedMint(event, payload.BaseMint, payload.QuoteMint),
		Protocol:      protocolPumpAmm,
		MarketType:    marketTypePumpAmmPool,
		Pool:          &pool,
		BaseMint:      &baseMint,
		QuoteMint:     &quoteMint,
		LPMint:        &lpMint,
		StartedAt:     event.EventUnixTS,
		CreateEventID: event.EventID,
	}
}

func buildPumpfunMigratePoolMarket(event *events.Envelope, payload events.PumpfunMigratePayload) store.MarketRecord {
	pool := payload.Pool
	lpMint := payload.LPMint

	return store.MarketRecord{
		MarketID:      payload.Pool,
		Mint:          payload.Mint,
		Protocol:      protocolPumpAmm,
		MarketType:    marketTypePumpAmmPool,
		Pool:          &pool,
		LPMint:        &lpMint,
		StartedAt:     event.EventUnixTS,
		CreateEventID: event.EventID,
	}
}

func buildPumpfunTrade(event *events.Envelope, payload events.PumpfunTradePayload) store.TradeRecord {
	bondingCurve := payload.BondingCurve

	return store.TradeRecord{
		EventID:        event.EventID,
		Mint:           payload.Mint,
		MarketID:       payload.BondingCurve,
		MarketType:     marketTypePumpfunCurve,
		Protocol:       protocolPumpfun,
		Side:           payload.Side,
		IxName:         payload.IxName,
		UserAddress:    payload.User,
		BondingCurve:   &bondingCurve,
		QuoteMint:      solMint,
		TokenAmount:    payload.TokenAmount,
		QuoteAmount:    payload.SolAmount,
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
	}
}

func buildPumpAmmTrade(event *events.Envelope, payload events.PumpAmmSwapPayload) (store.TradeRecord, error) {
	mint := trackedMint(event, payload.BaseMint, payload.QuoteMint)
	if mint == "" {
		return store.TradeRecord{}, fmt.Errorf("tracked mint missing for pumpamm swap event %s", event.EventID)
	}

	pool := payload.Pool
	trade := store.TradeRecord{
		EventID:        event.EventID,
		Mint:           mint,
		MarketID:       payload.Pool,
		MarketType:     marketTypePumpAmmPool,
		Protocol:       protocolPumpAmm,
		IxName:         payload.IxName,
		UserAddress:    payload.User,
		Pool:           &pool,
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
				return store.TradeRecord{}, err
			}
			quoteAmount, err := requiredAmount(payload.BaseAmountIn, "base_amount_in", event.EventID)
			if err != nil {
				return store.TradeRecord{}, err
			}
			trade.TokenAmount = tokenAmount
			trade.QuoteAmount = quoteAmount
		case "buy", "buy_exact_quote_in":
			trade.Side = tokenSideSell
			tokenAmount, err := requiredAmount(payload.QuoteAmountIn, "quote_amount_in", event.EventID)
			if err != nil {
				return store.TradeRecord{}, err
			}
			quoteAmount, err := requiredAmount(payload.BaseAmountOut, "base_amount_out", event.EventID)
			if err != nil {
				return store.TradeRecord{}, err
			}
			trade.TokenAmount = tokenAmount
			trade.QuoteAmount = quoteAmount
		default:
			return store.TradeRecord{}, fmt.Errorf("unsupported pumpamm swap side %q for event %s", payload.Side, event.EventID)
		}
	case mint == payload.BaseMint:
		trade.QuoteMint = payload.QuoteMint
		switch payload.Side {
		case "sell":
			trade.Side = tokenSideSell
			tokenAmount, err := requiredAmount(payload.BaseAmountIn, "base_amount_in", event.EventID)
			if err != nil {
				return store.TradeRecord{}, err
			}
			quoteAmount, err := requiredAmount(payload.QuoteAmountOut, "quote_amount_out", event.EventID)
			if err != nil {
				return store.TradeRecord{}, err
			}
			trade.TokenAmount = tokenAmount
			trade.QuoteAmount = quoteAmount
		case "buy", "buy_exact_quote_in":
			trade.Side = tokenSideBuy
			tokenAmount, err := requiredAmount(payload.BaseAmountOut, "base_amount_out", event.EventID)
			if err != nil {
				return store.TradeRecord{}, err
			}
			quoteAmount, err := requiredAmount(payload.QuoteAmountIn, "quote_amount_in", event.EventID)
			if err != nil {
				return store.TradeRecord{}, err
			}
			trade.TokenAmount = tokenAmount
			trade.QuoteAmount = quoteAmount
		default:
			return store.TradeRecord{}, fmt.Errorf("unsupported pumpamm swap side %q for event %s", payload.Side, event.EventID)
		}
	default:
		return store.TradeRecord{}, fmt.Errorf(
			"tracked mint %s does not match pumpamm swap pair %s/%s for event %s",
			mint,
			payload.BaseMint,
			payload.QuoteMint,
			event.EventID,
		)
	}

	return trade, nil
}

func buildPumpfunCreateTimeline(event *events.Envelope, payload events.PumpfunCreatePayload) (store.TokenTimelineRecord, error) {
	details, err := json.Marshal(map[string]any{
		"ix_name":        payload.IxName,
		"name":           payload.Name,
		"symbol":         payload.Symbol,
		"uri":            payload.URI,
		"creator":        payload.Creator,
		"user":           payload.User,
		"token_program":  payload.TokenProgram,
		"virtual_sol":    payload.VirtualSolReserves,
		"virtual_token":  payload.VirtualTokenReserves,
		"real_token":     payload.RealTokenReserves,
		"total_supply":   payload.TokenTotalSupply,
		"is_mayhem_mode": payload.IsMayhemMode,
	})
	if err != nil {
		return store.TokenTimelineRecord{}, fmt.Errorf("marshal create timeline details: %w", err)
	}

	return store.TokenTimelineRecord{
		EventID:        event.EventID,
		Mint:           payload.Mint,
		Protocol:       event.Protocol,
		EventType:      event.EventType,
		TimelineType:   "create",
		MarketID:       ptrIfNotEmpty(payload.BondingCurve),
		MarketType:     ptrIfNotEmpty(marketTypePumpfunCurve),
		UserAddress:    ptrIfNotEmpty(payload.Creator),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
		Details:        details,
	}, nil
}

func buildPumpAmmCreatePoolTimeline(event *events.Envelope, payload events.PumpAmmCreatePoolPayload) (store.TokenTimelineRecord, error) {
	mint := trackedMint(event, payload.BaseMint, payload.QuoteMint)
	details, err := json.Marshal(map[string]any{
		"creator":           payload.Creator,
		"base_mint":         payload.BaseMint,
		"quote_mint":        payload.QuoteMint,
		"lp_mint":           payload.LPMint,
		"base_amount_in":    payload.BaseAmountIn,
		"quote_amount_in":   payload.QuoteAmountIn,
		"initial_liquidity": payload.InitialLiquidity,
		"coin_creator":      payload.CoinCreator,
		"is_mayhem_mode":    payload.IsMayhemMode,
	})
	if err != nil {
		return store.TokenTimelineRecord{}, fmt.Errorf("marshal create_pool timeline details: %w", err)
	}

	return store.TokenTimelineRecord{
		EventID:        event.EventID,
		Mint:           mint,
		Protocol:       event.Protocol,
		EventType:      event.EventType,
		TimelineType:   "create_pool",
		MarketID:       ptrIfNotEmpty(payload.Pool),
		MarketType:     ptrIfNotEmpty(marketTypePumpAmmPool),
		UserAddress:    ptrIfNotEmpty(payload.Creator),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
		Details:        details,
	}, nil
}

func buildPumpfunMigrateTimeline(event *events.Envelope, payload events.PumpfunMigratePayload) (store.TokenTimelineRecord, error) {
	details, err := json.Marshal(map[string]any{
		"user":           payload.User,
		"bonding_curve":  payload.BondingCurve,
		"pool":           payload.Pool,
		"mint_amount":    payload.MintAmount,
		"sol_amount":     payload.SolAmount,
		"migration_fee":  payload.PoolMigrationFee,
		"lp_mint":        payload.LPMint,
		"token_program":  payload.TokenProgram,
		"withdraw_auth":  payload.WithdrawAuthority,
		"pool_authority": payload.PoolAuthority,
	})
	if err != nil {
		return store.TokenTimelineRecord{}, fmt.Errorf("marshal migrate timeline details: %w", err)
	}

	return store.TokenTimelineRecord{
		EventID:        event.EventID,
		Mint:           payload.Mint,
		Protocol:       event.Protocol,
		EventType:      event.EventType,
		TimelineType:   "migrate",
		MarketID:       ptrIfNotEmpty(payload.Pool),
		MarketType:     ptrIfNotEmpty(marketTypePumpAmmPool),
		UserAddress:    ptrIfNotEmpty(payload.User),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
		Details:        details,
	}, nil
}

func buildTradeTimeline(event *events.Envelope, trade store.TradeRecord) (store.TokenTimelineRecord, error) {
	details, err := json.Marshal(map[string]any{
		"ix_name": trade.IxName,
	})
	if err != nil {
		return store.TokenTimelineRecord{}, fmt.Errorf("marshal trade timeline details: %w", err)
	}

	return store.TokenTimelineRecord{
		EventID:        event.EventID,
		Mint:           trade.Mint,
		Protocol:       event.Protocol,
		EventType:      event.EventType,
		TimelineType:   "trade",
		MarketID:       ptrIfNotEmpty(trade.MarketID),
		MarketType:     ptrIfNotEmpty(trade.MarketType),
		UserAddress:    ptrIfNotEmpty(trade.UserAddress),
		Side:           ptrIfNotEmpty(trade.Side),
		QuoteMint:      ptrIfNotEmpty(trade.QuoteMint),
		TokenAmount:    ptrIfNotEmpty(trade.TokenAmount),
		QuoteAmount:    ptrIfNotEmpty(trade.QuoteAmount),
		TxSignature:    trade.TxSignature,
		Slot:           trade.Slot,
		EventUnixTS:    trade.EventUnixTS,
		RawEventSource: trade.RawEventSource,
		Details:        details,
	}, nil
}

func buildPumpAmmLiquidityTimeline(event *events.Envelope, payload events.PumpAmmLiquidityPayload) (store.TokenTimelineRecord, error) {
	mint := trackedMint(event, payload.BaseMint, payload.QuoteMint)
	details, err := json.Marshal(map[string]any{
		"action":              payload.Action,
		"pool":                payload.Pool,
		"base_mint":           payload.BaseMint,
		"quote_mint":          payload.QuoteMint,
		"lp_mint":             payload.LPMint,
		"lp_token_amount_in":  payload.LPTokenAmountIn,
		"lp_token_amount_out": payload.LPTokenAmountOut,
		"base_amount_in":      payload.BaseAmountIn,
		"quote_amount_in":     payload.QuoteAmountIn,
		"base_amount_out":     payload.BaseAmountOut,
		"quote_amount_out":    payload.QuoteAmountOut,
		"lp_mint_supply":      payload.LPMintSupply,
	})
	if err != nil {
		return store.TokenTimelineRecord{}, fmt.Errorf("marshal liquidity timeline details: %w", err)
	}

	return store.TokenTimelineRecord{
		EventID:        event.EventID,
		Mint:           mint,
		Protocol:       event.Protocol,
		EventType:      event.EventType,
		TimelineType:   "liquidity",
		MarketID:       ptrIfNotEmpty(payload.Pool),
		MarketType:     ptrIfNotEmpty(marketTypePumpAmmPool),
		UserAddress:    ptrIfNotEmpty(payload.User),
		TxSignature:    event.TxSignature,
		Slot:           event.Slot,
		EventUnixTS:    event.EventUnixTS,
		RawEventSource: event.EventSource,
		Details:        details,
	}, nil
}

func trackedMint(event *events.Envelope, baseMint string, quoteMint string) string {
	if event.Refs.Mint != nil && *event.Refs.Mint != "" {
		return *event.Refs.Mint
	}

	switch {
	case baseMint == solMint && quoteMint != "":
		return quoteMint
	case quoteMint == solMint && baseMint != "":
		return baseMint
	default:
		return ""
	}
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
