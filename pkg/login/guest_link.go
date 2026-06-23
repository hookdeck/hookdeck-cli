package login

import (
	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	log "github.com/sirupsen/logrus"
)

// RefreshGuestSigninLink mints a fresh guest sign-in URL for the active guest profile.
// On failure, returns the existing guest URL and logs a warning.
func RefreshGuestSigninLink(config *configpkg.Config) string {
	if config == nil || config.Profile.GuestURL == "" || config.Profile.APIKey == "" {
		return ""
	}

	response, err := config.GetAPIClient().RefreshGuestSigninLink()
	if err != nil {
		log.WithError(err).Warn("Failed to refresh guest sign-in link; using saved URL")
		return config.Profile.GuestURL
	}
	if response.Url == "" {
		log.Warn("Guest sign-in link refresh returned empty link; using saved URL")
		return config.Profile.GuestURL
	}

	config.Profile.GuestURL = response.Url
	// Id is the guest user id (core POST /cli/guest/signin-link returns req.context.user.id).
	if response.Id != "" {
		config.Profile.GuestUserID = response.Id
	}

	if err := config.Profile.SaveProfile(); err != nil {
		log.WithError(err).Warn("Refreshed guest sign-in link but failed to save profile")
	}

	return config.Profile.GuestURL
}
