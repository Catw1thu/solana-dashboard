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
			"associated_bonding_curve":"associated_curve_1",
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

	if trade.AssociatedBondingCurve != "associated_curve_1" {
		t.Fatalf("expected associated_bonding_curve=associated_curve_1, got %s", trade.AssociatedBondingCurve)
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

func TestDecodePayloadPumpfunCreate(t *testing.T) {
	event := Envelope{
		Protocol:  "pumpfun",
		EventType: "create",
		Payload: json.RawMessage(`{
			"ix_name":"create_v2",
			"mint":"mint_create",
			"bonding_curve":"curve_create",
			"user":"user_create",
			"creator":"creator_create",
			"name":"Test Token",
			"symbol":"TEST",
			"uri":"https://example.com/token.json",
			"token_program":"token_program_create",
			"virtual_token_reserves":"1000",
			"virtual_sol_reserves":"2000",
			"real_token_reserves":"3000",
			"token_total_supply":"4000",
			"is_mayhem_mode":false,
			"is_cashback_enabled":true
		}`),
	}

	payload, err := DecodePayload(event)
	if err != nil {
		t.Fatalf("DecodePayload returned error: %v", err)
	}

	create, ok := payload.(PumpfunCreatePayload)
	if !ok {
		t.Fatalf("expected PumpfunCreatePayload, got %T", payload)
	}

	if create.Symbol != "TEST" {
		t.Fatalf("expected symbol=TEST, got %s", create.Symbol)
	}
}

func TestDecodePayloadPumpfunMigrate(t *testing.T) {
	event := Envelope{
		Protocol:  "pumpfun",
		EventType: "migrate",
		Payload: json.RawMessage(`{
			"mint":"mint_migrate",
			"user":"user_migrate",
			"bonding_curve":"curve_migrate",
			"pool":"pool_migrate",
			"mint_amount":"100",
			"sol_amount":"200",
			"pool_migration_fee":"3",
			"withdraw_authority":"withdraw_auth",
			"associated_bonding_curve":"associated_curve",
			"token_program":"token_program_migrate",
			"pump_amm":"pumpamm_program",
			"pool_authority":"pool_auth",
			"lp_mint":"lp_mint_1"
		}`),
	}

	payload, err := DecodePayload(event)
	if err != nil {
		t.Fatalf("DecodePayload returned error: %v", err)
	}

	migrate, ok := payload.(PumpfunMigratePayload)
	if !ok {
		t.Fatalf("expected PumpfunMigratePayload, got %T", payload)
	}

	if migrate.Pool != "pool_migrate" {
		t.Fatalf("expected pool=pool_migrate, got %s", migrate.Pool)
	}
}

func TestDecodePayloadPumpAmmCreatePool(t *testing.T) {
	event := Envelope{
		Protocol:  "pumpamm",
		EventType: "create_pool",
		Payload: json.RawMessage(`{
			"pool":"pool_create",
			"creator":"creator_create",
			"base_mint":"base_create",
			"quote_mint":"quote_create",
			"lp_mint":"lp_create",
			"base_amount_in":"100",
			"quote_amount_in":"200",
			"initial_liquidity":"300",
			"coin_creator":"coin_creator_1",
			"is_mayhem_mode":true,
			"instruction_args":{
				"index":7,
				"coin_creator":"coin_creator_1",
				"is_mayhem_mode":true,
				"is_cashback_coin":false
			}
		}`),
	}

	payload, err := DecodePayload(event)
	if err != nil {
		t.Fatalf("DecodePayload returned error: %v", err)
	}

	createPool, ok := payload.(PumpAmmCreatePoolPayload)
	if !ok {
		t.Fatalf("expected PumpAmmCreatePoolPayload, got %T", payload)
	}

	if createPool.Pool != "pool_create" {
		t.Fatalf("expected pool=pool_create, got %s", createPool.Pool)
	}
}

func TestDecodePayloadPumpAmmLiquidity(t *testing.T) {
	event := Envelope{
		Protocol:  "pumpamm",
		EventType: "deposit",
		Payload: json.RawMessage(`{
			"action":"deposit",
			"pool":"pool_liquidity",
			"user":"user_liquidity",
			"base_mint":"base_liquidity",
			"quote_mint":"quote_liquidity",
			"lp_mint":"lp_liquidity",
			"lp_token_amount_in":null,
			"lp_token_amount_out":"10",
			"base_amount_in":"20",
			"quote_amount_in":"30",
			"base_amount_out":null,
			"quote_amount_out":null,
			"lp_mint_supply":"1000",
			"instruction_args":{
				"lp_token_amount_in":null,
				"lp_token_amount_out":"10",
				"max_base_amount_in":"20",
				"max_quote_amount_in":"30",
				"min_base_amount_out":null,
				"min_quote_amount_out":null
			}
		}`),
	}

	payload, err := DecodePayload(event)
	if err != nil {
		t.Fatalf("DecodePayload returned error: %v", err)
	}

	liquidity, ok := payload.(PumpAmmLiquidityPayload)
	if !ok {
		t.Fatalf("expected PumpAmmLiquidityPayload, got %T", payload)
	}

	if liquidity.Action != "deposit" {
		t.Fatalf("expected action=deposit, got %s", liquidity.Action)
	}
}
