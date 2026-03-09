package mcp

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TranslateAPIError converts a Hookdeck API error into a human-readable
// message suitable for returning to an MCP client. If the error is not an
// *hookdeck.APIError, the original error message is returned unchanged.
func TranslateAPIError(err error) string {
	var apiErr *hookdeck.APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}

	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		return "Authentication failed. Check your API key."
	case http.StatusNotFound:
		return fmt.Sprintf("Resource not found: %s", apiErr.Message)
	case http.StatusUnprocessableEntity:
		// Validation errors — pass through the API message directly.
		return apiErr.Message
	case http.StatusTooManyRequests:
		return "Rate limited. Retry after a brief pause."
	default:
		if apiErr.StatusCode >= 500 {
			return fmt.Sprintf("Hookdeck API error: %s", apiErr.Message)
		}
		return apiErr.Message
	}
}
