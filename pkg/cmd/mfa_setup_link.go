package cmd

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// withMFASetupLink returns the error's message, followed by the dashboard link
// to set up MFA when the API refused the request because the organization
// requires it.
func withMFASetupLink(err error, dashboardBaseURL string) string {
	message := err.Error()
	if link := mfaSetupLink(err, dashboardBaseURL); link != "" {
		message += "\nSet it up here: " + link
	}
	return message
}

// mfaSetupLink returns the link to set up MFA, or "" when err isn't the API's
// MFA refusal. The API sends a dashboard path such as
// "/setup-mfa?organization_id=org_123". Only that path is used, joined to the
// CLI's own dashboard address, so an error response can't point the user to
// another site.
func mfaSetupLink(err error, dashboardBaseURL string) string {
	var apiErr *hookdeck.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		return ""
	}
	if !strings.HasPrefix(apiErr.Redirect, "/setup-mfa") {
		return ""
	}

	base, parseErr := url.Parse(dashboardBaseURL)
	if parseErr != nil || base.Scheme == "" || base.Host == "" {
		return ""
	}
	ref, parseErr := url.Parse(apiErr.Redirect)
	if parseErr != nil || ref.IsAbs() || ref.Host != "" {
		return ""
	}

	resolved := base.ResolveReference(ref)
	if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
		return ""
	}
	return resolved.String()
}
