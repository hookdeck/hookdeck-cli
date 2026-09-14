package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActionableErrorSurvivesUnauthorizedClassification is the regression test
// for the guidance being silently replaced.
//
// Execute rewrites any error IsUnauthorizedError recognises into a generic
// "your API key is invalid or expired" message. A 401 from the HOOKDECK_API_KEY
// exchange is recognised, so the specific advice — that HOOKDECK_API_KEY takes a
// Project API key and a CLI client key belongs in --cli-key — was thrown away
// exactly when it was most useful.
func TestActionableErrorSurvivesUnauthorizedClassification(t *testing.T) {
	apiErr := &hookdeck.APIError{StatusCode: http.StatusUnauthorized, Message: "Unauthorized"}

	wrapped := newActionableError(fmt.Errorf(
		"could not authenticate with HOOKDECK_API_KEY: %w\n\n"+
			"HOOKDECK_API_KEY must be a Project API key from the Hookdeck dashboard "+
			"(Project Settings > API Keys). For a CLI client key, use --cli-key instead.",
		apiErr,
	))

	// The whole reason the type exists: this error IS classified as
	// unauthorized, so ordering in Execute is what protects the message.
	// If this assertion ever fails, the actionableError case can be dropped.
	assert.True(t, hookdeck.IsUnauthorizedError(wrapped),
		"precondition: a wrapped 401 is still recognised as unauthorized")

	var actionable *actionableError
	require.True(t, errors.As(wrapped, &actionable),
		"Execute must be able to detect the error carries its own guidance")

	assert.Contains(t, wrapped.Error(), "Project API key",
		"the specific guidance must survive wrapping")
	assert.Contains(t, wrapped.Error(), "--cli-key")

	// The cause stays inspectable for anything else that needs it.
	var unwrappedAPIErr *hookdeck.APIError
	assert.True(t, errors.As(wrapped, &unwrappedAPIErr),
		"the underlying API error should remain reachable")
	assert.Equal(t, http.StatusUnauthorized, unwrappedAPIErr.StatusCode)
}

// TestActionableErrorDoesNotCaptureOrdinaryErrors guards the other direction:
// only errors explicitly marked as actionable should bypass Execute's generic
// handling, or every 401 would print raw API text instead of recovery steps.
func TestActionableErrorDoesNotCaptureOrdinaryErrors(t *testing.T) {
	plain := fmt.Errorf("some unrelated failure: %w",
		&hookdeck.APIError{StatusCode: http.StatusUnauthorized})

	var actionable *actionableError
	assert.False(t, errors.As(plain, &actionable),
		"an unmarked error must still get Execute's generic recovery message")
}

// TestUnauthorizedServerMessage covers the middle path chosen for #283: when a
// 401 carries an explanation from the API, show that rather than our own guess
// about what went wrong.
//
// The guess is not reliable. A bare 401 cannot distinguish an expired CLI key
// from a project API key from a typo - validators.APIKey only checks length, so
// there is no key shape to inspect - and "invalid or expired" is often untrue:
// a project API key is valid, just not accepted by the CLI auth endpoints.
//
// Today these endpoints answer with a bare "Unauthorized" body, so the fallback
// is what users see. The point of the helper is that it stops being a guess the
// moment the server says anything.
func TestUnauthorizedServerMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "a real explanation is surfaced",
			err:      &hookdeck.APIError{StatusCode: 401, Message: "This credential is scoped to a single project"},
			expected: "This credential is scoped to a single project",
		},
		{
			name: "the bare status word adds nothing",
			// What /cli-auth/validate and /cli-auth/ci actually return today.
			err:      &hookdeck.APIError{StatusCode: 401, Message: "Unauthorized"},
			expected: "",
		},
		{
			name:     "case does not matter",
			err:      &hookdeck.APIError{StatusCode: 401, Message: "  unauthorized  "},
			expected: "",
		},
		{
			name:     "no message at all",
			err:      &hookdeck.APIError{StatusCode: 401},
			expected: "",
		},
		{
			name:     "not an API error",
			err:      errors.New("dial tcp: connection refused"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, unauthorizedServerMessage(tt.err))
		})
	}
}
