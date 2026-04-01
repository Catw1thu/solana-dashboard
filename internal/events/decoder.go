package events

import (
	"encoding/json"
	"fmt"
)

func DecodePayload(event Envelope) (any, error) {
	switch {
	case event.Protocol == "pumpfun" && event.EventType == "trade":
		var payload PumpfunTradePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case event.Protocol == "pumpfun" && event.EventType == "create":
		var payload PumpfunCreatePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case event.Protocol == "pumpfun" && event.EventType == "migrate":
		var payload PumpfunMigratePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case event.Protocol == "pumpamm" && event.EventType == "swap":
		var payload PumpAmmSwapPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case event.Protocol == "pumpamm" && event.EventType == "create_pool":
		var payload PumpAmmCreatePoolPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case event.Protocol == "pumpamm" && event.EventType == "deposit":
		var payload PumpAmmLiquidityPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case event.Protocol == "pumpamm" && event.EventType == "withdraw":
		var payload PumpAmmLiquidityPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported event type: %s.%s", event.Protocol, event.EventType)
	}
}
