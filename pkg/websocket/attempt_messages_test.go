package websocket

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// request_id is optional on the wire: servers that do not send it must still
// decode, and listen links fall back to the Source page without it.
func TestAttemptBodyRequestID(t *testing.T) {
	t.Run("decoded when the server sends it", func(t *testing.T) {
		var attempt Attempt
		require.NoError(t, json.Unmarshal([]byte(`{"type":"attempt","body":{"event_id":"evt_1","attempt_id":"atm_1","webhook_id":"web_1","request_id":"req_1","request":{"method":"POST"}}}`), &attempt))
		assert.Equal(t, "req_1", attempt.Body.RequestID)
		assert.Equal(t, "web_1", attempt.Body.ConnectionId)
	})

	t.Run("empty when the server does not send it", func(t *testing.T) {
		var attempt Attempt
		require.NoError(t, json.Unmarshal([]byte(`{"type":"attempt","body":{"event_id":"evt_1","attempt_id":"atm_1","webhook_id":"web_1","request":{"method":"POST"}}}`), &attempt))
		assert.Equal(t, "", attempt.Body.RequestID)
	})
}
