package events

import (
	"fmt"

	serviceeventpb "solana-dashboard-go/internal/gen/serviceeventpb"
)

func DecodedEnvelopeFromProto(event *serviceeventpb.EventEnvelope) (DecodedEnvelope, error) {
	envelope, err := envelopeMetadataFromProto(event)
	if err != nil {
		return DecodedEnvelope{}, err
	}

	payload, err := payloadFromProto(event)
	if err != nil {
		return DecodedEnvelope{}, err
	}

	return DecodedEnvelope{
		Envelope: envelope,
		Payload:  payload,
	}, nil
}

func EnvelopeFromProto(event *serviceeventpb.EventEnvelope) (Envelope, error) {
	decoded, err := DecodedEnvelopeFromProto(event)
	if err != nil {
		return Envelope{}, err
	}

	return decoded.EnvelopeWithPayload()
}

func envelopeMetadataFromProto(event *serviceeventpb.EventEnvelope) (Envelope, error) {
	if event == nil {
		return Envelope{}, fmt.Errorf("proto event is nil")
	}

	instructionPath, err := instructionPathFromProto(event.GetInstructionPath())
	if err != nil {
		return Envelope{}, err
	}

	return Envelope{
		SchemaVersion:   int(event.GetSchemaVersion()),
		EventID:         event.GetEventId(),
		Chain:           chainFromProto(event.GetChain()),
		Protocol:        protocolFromProto(event.GetProtocol()),
		EventType:       eventTypeFromProto(event.GetEventType()),
		Commitment:      commitmentFromProto(event.GetCommitment()),
		Slot:            event.GetSlot(),
		TxSignature:     event.GetTxSignature(),
		TxIndex:         event.GetTxIndex(),
		InstructionPath: instructionPath,
		EventSource:     eventOriginFromProto(event.GetEventSource()),
		EventUnixTS:     event.GetEventUnixTs(),
		Refs:            refsFromProto(event.GetRefs()),
	}, nil
}

func DecodeEnvelope(event Envelope) (DecodedEnvelope, error) {
	payload, err := DecodePayload(event)
	if err != nil {
		return DecodedEnvelope{}, err
	}

	return DecodedEnvelope{
		Envelope: event,
		Payload:  payload,
	}, nil
}

func payloadFromProto(event *serviceeventpb.EventEnvelope) (any, error) {
	switch payload := event.Payload.(type) {
	case *serviceeventpb.EventEnvelope_PumpfunTrade:
		return pumpfunTradePayloadFromProto(payload.PumpfunTrade), nil
	case *serviceeventpb.EventEnvelope_PumpfunCreate:
		return pumpfunCreatePayloadFromProto(payload.PumpfunCreate), nil
	case *serviceeventpb.EventEnvelope_PumpfunMigrate:
		return pumpfunMigratePayloadFromProto(payload.PumpfunMigrate), nil
	case *serviceeventpb.EventEnvelope_PumpammSwap:
		return pumpAmmSwapPayloadFromProto(payload.PumpammSwap), nil
	case *serviceeventpb.EventEnvelope_PumpammCreatePool:
		return pumpAmmCreatePoolPayloadFromProto(payload.PumpammCreatePool), nil
	case *serviceeventpb.EventEnvelope_PumpammDeposit:
		return pumpAmmLiquidityPayloadFromProto(payload.PumpammDeposit), nil
	case *serviceeventpb.EventEnvelope_PumpammWithdraw:
		return pumpAmmLiquidityPayloadFromProto(payload.PumpammWithdraw), nil
	default:
		return nil, fmt.Errorf("unsupported proto payload type %T", payload)
	}
}

func pumpfunTradePayloadFromProto(payload *serviceeventpb.PumpfunTradePayload) PumpfunTradePayload {
	if payload == nil {
		return PumpfunTradePayload{}
	}

	args := payload.GetInstructionArgs()

	return PumpfunTradePayload{
		Side:                   payload.GetSide(),
		IxName:                 payload.GetIxName(),
		Mint:                   payload.GetMint(),
		User:                   payload.GetUser(),
		BondingCurve:           payload.GetBondingCurve(),
		AssociatedBondingCurve: payload.GetAssociatedBondingCurve(),
		Creator:                payload.GetCreator(),
		CreatorVault:           payload.GetCreatorVault(),
		TokenProgram:           payload.GetTokenProgram(),
		SolAmount:              payload.GetSolAmount(),
		TokenAmount:            payload.GetTokenAmount(),
		Fee:                    payload.GetFee(),
		CreatorFee:             payload.GetCreatorFee(),
		VirtualSolReserves:     payload.GetVirtualSolReserves(),
		VirtualTokenReserves:   payload.GetVirtualTokenReserves(),
		RealSolReserves:        payload.GetRealSolReserves(),
		RealTokenReserves:      payload.GetRealTokenReserves(),
		TrackVolume:            payload.GetTrackVolume(),
		MayhemMode:             payload.GetMayhemMode(),
		Cashback:               payload.GetCashback(),
		InstructionArgs: PumpfunTradeInstructionArgs{
			Amount:         optionalStringFromPumpfunTradeArgs(args, func(v *serviceeventpb.PumpfunTradeInstructionArgs) *string { return v.Amount }),
			MaxSolCost:     optionalStringFromPumpfunTradeArgs(args, func(v *serviceeventpb.PumpfunTradeInstructionArgs) *string { return v.MaxSolCost }),
			MinSolOutput:   optionalStringFromPumpfunTradeArgs(args, func(v *serviceeventpb.PumpfunTradeInstructionArgs) *string { return v.MinSolOutput }),
			SpendableSolIn: optionalStringFromPumpfunTradeArgs(args, func(v *serviceeventpb.PumpfunTradeInstructionArgs) *string { return v.SpendableSolIn }),
			MinTokensOut:   optionalStringFromPumpfunTradeArgs(args, func(v *serviceeventpb.PumpfunTradeInstructionArgs) *string { return v.MinTokensOut }),
		},
	}
}

func pumpfunCreatePayloadFromProto(payload *serviceeventpb.PumpfunCreatePayload) PumpfunCreatePayload {
	if payload == nil {
		return PumpfunCreatePayload{}
	}

	return PumpfunCreatePayload{
		IxName:               payload.GetIxName(),
		Mint:                 payload.GetMint(),
		BondingCurve:         payload.GetBondingCurve(),
		User:                 payload.GetUser(),
		Creator:              payload.GetCreator(),
		Name:                 payload.GetName(),
		Symbol:               payload.GetSymbol(),
		URI:                  payload.GetUri(),
		TokenProgram:         payload.GetTokenProgram(),
		VirtualTokenReserves: payload.GetVirtualTokenReserves(),
		VirtualSolReserves:   payload.GetVirtualSolReserves(),
		RealTokenReserves:    payload.GetRealTokenReserves(),
		TokenTotalSupply:     payload.GetTokenTotalSupply(),
		IsMayhemMode:         payload.GetIsMayhemMode(),
		IsCashbackEnabled:    payload.GetIsCashbackEnabled(),
	}
}

func pumpfunMigratePayloadFromProto(payload *serviceeventpb.PumpfunMigratePayload) PumpfunMigratePayload {
	if payload == nil {
		return PumpfunMigratePayload{}
	}

	return PumpfunMigratePayload{
		Mint:                   payload.GetMint(),
		User:                   payload.GetUser(),
		BondingCurve:           payload.GetBondingCurve(),
		Pool:                   payload.GetPool(),
		MintAmount:             payload.GetMintAmount(),
		SolAmount:              payload.GetSolAmount(),
		PoolMigrationFee:       payload.GetPoolMigrationFee(),
		WithdrawAuthority:      payload.GetWithdrawAuthority(),
		AssociatedBondingCurve: payload.GetAssociatedBondingCurve(),
		TokenProgram:           payload.GetTokenProgram(),
		PumpAmm:                payload.GetPumpAmm(),
		PoolAuthority:          payload.GetPoolAuthority(),
		LPMint:                 payload.GetLpMint(),
	}
}

func pumpAmmSwapPayloadFromProto(payload *serviceeventpb.PumpAmmSwapPayload) PumpAmmSwapPayload {
	if payload == nil {
		return PumpAmmSwapPayload{}
	}

	args := payload.GetInstructionArgs()

	return PumpAmmSwapPayload{
		Side:                   payload.GetSide(),
		IxName:                 payload.GetIxName(),
		Pool:                   payload.GetPool(),
		User:                   payload.GetUser(),
		BaseMint:               payload.GetBaseMint(),
		QuoteMint:              payload.GetQuoteMint(),
		CoinCreator:            payload.GetCoinCreator(),
		BaseAmountIn:           payload.BaseAmountIn,
		BaseAmountOut:          payload.BaseAmountOut,
		QuoteAmountIn:          payload.QuoteAmountIn,
		QuoteAmountOut:         payload.QuoteAmountOut,
		LpFee:                  payload.GetLpFee(),
		ProtocolFee:            payload.GetProtocolFee(),
		CoinCreatorFee:         payload.GetCoinCreatorFee(),
		Cashback:               payload.GetCashback(),
		PoolBaseTokenReserves:  payload.GetPoolBaseTokenReserves(),
		PoolQuoteTokenReserves: payload.GetPoolQuoteTokenReserves(),
		InstructionArgs: PumpAmmSwapInstructionArgs{
			BaseAmountIn:      optionalStringFromPumpAmmSwapArgs(args, func(v *serviceeventpb.PumpAmmSwapInstructionArgs) *string { return v.BaseAmountIn }),
			MinQuoteAmountOut: optionalStringFromPumpAmmSwapArgs(args, func(v *serviceeventpb.PumpAmmSwapInstructionArgs) *string { return v.MinQuoteAmountOut }),
			BaseAmountOut:     optionalStringFromPumpAmmSwapArgs(args, func(v *serviceeventpb.PumpAmmSwapInstructionArgs) *string { return v.BaseAmountOut }),
			MaxQuoteAmountIn:  optionalStringFromPumpAmmSwapArgs(args, func(v *serviceeventpb.PumpAmmSwapInstructionArgs) *string { return v.MaxQuoteAmountIn }),
			SpendableQuoteIn:  optionalStringFromPumpAmmSwapArgs(args, func(v *serviceeventpb.PumpAmmSwapInstructionArgs) *string { return v.SpendableQuoteIn }),
			MinBaseAmountOut:  optionalStringFromPumpAmmSwapArgs(args, func(v *serviceeventpb.PumpAmmSwapInstructionArgs) *string { return v.MinBaseAmountOut }),
		},
	}
}

func pumpAmmCreatePoolPayloadFromProto(payload *serviceeventpb.PumpAmmCreatePoolPayload) PumpAmmCreatePoolPayload {
	if payload == nil {
		return PumpAmmCreatePoolPayload{}
	}

	args := payload.GetInstructionArgs()

	return PumpAmmCreatePoolPayload{
		Pool:             payload.GetPool(),
		Creator:          payload.GetCreator(),
		BaseMint:         payload.GetBaseMint(),
		QuoteMint:        payload.GetQuoteMint(),
		LPMint:           payload.GetLpMint(),
		BaseAmountIn:     payload.GetBaseAmountIn(),
		QuoteAmountIn:    payload.GetQuoteAmountIn(),
		InitialLiquidity: payload.GetInitialLiquidity(),
		CoinCreator:      payload.GetCoinCreator(),
		IsMayhemMode:     payload.GetIsMayhemMode(),
		InstructionArgs: PumpAmmCreatePoolInstructionArgs{
			Index:          uint16(optionalPumpAmmCreatePoolArgs(args, func(v *serviceeventpb.PumpAmmCreatePoolInstructionArgs) uint32 { return v.GetIndex() })),
			CoinCreator:    optionalPumpAmmCreatePoolArgs(args, func(v *serviceeventpb.PumpAmmCreatePoolInstructionArgs) string { return v.GetCoinCreator() }),
			IsMayhemMode:   optionalPumpAmmCreatePoolArgs(args, func(v *serviceeventpb.PumpAmmCreatePoolInstructionArgs) bool { return v.GetIsMayhemMode() }),
			IsCashbackCoin: optionalPumpAmmCreatePoolBoolPtr(args, func(v *serviceeventpb.PumpAmmCreatePoolInstructionArgs) *bool { return v.IsCashbackCoin }),
		},
	}
}

func pumpAmmLiquidityPayloadFromProto(payload *serviceeventpb.PumpAmmLiquidityPayload) PumpAmmLiquidityPayload {
	if payload == nil {
		return PumpAmmLiquidityPayload{}
	}

	args := payload.GetInstructionArgs()

	return PumpAmmLiquidityPayload{
		Action:           payload.GetAction(),
		Pool:             payload.GetPool(),
		User:             payload.GetUser(),
		BaseMint:         payload.GetBaseMint(),
		QuoteMint:        payload.GetQuoteMint(),
		LPMint:           payload.GetLpMint(),
		LPTokenAmountIn:  payload.LpTokenAmountIn,
		LPTokenAmountOut: payload.LpTokenAmountOut,
		BaseAmountIn:     payload.BaseAmountIn,
		QuoteAmountIn:    payload.QuoteAmountIn,
		BaseAmountOut:    payload.BaseAmountOut,
		QuoteAmountOut:   payload.QuoteAmountOut,
		LPMintSupply:     payload.GetLpMintSupply(),
		InstructionArgs: PumpAmmLiquidityInstructionArgs{
			LPTokenAmountIn:   optionalStringFromPumpAmmLiquidityArgs(args, func(v *serviceeventpb.PumpAmmLiquidityInstructionArgs) *string { return v.LpTokenAmountIn }),
			LPTokenAmountOut:  optionalStringFromPumpAmmLiquidityArgs(args, func(v *serviceeventpb.PumpAmmLiquidityInstructionArgs) *string { return v.LpTokenAmountOut }),
			MaxBaseAmountIn:   optionalStringFromPumpAmmLiquidityArgs(args, func(v *serviceeventpb.PumpAmmLiquidityInstructionArgs) *string { return v.MaxBaseAmountIn }),
			MaxQuoteAmountIn:  optionalStringFromPumpAmmLiquidityArgs(args, func(v *serviceeventpb.PumpAmmLiquidityInstructionArgs) *string { return v.MaxQuoteAmountIn }),
			MinBaseAmountOut:  optionalStringFromPumpAmmLiquidityArgs(args, func(v *serviceeventpb.PumpAmmLiquidityInstructionArgs) *string { return v.MinBaseAmountOut }),
			MinQuoteAmountOut: optionalStringFromPumpAmmLiquidityArgs(args, func(v *serviceeventpb.PumpAmmLiquidityInstructionArgs) *string { return v.MinQuoteAmountOut }),
		},
	}
}

func optionalStringFromPumpfunTradeArgs(
	args *serviceeventpb.PumpfunTradeInstructionArgs,
	getter func(*serviceeventpb.PumpfunTradeInstructionArgs) *string,
) *string {
	if args == nil {
		return nil
	}
	return getter(args)
}

func optionalStringFromPumpAmmSwapArgs(
	args *serviceeventpb.PumpAmmSwapInstructionArgs,
	getter func(*serviceeventpb.PumpAmmSwapInstructionArgs) *string,
) *string {
	if args == nil {
		return nil
	}
	return getter(args)
}

func optionalPumpAmmCreatePoolArgs[T any](
	args *serviceeventpb.PumpAmmCreatePoolInstructionArgs,
	getter func(*serviceeventpb.PumpAmmCreatePoolInstructionArgs) T,
) T {
	var zero T
	if args == nil {
		return zero
	}
	return getter(args)
}

func optionalPumpAmmCreatePoolBoolPtr(
	args *serviceeventpb.PumpAmmCreatePoolInstructionArgs,
	getter func(*serviceeventpb.PumpAmmCreatePoolInstructionArgs) *bool,
) *bool {
	if args == nil {
		return nil
	}
	return getter(args)
}

func optionalStringFromPumpAmmLiquidityArgs(
	args *serviceeventpb.PumpAmmLiquidityInstructionArgs,
	getter func(*serviceeventpb.PumpAmmLiquidityInstructionArgs) *string,
) *string {
	if args == nil {
		return nil
	}
	return getter(args)
}

func instructionPathFromProto(path *serviceeventpb.InstructionPath) (InstructionPath, error) {
	if path == nil {
		return InstructionPath{}, fmt.Errorf("instruction_path is required")
	}

	var innerIndex *int
	if path.InnerIndex != nil {
		value := int(*path.InnerIndex)
		innerIndex = &value
	}

	return InstructionPath{
		Source:     instructionSourceFromProto(path.GetSource()),
		OuterIndex: int(path.GetOuterIndex()),
		InnerIndex: innerIndex,
	}, nil
}

func refsFromProto(refs *serviceeventpb.EventRefs) EventRefs {
	if refs == nil {
		return EventRefs{}
	}

	return EventRefs{
		Mint:         refs.Mint,
		Pool:         refs.Pool,
		BondingCurve: refs.BondingCurve,
		User:         refs.User,
		Creator:      refs.Creator,
		BaseMint:     refs.BaseMint,
		QuoteMint:    refs.QuoteMint,
		LPMint:       refs.LpMint,
	}
}

func chainFromProto(chain serviceeventpb.Chain) string {
	switch chain {
	case serviceeventpb.Chain_CHAIN_SOLANA:
		return "solana"
	default:
		return ""
	}
}

func protocolFromProto(protocol serviceeventpb.Protocol) string {
	switch protocol {
	case serviceeventpb.Protocol_PROTOCOL_PUMPFUN:
		return "pumpfun"
	case serviceeventpb.Protocol_PROTOCOL_PUMPAMM:
		return "pumpamm"
	default:
		return ""
	}
}

func eventTypeFromProto(eventType serviceeventpb.EventType) string {
	switch eventType {
	case serviceeventpb.EventType_EVENT_TYPE_TRADE:
		return "trade"
	case serviceeventpb.EventType_EVENT_TYPE_CREATE:
		return "create"
	case serviceeventpb.EventType_EVENT_TYPE_MIGRATE:
		return "migrate"
	case serviceeventpb.EventType_EVENT_TYPE_SWAP:
		return "swap"
	case serviceeventpb.EventType_EVENT_TYPE_CREATE_POOL:
		return "create_pool"
	case serviceeventpb.EventType_EVENT_TYPE_DEPOSIT:
		return "deposit"
	case serviceeventpb.EventType_EVENT_TYPE_WITHDRAW:
		return "withdraw"
	default:
		return ""
	}
}

func commitmentFromProto(commitment serviceeventpb.CommitmentLevel) string {
	switch commitment {
	case serviceeventpb.CommitmentLevel_COMMITMENT_LEVEL_PROCESSED:
		return "processed"
	case serviceeventpb.CommitmentLevel_COMMITMENT_LEVEL_CONFIRMED:
		return "confirmed"
	case serviceeventpb.CommitmentLevel_COMMITMENT_LEVEL_FINALIZED:
		return "finalized"
	default:
		return ""
	}
}

func instructionSourceFromProto(source serviceeventpb.InstructionSource) string {
	switch source {
	case serviceeventpb.InstructionSource_INSTRUCTION_SOURCE_OUTER:
		return "outer"
	case serviceeventpb.InstructionSource_INSTRUCTION_SOURCE_INNER:
		return "inner"
	default:
		return ""
	}
}

func eventOriginFromProto(origin serviceeventpb.EventOrigin) string {
	switch origin {
	case serviceeventpb.EventOrigin_EVENT_ORIGIN_LOGS:
		return "logs"
	case serviceeventpb.EventOrigin_EVENT_ORIGIN_INNER_CPI:
		return "inner_cpi"
	default:
		return ""
	}
}
