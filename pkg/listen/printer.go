package listen

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/listen/links"
	"github.com/hookdeck/hookdeck-cli/pkg/listen/summary"
)

// hyperlink renders url as an OSC 8 hyperlink labelled display when w can show
// one, and as the full url — query parameters and all — when it cannot.
//
// Both halves matter for #403. Emitting the escape to a pipe wrote bytes nothing
// downstream can render; and because the label deliberately omits team_id, the
// plain-text fallback has to be the real url or the redirected output ends up
// carrying *less* information than the terminal output it replaced.
func hyperlink(url, display string, w io.Writer) string {
	if !ansi.CanHyperlink(w) {
		return url
	}
	return ansi.Linkify(display, url, w)
}

func printSourcesWithConnections(config *config.Config, projectID string, sources []*hookdeck.Source, connections []*hookdeck.Connection, targetURL *url.URL, guestURL string) {
	// Group connections by source ID
	sourceConnections := make(map[string][]*hookdeck.Connection)
	for _, connection := range connections {
		sourceID := connection.Source.ID
		sourceConnections[sourceID] = append(sourceConnections[sourceID], connection)
	}

	// Print the Sources title line. It carries the same counts as the
	// interactive header: compact is the automatic no-TTY fallback, so this is
	// the line most CI logs keep, and a bare "Listening on" told them nothing.
	fmt.Printf("%s\n", ansi.Faint(summary.Listening(len(sources), len(connections))))
	fmt.Println()

	// Print each source with its connections
	for i, source := range sources {
		// Print source name
		fmt.Printf("%s\n", ansi.Bold(source.Name))

		// Print connections for this source
		if sourceConns, exists := sourceConnections[source.ID]; exists {
			numConns := len(sourceConns)

			// Print webhook URL with vertical line only (no horizontal branch)
			fmt.Printf("│  Requests to → %s\n", source.URL)

			// Print each connection
			for j, connection := range sourceConns {
				cliPath := connection.Destination.GetCLIPath()
				path := "/"
				if cliPath != nil {
					path = *cliPath
				}
				fullPath := targetURL.Scheme + "://" + targetURL.Host + path

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
			fmt.Printf("   Request sents to → %s\n", source.URL)
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
		url := links.DashboardHome(config.DashboardBaseURL, config.ConsoleBaseURL, config.Profile.ProjectType, projectID)
		displayURL := links.DashboardHomeDisplay(config.DashboardBaseURL, config.ConsoleBaseURL, config.Profile.ProjectType)
		fmt.Printf("💡 Open dashboard to inspect, retry & bookmark events: %s\n", hyperlink(url, displayURL, os.Stdout))
	}
}
