package config

import (
	"net/url"
	"sync"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

var apiClient *hookdeck.Client
var apiClientOnce sync.Once

// The Outpost API lives on its own host, so it needs its own client instance.
// It is kept separate rather than derived on demand because MCP tool handlers
// mutate the client in place (e.g. ProjectID on a project switch), and those
// mutations must not leak between the two products.
var outpostAPIClient *hookdeck.Client
var outpostAPIClientOnce sync.Once

func resetAPIClient() {
	apiClient = nil
	apiClientOnce = sync.Once{}
	outpostAPIClient = nil
	outpostAPIClientOnce = sync.Once{}
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
	if apiClient != nil {
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

	// The Outpost client shares the profile's credentials and project, so it
	// has to be refreshed too. Skipping it would leave it holding the key from
	// before a login or project switch.
	if outpostAPIClient != nil {
		outpostBaseURL, err := url.Parse(c.OutpostAPIBaseURL)
		if err != nil {
			panic("Invalid Outpost API base URL: " + err.Error())
		}
		outpostAPIClient.BaseURL = outpostBaseURL
		outpostAPIClient.APIKey = c.Profile.APIKey
		outpostAPIClient.ProjectID = c.Profile.ProjectId
		outpostAPIClient.Verbose = c.LogLevel == "debug"
		outpostAPIClient.TelemetryDisabled = c.TelemetryDisabled
	}
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

// GetOutpostAPIClient returns the API client instance for the Hookdeck Outpost
// API. It is the same client type as GetAPIClient, pointed at the Outpost host:
// authentication, project scoping and telemetry all behave identically.
func (c *Config) GetOutpostAPIClient() *hookdeck.Client {
	outpostAPIClientOnce.Do(func() {
		baseURL, err := url.Parse(c.OutpostAPIBaseURL)
		if err != nil {
			panic("Invalid Outpost API base URL: " + err.Error())
		}

		outpostAPIClient = &hookdeck.Client{
			BaseURL:           baseURL,
			APIKey:            c.Profile.APIKey,
			ProjectID:         c.Profile.ProjectId,
			Verbose:           c.LogLevel == "debug",
			TelemetryDisabled: c.TelemetryDisabled,
			// Outpost answers 201 on create and 202 on publish/retry, so
			// restricting success to 200 would fail every write.
			AcceptAnySuccessStatus: true,
		}
	})

	return outpostAPIClient
}
