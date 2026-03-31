package events

import (
	"encoding/json"
	"testing"
)

func TestDecodePayloadPumpfunTrade(t *testing.T) {
	event := Envelope{
		Protocol:  "pumpfun",
		EventType: "trade",
		Payload: json.RawMessage(`{
			"side":"buy",
			"ix_name":"buy",
			"mint":"mint_1",
			"user":"user_1",
			"bonding_curve":"curve_1",
			"creator":"creator_1",
			"creator_vault":"vault_1",
			"token_program":"token_program_1",
			"sol_amount":"100",
			"token_amount":"200",
			"fee":"1",
			"creator_fee":"2",
			"virtual_sol_reserves":"300",
			"virtual_token_reserves":"400",
			"real_sol_reserves":"500",
			"real_token_reserves":"600",
			"track_volume":true,
			"mayhem_mode":false,
			"cashback":"0",
			"instruction_args":{
				"amount":"1000",
				"max_sol_cost":"2000",
				"min_sol_output":null,
				"spendable_sol_in":null,
				"min_tokens_out":null
			}
		}`),
	}

	payload, err := DecodePayload(event)
	if err != nil {
		t.Fatalf("DecodePayload returned error: %v", err)
	}

	trade, ok := payload.(PumpfunTradePayload)
	if !ok {
		t.Fatalf("expected PumpfunTradePayload, got %T", payload)
	}

	if trade.Side != "buy" {
		t.Fatalf("expected side=buy, got %s", trade.Side)
	}

	if trade.Mint != "mint_1" {
		t.Fatalf("expected mint=mint_1, got %s", trade.Mint)
	}

	if trade.InstructionArgs.Amount == nil || *trade.InstructionArgs.Amount != "1000" {
		t.Fatalf("expected instruction_args.amount=1000, got %#v", trade.InstructionArgs.Amount)
	}
}

func TestDecodePayloadPumpAmmSwap(t *testing.T) {
	event := Envelope{
		Protocol:  "pumpamm",
		EventType: "swap",
		Payload: json.RawMessage(`{
			"side":"sell",
			"ix_name":"sell",
			"pool":"pool_1",
			"user":"user_2",
			"base_mint":"base_1",
			"quote_mint":"quote_1",
			"coin_creator":"creator_2",
			"base_amount_in":"123",
			"base_amount_out":null,
			"quote_amount_in":null,
			"quote_amount_out":"456",
			"lp_fee":"3",
			"protocol_fee":"4",
			"coin_creator_fee":"5",
			"cashback":"0",
			"pool_base_token_reserves":"1000",
			"pool_quote_token_reserves":"2000",
			"instruction_args":{
				"base_amount_in":"123",
				"min_quote_amount_out":"450",
				"base_amount_out":null,
				"max_quote_amount_in":null,
				"spendable_quote_in":null,
				"min_base_amount_out":null
			}
		}`),
	}

	payload, err := DecodePayload(event)
	if err != nil {
		t.Fatalf("DecodePayload returned error: %v", err)
	}

	swap, ok := payload.(PumpAmmSwapPayload)
	if !ok {
		t.Fatalf("expected PumpAmmSwapPayload, got %T", payload)
	}

	if swap.Side != "sell" {
		t.Fatalf("expected side=sell, got %s", swap.Side)
	}

	if swap.Pool != "pool_1" {
		t.Fatalf("expected pool=pool_1, got %s", swap.Pool)
	}

	if swap.InstructionArgs.BaseAmountIn == nil || *swap.InstructionArgs.BaseAmountIn != "123" {
		t.Fatalf("expected instruction_args.base_amount_in=123, got %#v", swap.InstructionArgs.BaseAmountIn)
	}
}
