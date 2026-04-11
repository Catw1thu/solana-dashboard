package projector

import (
	"testing"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/store"
)

func TestBuildPumpAmmTradeWhenTrackedMintIsQuoteAndSellMeansBuyToken(t *testing.T) {
	mint := "token_mint"
	event := &events.Envelope{
		EventID:     "event_1",
		TxSignature: "sig_1",
		Slot:        1,
		EventUnixTS: 100,
		EventSource: "logs",
		Refs: events.EventRefs{
			Mint: &mint,
		},
	}

	payload := events.PumpAmmSwapPayload{
		Side:           "sell",
		IxName:         "sell",
		Pool:           "pool_1",
		User:           "user_1",
		BaseMint:       store.SolMint,
		QuoteMint:      mint,
		BaseAmountIn:   ptr("100"),
		QuoteAmountOut: ptr("250"),
	}

	trade, err := buildPumpAmmTrade(event, payload)
	if err != nil {
		t.Fatalf("buildPumpAmmTrade returned error: %v", err)
	}

	if trade.Side != tokenSideBuy {
		t.Fatalf("expected token side buy, got %s", trade.Side)
	}
	if trade.TokenAmountRaw != "250" {
		t.Fatalf("expected token amount raw 250, got %s", trade.TokenAmountRaw)
	}
	if trade.QuoteAmountRaw != "100" {
		t.Fatalf("expected quote amount raw 100, got %s", trade.QuoteAmountRaw)
	}
	if trade.QuoteMint != store.SolMint {
		t.Fatalf("expected quote mint %s, got %s", store.SolMint, trade.QuoteMint)
	}
}

func TestBuildPumpAmmTradeWhenTrackedMintIsQuoteAndBuyMeansSellToken(t *testing.T) {
	mint := "token_mint"
	event := &events.Envelope{
		EventID:     "event_2",
		TxSignature: "sig_2",
		Slot:        1,
		EventUnixTS: 100,
		EventSource: "logs",
		Refs: events.EventRefs{
			Mint: &mint,
		},
	}

	payload := events.PumpAmmSwapPayload{
		Side:          "buy_exact_quote_in",
		IxName:        "buy_exact_quote_in",
		Pool:          "pool_1",
		User:          "user_1",
		BaseMint:      store.SolMint,
		QuoteMint:     mint,
		BaseAmountOut: ptr("90"),
		QuoteAmountIn: ptr("200"),
	}

	trade, err := buildPumpAmmTrade(event, payload)
	if err != nil {
		t.Fatalf("buildPumpAmmTrade returned error: %v", err)
	}

	if trade.Side != tokenSideSell {
		t.Fatalf("expected token side sell, got %s", trade.Side)
	}
	if trade.TokenAmountRaw != "200" {
		t.Fatalf("expected token amount raw 200, got %s", trade.TokenAmountRaw)
	}
	if trade.QuoteAmountRaw != "90" {
		t.Fatalf("expected quote amount raw 90, got %s", trade.QuoteAmountRaw)
	}
	if trade.QuoteMint != store.SolMint {
		t.Fatalf("expected quote mint %s, got %s", store.SolMint, trade.QuoteMint)
	}
}

func ptr(value string) *string {
	return &value
}
