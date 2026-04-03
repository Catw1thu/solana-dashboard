package events

import (
	"encoding/json"
	"fmt"

	serviceeventpb "solana-dashboard-go/internal/gen/serviceeventpb"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var payloadMarshalOptions = protojson.MarshalOptions{
	UseProtoNames:   true,
	EmitUnpopulated: false,
}

func EnvelopeFromProto(event *serviceeventpb.EventEnvelope) (Envelope, error) {
	if event == nil {
		return Envelope{}, fmt.Errorf("proto event is nil")
	}

	payloadJSON, err := marshalProtoPayload(event)
	if err != nil {
		return Envelope{}, err
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
		Payload:         payloadJSON,
	}, nil
}

func marshalProtoPayload(event *serviceeventpb.EventEnvelope) (json.RawMessage, error) {
	var message proto.Message

	switch payload := event.Payload.(type) {
	case *serviceeventpb.EventEnvelope_PumpfunTrade:
		message = payload.PumpfunTrade
	case *serviceeventpb.EventEnvelope_PumpfunCreate:
		message = payload.PumpfunCreate
	case *serviceeventpb.EventEnvelope_PumpfunMigrate:
		message = payload.PumpfunMigrate
	case *serviceeventpb.EventEnvelope_PumpammSwap:
		message = payload.PumpammSwap
	case *serviceeventpb.EventEnvelope_PumpammCreatePool:
		message = payload.PumpammCreatePool
	case *serviceeventpb.EventEnvelope_PumpammDeposit:
		message = payload.PumpammDeposit
	case *serviceeventpb.EventEnvelope_PumpammWithdraw:
		message = payload.PumpammWithdraw
	default:
		return nil, fmt.Errorf("unsupported proto payload type %T", payload)
	}

	bytes, err := payloadMarshalOptions.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("marshal proto payload to json: %w", err)
	}

	return json.RawMessage(bytes), nil
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
