package events

import (
	"encoding/json"
	"fmt"
)

func DecodePayload(env Envelope) (any, error) {
	switch {
	case env.Protocol == "pumpfun" && env.EventType == "trade":
		var payload PumpfunTradePayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case env.Protocol == "pumpamm" && env.EventType == "swap":
		var payload PumpAmmSwapPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported event type: %s.%s", env.Protocol, env.EventType)
	}

}
