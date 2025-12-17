package login

import (
	"net/url"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// ValidateKey validates an API key and returns user/project information
func ValidateKey(baseURL string, key string, projectId string) (*hookdeck.ValidateAPIKeyResponse, error) {
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	client := &hookdeck.Client{
		BaseURL:   parsedBaseURL,
		APIKey:    key,
		ProjectID: projectId,
	}

	return client.ValidateAPIKey()
}
