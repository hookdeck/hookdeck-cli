package config

import (
	"net/url"
	"sync"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

var apiClient *hookdeck.Client
var apiClientOnce sync.Once

// ResetAPIClient clears the cached API client singleton. The next GetAPIClient
// call builds a fresh client from the current config (used after login updates credentials).
func ResetAPIClient() {
	apiClient = nil
	apiClientOnce = sync.Once{}
}

// ResetAPIClientForTesting resets the global API client singleton so that
// tests can start with a fresh instance. Must only be called from tests.
func ResetAPIClientForTesting() {
	ResetAPIClient()
}

// GetAPIClient returns the internal API client instance
func (c *Config) GetAPIClient() *hookdeck.Client {
	apiClientOnce.Do(func() {
		baseURL, err := url.Parse(c.APIBaseURL)
		if err != nil {
			panic("Invalid API base URL: " + err.Error())
		}

		apiClient = &hookdeck.Client{
			BaseURL:           baseURL,
			APIKey:            c.Profile.APIKey,
			ProjectID:         c.Profile.ProjectId,
			Verbose:           c.LogLevel == "debug",
			TelemetryDisabled: c.TelemetryDisabled,
		}
	})

	return apiClient
}
