package project

import (
	"errors"
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// ErrCIScopedCredentials is returned when the stored CLI key is valid but not
// associated with a user (for example after hookdeck ci).
var ErrCIScopedCredentials = errors.New(
	"listing projects requires a user-associated CLI key; keys from hookdeck ci are scoped to a single project; " +
		"run hookdeck login in an interactive terminal, hookdeck login --cli-key with a product CLI key, or hookdeck_login via MCP for full account access",
)

// ValidateCredentials calls GET /cli-auth/validate without sending X-Team-ID.
func ValidateCredentials(config *config.Config) (*hookdeck.ValidateAPIKeyResponse, error) {
	return config.GetAPIClient().ValidateAPIKey()
}

// EnsureUserAssociatedCredentials rejects CI-scoped keys before cross-project calls.
func EnsureUserAssociatedCredentials(config *config.Config) error {
	response, err := ValidateCredentials(config)
	if err != nil {
		return err
	}
	if response.UserID == "" {
		return ErrCIScopedCredentials
	}
	return nil
}

// EnsureUserAssociatedClient rejects CI-scoped keys for an in-memory API client (MCP).
func EnsureUserAssociatedClient(client *hookdeck.Client) error {
	if client == nil || client.APIKey == "" {
		return fmt.Errorf("not authenticated")
	}
	response, err := client.ValidateAPIKey()
	if err != nil {
		return err
	}
	if response.UserID == "" {
		return ErrCIScopedCredentials
	}
	return nil
}

// CredentialsLackUserAssociation reports whether validate succeeded without a user_id.
func CredentialsLackUserAssociation(client *hookdeck.Client) (bool, error) {
	if client == nil || client.APIKey == "" {
		return false, nil
	}
	response, err := client.ValidateAPIKey()
	if err != nil {
		return false, err
	}
	return response.UserID == "", nil
}
