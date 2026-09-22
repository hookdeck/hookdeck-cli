package cmd

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// orgAuthError explains a refusal from the organization routes.
//
// They need an ORGANIZATION API key. A CLI session from `hookdeck login` can
// list projects and work inside one, and still gets a bare 401 here, because
// the credential is the wrong kind rather than invalid. The API says
// "Authentication failed. Check your API key.", which sends the reader to check
// a key that is working perfectly.
func orgAuthError(err error) error {
	var apiErr *hookdeck.APIError
	if errors.As(err, &apiErr) &&
		(apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
		return fmt.Errorf("%w\n\n"+
			"This is most likely the wrong kind of credential rather than a bad one.\n"+
			"The organization commands need an organization API key; a CLI session from\n"+
			"`hookdeck login` can list projects and work inside one, but cannot read the\n"+
			"organization.\n\n"+
			"Create an organization API key in the Hookdeck dashboard, then run\n"+
			"  hookdeck ci --api-key <key>\n"+
			"or set HOOKDECK_API_KEY.", err)
	}
	return err
}
