package events

import "encoding/json"

type Envelope struct {
	LogID           int64           `json:"log_id,omitempty"`
	SchemaVersion   int             `json:"schema_version"`
	EventID         string          `json:"event_id"`
	Chain           string          `json:"chain"`
	Protocol        string          `json:"protocol"`
	EventType       string          `json:"event_type"`
	Commitment      string          `json:"commitment"`
	Slot            uint64          `json:"slot"`
	TxSignature     string          `json:"tx_signature"`
	TxIndex         uint64          `json:"tx_index"`
	InstructionPath InstructionPath `json:"instruction_path"`
	EventSource     string          `json:"event_source"`
	EventUnixTS     int64           `json:"event_unix_ts"`
	Refs            EventRefs       `json:"refs"`
	Payload         json.RawMessage `json:"payload"`
}

type DecodedEnvelope struct {
	Envelope Envelope
	Payload  any
}

func (d DecodedEnvelope) EnvelopeWithPayload() (Envelope, error) {
	envelope := d.Envelope
	if len(envelope.Payload) > 0 {
		return envelope, nil
	}

	if d.Payload == nil {
		return envelope, nil
	}

	payloadJSON, err := json.Marshal(d.Payload)
	if err != nil {
		return Envelope{}, err
	}
	envelope.Payload = payloadJSON

	return envelope, nil
}

type InstructionPath struct {
	Source     string `json:"source"`
	OuterIndex int    `json:"outer_index"`
	InnerIndex *int   `json:"inner_index"`
}

type EventRefs struct {
	Mint         *string `json:"mint"`
	Pool         *string `json:"pool"`
	BondingCurve *string `json:"bonding_curve"`
	User         *string `json:"user"`
	Creator      *string `json:"creator"`
	BaseMint     *string `json:"base_mint"`
	QuoteMint    *string `json:"quote_mint"`
	LPMint       *string `json:"lp_mint"`
}
