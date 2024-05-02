package project

import (
	"net/url"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func ListProjects(config *config.Config) ([]hookdeck.Project, error) {
	parsedBaseURL, err := url.Parse(config.APIBaseURL)
	if err != nil {
		return nil, err
	}

	client := &hookdeck.Client{
		BaseURL: parsedBaseURL,
		APIKey:  config.Profile.APIKey,
	}

	return client.ListProjects()
}
