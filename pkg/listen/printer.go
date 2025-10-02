package listen

import (
	"fmt"
	"net/url"

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

	if guestURL != "" {
		fmt.Printf("─ %s ──────────────────────────────────────────────────────\n", "Console")
		fmt.Println()
		fmt.Println("👉  Sign up to make your webhook URL permanent: %s", guestURL)
	} else {
		var url = config.DashboardBaseURL
		if config.Profile.ProjectId != "" {
			url += "/events/cli?team_id=" + config.Profile.ProjectId
		}
		if config.Profile.ProjectMode == "console" {
			url = config.ConsoleBaseURL
		}
		fmt.Printf("─ %s ──────────────────────────────────────────────────────\n", "Dashboard")
		fmt.Println()
		fmt.Printf("👉 Inspect, retry & boomark events: %s\n", url)
	}
}

func printSourcesWithConnections(config *config.Config, sources []*hookdecksdk.Source, connections []*hookdecksdk.Connection, targetURL *url.URL) {
	// Group connections by source ID
	sourceConnections := make(map[string][]*hookdecksdk.Connection)
	for _, connection := range connections {
		sourceID := connection.Source.Id
		sourceConnections[sourceID] = append(sourceConnections[sourceID], connection)
	}

	// Print the Sources title line
	fmt.Printf("─ %s ───────────────────────────────────────────────────\n", "Listening on")
	fmt.Println()

	// Print each source with its connections
	for _, source := range sources {
		// Print the source URL
		fmt.Printf("%s: %s\n", ansi.Bold(source.Name), source.Url)

		// Print connections for this source
		if sourceConns, exists := sourceConnections[source.Id]; exists {
			for _, connection := range sourceConns {
				// Calculate indentation based on source name length
				indent := len(source.Name) + 2 // +2 for ": "
				fullPath := targetURL.Scheme + "://" + targetURL.Host + *connection.Destination.CliPath
				fmt.Printf("%*s↳ %s → %s\n", indent, "", *connection.Name, fullPath)
			}
		}
	}
}
