package websocket

import (
	"encoding/json"
	"fmt"
)

// IncomingMessage represents any incoming message sent by Hookdeck.
type IncomingMessage struct {
	*Attempt
	// *RequestLogEvent
}

// UnmarshalJSON deserializes incoming messages sent by Hookdeck into the
// appropriate structure.
func (m *IncomingMessage) UnmarshalJSON(data []byte) error {
	incomingMessageEventOnly := struct {
		Event string `json:"event"`
	}{}

	if err := json.Unmarshal(data, &incomingMessageEventOnly); err != nil {
		return err
	}

	switch incomingMessageEventOnly.Event {
	case "attempt":
		var evt Attempt
		if err := json.Unmarshal(data, &evt); err != nil {
			return err
		}

		m.Attempt = &evt
	case "connect_response":
		return nil
	default:
		return fmt.Errorf("Unexpected message type: %s", incomingMessageEventOnly.Event)
	}

	return nil
}

// MarshalJSON serializes outgoing messages sent to Hookdeck.
func (m OutgoingMessage) MarshalJSON() ([]byte, error) {
	if m.AttemptResponse != nil {
		return json.Marshal(m.AttemptResponse)
	}

	if m.ErrorAttemptResponse != nil {
		return json.Marshal(m.ErrorAttemptResponse)
	}

	if m.ConnectionMessage != nil {
		return json.Marshal(m.ConnectionMessage)
	}

	return json.Marshal(nil)
}

// OutgoingMessage represents any outgoing message sent to Hookdeck.
type OutgoingMessage struct {
	*ErrorAttemptResponse
	*AttemptResponse
	*ConnectionMessage
}
