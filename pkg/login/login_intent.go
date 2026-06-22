package login

import (
	"io"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// Options configures hookdeck login when guest credentials or intent selection applies.
type Options struct {
	ClaimGuest bool
}

func resolveLoginIntent(config *configpkg.Config, input io.Reader, opts Options) (hookdeck.CLIAuthIntent, error) {
	if opts.ClaimGuest {
		return hookdeck.CLIAuthIntentClaimGuest, nil
	}

	has_guest_profile := config != nil && config.Profile.GuestURL != ""
	if !has_guest_profile {
		return hookdeck.CLIAuthIntentLogin, nil
	}

	return hookdeck.CLIAuthIntentClaimGuest, nil
}

func ResolveLoginIntent(config *configpkg.Config, input io.Reader, opts Options) (hookdeck.CLIAuthIntent, error) {
	return resolveLoginIntent(config, input, opts)
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
