package mcp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

const listProjectsReauthHint = `This may happen if the stored key is a dashboard or single-project API key that cannot list all teams/projects. Try hookdeck_login with reauth: true so the user can sign in via the browser and replace the credential with a full CLI session, then retry hookdeck_projects.`

func listProjectsFailureMessage(err error) string {
	base := TranslateAPIError(err)
	if shouldSuggestReauthAfterListProjectsFailure(err) {
		return base + "\n\n" + listProjectsReauthHint
	}
	return base
}

func shouldSuggestReauthAfterListProjectsFailure(err error) bool {
	var apiErr *hookdeck.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusUnauthorized {
			return true
		}
		return strings.Contains(strings.ToLower(apiErr.Message), "fatal")
	}
	// ListProjects wraps some failures as plain fmt.Errorf with the status code
	// in the text (e.g. "unexpected http status code: 403 <nil>").
	// Match "status code: 4xx" to avoid false positives on IDs containing "401"/"403".
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "status code: 403") || strings.Contains(msg, "status code: 401") {
		return true
	}
	return strings.Contains(msg, "fatal")
}
