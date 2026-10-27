package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

const testDashboard = "https://dashboard.hookdeck.com"

func mfaRefusal(redirect string) error {
	return &hookdeck.APIError{
		StatusCode: http.StatusForbidden,
		Message:    "Acme requires multi-factor authentication. Please set up MFA to continue.",
		Redirect:   redirect,
	}
}

func TestWithMFASetupLinkAddsTheDashboardLink(t *testing.T) {
	got := withMFASetupLink(mfaRefusal("/setup-mfa?organization_id=org_1"), testDashboard)

	assert.Equal(t, "error: Acme requires multi-factor authentication. Please set up MFA to continue.\n"+
		"Set it up here: https://dashboard.hookdeck.com/setup-mfa?organization_id=org_1", got)
}

func TestMFASetupLinkFindsAWrappedRefusal(t *testing.T) {
	err := fmt.Errorf("listing sources: %w", mfaRefusal("/setup-mfa?organization_id=org_1"))

	assert.Equal(t, "https://dashboard.hookdeck.com/setup-mfa?organization_id=org_1", mfaSetupLink(err, testDashboard))
}

func TestMFASetupLinkUsesTheCLIsDashboard(t *testing.T) {
	assert.Equal(t, "http://tokens.localhost/setup-mfa?organization_id=org_1",
		mfaSetupLink(mfaRefusal("/setup-mfa?organization_id=org_1"), "http://tokens.localhost"))
}

// The CLI only ever links to its own dashboard, whatever the response says.
func TestMFASetupLinkIgnoresAnythingButASetupPath(t *testing.T) {
	cases := map[string]error{
		"another site":          mfaRefusal("https://evil.example.com/setup-mfa"),
		"another site, no host": mfaRefusal("//evil.example.com/setup-mfa"),
		"another page":          mfaRefusal("/billing"),
		"no redirect":           mfaRefusal(""),
		"not a 403": &hookdeck.APIError{
			StatusCode: http.StatusUnauthorized,
			Redirect:   "/setup-mfa?organization_id=org_1",
		},
		"not an API error": errors.New("boom"),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, mfaSetupLink(err, testDashboard))
			assert.Equal(t, err.Error(), withMFASetupLink(err, testDashboard))
		})
	}
}
