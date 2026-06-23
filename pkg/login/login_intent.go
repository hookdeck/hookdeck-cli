package login

import (
	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func BuildStartLoginInput(
	config *configpkg.Config,
	saved_guest_api_key string,
) hookdeck.StartLoginInput {
	return buildStartLoginInput(config, saved_guest_api_key)
}

func buildStartLoginInput(
	config *configpkg.Config,
	saved_guest_api_key string,
) hookdeck.StartLoginInput {
	input := hookdeck.StartLoginInput{
		DeviceName: config.DeviceName,
	}

	if config != nil && config.Profile.GuestURL != "" {
		input.GuestUserID = guestCredentialsUserID(config)
		input.GuestAPIKey = guestCredentialsAPIKey(config, saved_guest_api_key)
	}

	return input
}
