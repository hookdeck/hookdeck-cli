package tui

import (
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	api "github.com/hookdeck/hookdeck-go-sdk"
)

func SourceFull(config *config.Config, source *api.Source) {
	fmt.Printf("%s %s\n", source.Id, source.Name)
}
