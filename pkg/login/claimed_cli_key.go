package login

import (
	"fmt"
	"os"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// ConfigureFromClaimedCliKey validates a product-issued CLI key (dashboard onboarding, Console
// destination, etc.) and saves the profile. Unlike Login(), this path does not start browser
// device auth or guest sandbox claim—even when the existing profile is a guest Console session.
func ConfigureFromClaimedCliKey(config *configpkg.Config, cli_key string) error {
	cli_key = strings.TrimSpace(cli_key)
	if cli_key == "" {
		return fmt.Errorf("--cli-key is required")
	}
	if err := validators.APIKey(cli_key); err != nil {
		return err
	}

	config.Profile.APIKey = cli_key

	spinner := ansi.StartNewSpinner("Verifying credentials...", os.Stdout)
	response, err := config.GetAPIClient().ValidateAPIKey()
	if err != nil {
		ansi.StopSpinner(spinner, "", os.Stdout)
		return err
	}

	message := SuccessMessage(
		response.UserName,
		response.UserEmail,
		response.OrganizationName,
		response.ProjectName,
		configpkg.IsConsoleProject(response.ProjectType, response.ProjectProduct, response.ProjectMode),
	)
	ansi.StopSpinner(spinner, message, os.Stdout)

	config.Profile.ApplyValidateAPIKeyResponse(response, true)

	if err := config.Profile.SaveProfile(); err != nil {
		return err
	}
	if err := config.Profile.UseProfile(); err != nil {
		return err
	}
	config.RefreshCachedAPIClient()

	return nil
}
