package tui

import (
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	api "github.com/hookdeck/hookdeck-go-sdk"
)

func DestinationFull(config *config.Config, destination *api.Destination) {
	fmt.Printf("%s %s\n", destination.Id, destination.Name)
}
