package workspace

import (
	"net/url"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func ListWorkspaces(config *config.Config) ([]hookdeck.Team, error) {
	parsedBaseURL, err := url.Parse(config.APIBaseURL)
	if err != nil {
		return nil, err
	}

	client := &hookdeck.Client{
		BaseURL: parsedBaseURL,
		APIKey:  config.Profile.APIKey,
	}

	return client.ListTeams()
}