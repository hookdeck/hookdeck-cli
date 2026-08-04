package project

import (
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func ListProjects(config *config.Config) ([]hookdeck.Project, error) {
	client := config.GetAPIClient()
	return client.ListProjects()
}
