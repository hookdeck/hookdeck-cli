package config

import (
	"net/url"
	"sync"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

var apiClient *hookdeck.Client
var apiClientOnce sync.Once

// GetAPIClient returns the internal API client instance
func (c *Config) GetAPIClient() *hookdeck.Client {
	apiClientOnce.Do(func() {
		baseURL, err := url.Parse(c.APIBaseURL)
		if err != nil {
			panic("Invalid API base URL: " + err.Error())
		}

		apiClient = &hookdeck.Client{
			BaseURL:   baseURL,
			APIKey:    c.Profile.APIKey,
			ProjectID: c.Profile.ProjectId,
			Verbose:   c.LogLevel == "debug",
		}
	})

	return apiClient
}
