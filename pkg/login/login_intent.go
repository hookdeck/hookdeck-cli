package login

import (
	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func resolveLoginIntent(config *configpkg.Config) hookdeck.CLIAuthIntent {
	has_guest_profile := config != nil && config.Profile.GuestURL != ""
	if !has_guest_profile {
		return hookdeck.CLIAuthIntentLogin
	}
	return hookdeck.CLIAuthIntentClaimGuest
}

func ResolveLoginIntent(config *configpkg.Config) hookdeck.CLIAuthIntent {
	return resolveLoginIntent(config)
}

func BuildStartLoginInput(
	config *configpkg.Config,
	intent hookdeck.CLIAuthIntent,
	saved_guest_api_key string,
) hookdeck.StartLoginInput {
	return buildStartLoginInput(config, intent, saved_guest_api_key)
}

func buildStartLoginInput(
	config *configpkg.Config,
	intent hookdeck.CLIAuthIntent,
	saved_guest_api_key string,
) hookdeck.StartLoginInput {
	input := hookdeck.StartLoginInput{
		DeviceName: config.DeviceName,
		AuthIntent: intent,
	}

	if intent == hookdeck.CLIAuthIntentClaimGuest {
		input.GuestUserID = guestCredentialsUserID(config)
		input.GuestAPIKey = guestCredentialsAPIKey(config, saved_guest_api_key)
	}

	return input
}
