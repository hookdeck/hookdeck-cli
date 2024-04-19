package tui

import (
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	api "github.com/hookdeck/hookdeck-go-sdk"
)

func ConnectionFull(config *config.Config, connection *api.Connection) {
	fmt.Printf("%s %s\n", connection.Id, *connection.FullName)
}
