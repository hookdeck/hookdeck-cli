package listen

import (
	"fmt"
	"net/url"
	"strings"

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

func printSourcesWithConnections(config *config.Config, sources []*hookdecksdk.Source, connections []*hookdecksdk.Connection, targetURL *url.URL, guestURL string) {
	// Group connections by source ID
	sourceConnections := make(map[string][]*hookdecksdk.Connection)
	for _, connection := range connections {
		sourceID := connection.Source.Id
		sourceConnections[sourceID] = append(sourceConnections[sourceID], connection)
	}

	// Print the Sources title line
	fmt.Printf("%s\n", ansi.Faint("Listening on"))
	fmt.Println()

	// Print each source with its connections
	for i, source := range sources {
		// Print source name
		fmt.Printf("%s\n", ansi.Bold(source.Name))

		// Print connections for this source
		if sourceConns, exists := sourceConnections[source.Id]; exists {
			numConns := len(sourceConns)

			// Print webhook URL with vertical line only (no horizontal branch)
			fmt.Printf("│  Requests to → %s\n", source.Url)

			// Print each connection
			for j, connection := range sourceConns {
				fullPath := targetURL.Scheme + "://" + targetURL.Host + *connection.Destination.CliPath

				// Get connection name from FullName (format: "source -> destination")
				// Split on "->" and take the second part (destination)
				connNameDisplay := ""
				if connection.FullName != nil && *connection.FullName != "" {
					parts := strings.Split(*connection.FullName, "->")
					if len(parts) == 2 {
						destinationName := strings.TrimSpace(parts[1])
						if destinationName != "" {
							connNameDisplay = " " + ansi.Faint(fmt.Sprintf("(%s)", destinationName))
						}
					}
				}

				if j == numConns-1 {
					// Last connection - use └─
					fmt.Printf("└─ Forwards to → %s%s\n", fullPath, connNameDisplay)
				} else {
					// Not last connection - use ├─
					fmt.Printf("├─ Forwards to → %s%s\n", fullPath, connNameDisplay)
				}
			}
		} else {
			// No connections, just show webhook URL
			fmt.Printf("   Request sents to → %s\n", source.Url)
		}

		// Add spacing between sources (but not after the last one)
		if i < len(sources)-1 {
			fmt.Println()
		}
	}

	// Print dashboard hint
	fmt.Println()
	if guestURL != "" {
		fmt.Printf("💡 Sign up to make your webhook URL permanent: %s\n", guestURL)
	} else {
		var url = config.DashboardBaseURL
		var displayURL = config.DashboardBaseURL
		if config.Profile.ProjectId != "" {
			url += "/events/cli?team_id=" + config.Profile.ProjectId
			displayURL += "/events/cli"
		}
		if config.Profile.ProjectMode == "console" {
			url = config.ConsoleBaseURL
			displayURL = config.ConsoleBaseURL
		}
		// Create clickable link with OSC 8 hyperlink sequence
		// Format: \033]8;;URL\033\\DISPLAY_TEXT\033]8;;\033\\
		fmt.Printf("💡 View dashboard to inspect, retry & bookmark events: \033]8;;%s\033\\%s\033]8;;\033\\\n", url, displayURL)
	}
}
