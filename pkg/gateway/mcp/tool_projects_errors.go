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
	// ListProjects wraps some failures as plain fmt.Errorf with status in the text.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "403") || strings.Contains(msg, "401") {
		return true
	}
	return strings.Contains(msg, "fatal")
}
