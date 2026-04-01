package projector

import (
	"context"
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

type Projector struct {
	markets marketWriter
	trades  tradeWriter
}

func New(markets marketWriter, trades tradeWriter) *Projector {
	return &Projector{
		markets: markets,
		trades:  trades,
	}
}

func (p *Projector) Project(ctx context.Context, event *events.Envelope, payload any) error {
	switch value := payload.(type) {
	case events.PumpfunCreatePayload:
		return p.markets.UpsertMarket(ctx, buildPumpfunCreateMarket(event, value))
	case events.PumpAmmCreatePoolPayload:
		return p.markets.UpsertMarket(ctx, buildPumpAmmCreatePoolMarket(event, value))
	case events.PumpfunMigratePayload:
		if err := p.markets.CloseMarket(ctx, value.BondingCurve, event.EventUnixTS); err != nil {
			return fmt.Errorf("close pumpfun curve market: %w", err)
		}
		return p.markets.UpsertMarket(ctx, buildPumpfunMigratePoolMarket(event, value))
	case events.PumpfunTradePayload:
		return p.trades.InsertTrade(ctx, buildPumpfunTrade(event, value))
	case events.PumpAmmSwapPayload:
		trade, err := buildPumpAmmTrade(event, value)
		if err != nil {
			return err
		}
		return p.trades.InsertTrade(ctx, trade)
	default:
		return nil
	}
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
