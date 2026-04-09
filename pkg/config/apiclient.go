package config

import (
	"net/url"
	"sync"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

var apiClient *hookdeck.Client
var apiClientOnce sync.Once

func resetAPIClient() {
	apiClient = nil
	apiClientOnce = sync.Once{}
}

// ResetAPIClientForTesting resets the global API client singleton so that
// tests can start with a fresh instance. Must only be called from tests.
func ResetAPIClientForTesting() {
	resetAPIClient()
}

// RefreshCachedAPIClient copies the current config (API base, profile key and
// project id, log/telemetry flags) onto the cached *hookdeck.Client if one
// already exists. Use after login or other in-process profile updates so the
// singleton matches Profile without discarding the underlying http.Client.
// If GetAPIClient has never been called, this is a no-op (the next GetAPIClient
// will construct from Config).
func (c *Config) RefreshCachedAPIClient() {
	if apiClient == nil {
		return
	}
	baseURL, err := url.Parse(c.APIBaseURL)
	if err != nil {
		panic("Invalid API base URL: " + err.Error())
	}
	apiClient.BaseURL = baseURL
	apiClient.APIKey = c.Profile.APIKey
	apiClient.ProjectID = c.Profile.ProjectId
	apiClient.Verbose = c.LogLevel == "debug"
	apiClient.TelemetryDisabled = c.TelemetryDisabled
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
