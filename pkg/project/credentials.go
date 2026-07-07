package project

import (
	"errors"
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// ErrProjectScopedCredentials is returned when the stored CLI key is valid but
// limited to a single project (for example after hookdeck ci).
var ErrProjectScopedCredentials = errors.New(
	"this credential is scoped to a single project and cannot list all projects; " +
		"keys from hookdeck ci are project-scoped; " +
		"run hookdeck login in an interactive terminal, hookdeck login --cli-key with a product CLI key, or hookdeck_login via MCP for account-wide access",
)

// ErrCIScopedCredentials is an alias for ErrProjectScopedCredentials.
var ErrCIScopedCredentials = ErrProjectScopedCredentials

// ValidateCredentials calls GET /cli-auth/validate without sending X-Team-ID.
func ValidateCredentials(config *config.Config) (*hookdeck.ValidateAPIKeyResponse, error) {
	return config.GetAPIClient().ValidateAPIKey()
}

// EnsureUserAssociatedCredentials rejects project-scoped keys before cross-project calls.
func EnsureUserAssociatedCredentials(config *config.Config) error {
	response, err := ValidateCredentials(config)
	if err != nil {
		return err
	}
	if response.UserID == "" {
		return ErrProjectScopedCredentials
	}
	return nil
}

// EnsureUserAssociatedClient rejects project-scoped keys for an in-memory API client (MCP).
func EnsureUserAssociatedClient(client *hookdeck.Client) error {
	if client == nil || client.APIKey == "" {
		return fmt.Errorf("not authenticated")
	}
	response, err := client.ValidateAPIKey()
	if err != nil {
		return err
	}
	if response.UserID == "" {
		return ErrProjectScopedCredentials
	}
	return nil
}

// CredentialsLackUserAssociation reports whether validate succeeded for a project-scoped key
// (no user_id — our proxy for single-project credential scope today).
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
