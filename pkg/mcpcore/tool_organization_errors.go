package mcpcore

import (
	"errors"
	"net/http"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// organizationAuthHint explains a 401 from the organization routes.
//
// /organizations/current needs an ORGANIZATION API key. A CLI session — from
// `hookdeck login` — can list projects and do everything inside one, and still
// gets a bare 401 here, because the credential is the wrong kind rather than
// invalid. The API's own message is "Authentication failed. Check your API
// key.", which sends the reader to check a key that is working perfectly.
//
// Diagnosing that took two rounds of probing the API by hand. Nobody else
// should have to.
const organizationAuthHint = "This is most likely the wrong kind of credential rather than a bad one. " +
	"The organization routes need an organization API key; a CLI session from `hookdeck login` " +
	"can list projects and work inside one, but cannot read the organization. " +
	"Create an organization API key in the Hookdeck dashboard, then use it with " +
	"`hookdeck ci --api-key` or HOOKDECK_API_KEY."

// organizationFailureMessage renders an organization error, adding the hint
// when the status says the credential was refused.
func organizationFailureMessage(err error) string {
	base := TranslateAPIError(err)

	var apiErr *hookdeck.APIError
	if errors.As(err, &apiErr) &&
		(apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
		return base + "\n\n" + organizationAuthHint
	}
	return base
}
