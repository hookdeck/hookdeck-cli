package listen

import (
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

func printListenMessage(config *config.Config, isMultiSource bool) {
	if !isMultiSource {
		return
	}

	fmt.Println()
	fmt.Println("Listening for events on Sources that have Connections with CLI Destinations")
}

func printDashboardInformation(config *config.Config, guestURL string) {
	fmt.Println(ansi.Bold("Dashboard"))
	if guestURL != "" {
		fmt.Println("👤 Console URL: " + guestURL)
		fmt.Println("Sign up in the Console to make your webhook URL permanent.")
		fmt.Println()
	} else {
		var url = config.DashboardBaseURL
		if config.Profile.TeamID != "" {
			url += "?team_id=" + config.Profile.TeamID
		}
		if config.Profile.TeamMode == "console" {
			url = config.ConsoleBaseURL
		}
		fmt.Println("👉 Inspect and replay events: " + url)
	}
}

func printSources(config *config.Config, sources []*hookdecksdk.Source) {
	fmt.Println(ansi.Bold("Sources"))

	for _, source := range sources {
		fmt.Printf("🔌 %s URL: %s\n", source.Name, source.Url)
	}
}

func printConnections(config *config.Config, connections []*hookdecksdk.Connection) {
	fmt.Println(ansi.Bold("Connections"))
	for _, connection := range connections {
		fmt.Println(*connection.FullName + " forwarding to " + *connection.Destination.CliPath)
	}
}
