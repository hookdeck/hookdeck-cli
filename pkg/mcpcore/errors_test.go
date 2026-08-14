package mcpcore

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func TestTranslateAPIError_403Forbidden(t *testing.T) {
	t.Run("keeps the API explanation", func(t *testing.T) {
		msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 403, Message: "missing scope tenants:write"})
		assert.Contains(t, msg, "Not permitted")
		assert.Contains(t, msg, "missing scope tenants:write")
		assert.NotContains(t, msg, "Check your API key")
	})

	t.Run("falls back when the API gives no message", func(t *testing.T) {
		msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 403})
		assert.Contains(t, msg, "Not permitted")
	})
}

func TestTranslateAPIError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantSubstr string
	}{
		{"401 Unauthorized", &hookdeck.APIError{StatusCode: 401, Message: "bad key"}, "Authentication failed"},
		{"404 Not Found", &hookdeck.APIError{StatusCode: 404, Message: "resource xyz"}, "Resource not found"},
		{"410 Gone", &hookdeck.APIError{StatusCode: 410, Message: "resource xyz"}, "Resource not found"},
		{"422 Validation", &hookdeck.APIError{StatusCode: 422, Message: "invalid field foo"}, "invalid field foo"},
		{"429 Rate Limit", &hookdeck.APIError{StatusCode: 429, Message: "slow down"}, "Rate limited"},
		{"500 Server Error", &hookdeck.APIError{StatusCode: 500, Message: "internal"}, "Hookdeck API error"},
		{"Non-API error", fmt.Errorf("network timeout"), "network timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := TranslateAPIError(tt.err)
			assert.Contains(t, msg, tt.wantSubstr)
		})
	}
}

// ---------------------------------------------------------------------------
// Sources tool
// ---------------------------------------------------------------------------

func TestTranslateAPIError_RetryAfterMessage(t *testing.T) {
	msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 429, Message: "rate limited"})
	assert.Contains(t, msg, "Rate limited")
	assert.Contains(t, msg, "Retry after")
}

func TestTranslateAPIError_GenericClientError(t *testing.T) {
	// A 4xx status not explicitly handled should pass through the message
	msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 409, Message: "conflict on resource"})
	assert.Contains(t, msg, "conflict on resource")
}

func TestTranslateAPIError_502GatewayError(t *testing.T) {
	msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 502, Message: "bad gateway"})
	assert.Contains(t, msg, "Hookdeck API error")
}

func TestTranslateAPIError_503ServiceUnavailable(t *testing.T) {
	msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 503, Message: "service unavailable"})
	assert.Contains(t, msg, "Hookdeck API error")
}
