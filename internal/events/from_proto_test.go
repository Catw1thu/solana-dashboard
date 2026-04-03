package events

import (
	"testing"

	serviceeventpb "solana-dashboard-go/internal/gen/serviceeventpb"
)

func TestEnvelopeFromProtoPumpfunTrade(t *testing.T) {
	innerIndex := uint32(7)
	mint := "mint_1"
	bondingCurve := "curve_1"
	user := "user_1"
	creator := "creator_1"
	amount := "1000"
	maxSolCost := "2000"

	protoEvent := &serviceeventpb.EventEnvelope{
		SchemaVersion: 1,
		EventId:       "solana:pumpfun:trade:testsig:inner:3:7",
		Chain:         serviceeventpb.Chain_CHAIN_SOLANA,
		Protocol:      serviceeventpb.Protocol_PROTOCOL_PUMPFUN,
		EventType:     serviceeventpb.EventType_EVENT_TYPE_TRADE,
		Commitment:    serviceeventpb.CommitmentLevel_COMMITMENT_LEVEL_PROCESSED,
		Slot:          42,
		TxSignature:   "testsig",
		TxIndex:       2,
		InstructionPath: &serviceeventpb.InstructionPath{
			Source:     serviceeventpb.InstructionSource_INSTRUCTION_SOURCE_INNER,
			OuterIndex: 3,
			InnerIndex: &innerIndex,
		},
		EventSource: serviceeventpb.EventOrigin_EVENT_ORIGIN_LOGS,
		EventUnixTs: 1_700_000_000,
		Refs: &serviceeventpb.EventRefs{
			Mint:         &mint,
			BondingCurve: &bondingCurve,
			User:         &user,
			Creator:      &creator,
		},
		Payload: &serviceeventpb.EventEnvelope_PumpfunTrade{
			PumpfunTrade: &serviceeventpb.PumpfunTradePayload{
				Side:                   "buy",
				IxName:                 "buy",
				Mint:                   mint,
				User:                   user,
				BondingCurve:           bondingCurve,
				AssociatedBondingCurve: "assoc_curve_1",
				Creator:                creator,
				CreatorVault:           "vault_1",
				TokenProgram:           "token_program_1",
				SolAmount:              "100",
				TokenAmount:            "200",
				Fee:                    "1",
				CreatorFee:             "2",
				VirtualSolReserves:     "300",
				VirtualTokenReserves:   "400",
				RealSolReserves:        "500",
				RealTokenReserves:      "600",
				TrackVolume:            true,
				MayhemMode:             false,
				Cashback:               "0",
				InstructionArgs: &serviceeventpb.PumpfunTradeInstructionArgs{
					Amount:     &amount,
					MaxSolCost: &maxSolCost,
				},
			},
		},
	}

	event, err := EnvelopeFromProto(protoEvent)
	if err != nil {
		t.Fatalf("EnvelopeFromProto returned error: %v", err)
	}

	if event.Protocol != "pumpfun" {
		t.Fatalf("expected protocol=pumpfun, got %s", event.Protocol)
	}
	if event.EventType != "trade" {
		t.Fatalf("expected event_type=trade, got %s", event.EventType)
	}
	if event.InstructionPath.Source != "inner" {
		t.Fatalf("expected instruction_path.source=inner, got %s", event.InstructionPath.Source)
	}
	if event.InstructionPath.InnerIndex == nil || *event.InstructionPath.InnerIndex != 7 {
		t.Fatalf("expected instruction_path.inner_index=7, got %#v", event.InstructionPath.InnerIndex)
	}

	payload, err := DecodePayload(event)
	if err != nil {
		t.Fatalf("DecodePayload returned error: %v", err)
	}

	trade, ok := payload.(PumpfunTradePayload)
	if !ok {
		t.Fatalf("expected PumpfunTradePayload, got %T", payload)
	}
	if trade.Mint != mint {
		t.Fatalf("expected mint=%s, got %s", mint, trade.Mint)
	}
	if trade.InstructionArgs.Amount == nil || *trade.InstructionArgs.Amount != amount {
		t.Fatalf("expected amount=%s, got %#v", amount, trade.InstructionArgs.Amount)
	}
}
