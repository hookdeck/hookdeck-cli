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
	// 401 only. A 403 from these routes means the credential was accepted and
	// the operation is not permitted for it — "API keys cannot create
	// organization API keys", for instance, which needs an admin session rather
	// than a different key. The API's own message is precise there, and adding
	// "this is the wrong kind of credential" on top of it contradicts it.
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized {
		// newActionableError, not a bare wrap: Execute rewrites any 401 into
		// the generic "your API key is invalid or expired" unless the error
		// says it carries its own guidance. Without this the hint below is
		// built and then thrown away, which is what happened when it was first
		// added — the message a user saw was the generic one.
		return newActionableError(fmt.Errorf("%w\n\n"+
			"This is most likely the wrong kind of credential rather than a bad one.\n"+
			"The organization commands need an organization API key; a CLI session from\n"+
			"`hookdeck login` can list projects and work inside one, but cannot read the\n"+
			"organization.\n\n"+
			"Create an organization API key in the Hookdeck dashboard, then pass it with\n"+
			"  hookdeck org --api-key <key> ...\n"+
			"or set HOOKDECK_API_KEY. Note that `hookdeck ci --api-key` will not take one:\n"+
			"that path wants a project key.", err))
	}
	return err
}
