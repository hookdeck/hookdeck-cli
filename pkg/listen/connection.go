package listen

import (
	"context"
	"fmt"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	log "github.com/sirupsen/logrus"
)

func getConnections(client *hookdeck.Client, sources []*hookdeck.Source, connectionFilterString string, isMultiSource bool, path string) ([]*hookdeck.Connection, error) {
	params := map[string]string{}
	for i, source := range sources {
		params[fmt.Sprintf("source_id[%d]", i)] = source.ID
	}

	connectionResp, err := client.ListConnections(context.Background(), params)
	if err != nil {
		return []*hookdeck.Connection{}, err
	}

	connections, err := filterConnections(toConnectionPtrs(connectionResp.Models), connectionFilterString)
	if err != nil {
		return []*hookdeck.Connection{}, err
	}

	connections, err = ensureConnections(client, connections, sources, isMultiSource, connectionFilterString, path)
	if err != nil {
		return []*hookdeck.Connection{}, err
	}

	return connections, nil
}

// 1. Filter to only include CLI destination
// 2. Apply connectionFilterString
func filterConnections(connections []*hookdeck.Connection, connectionFilterString string) ([]*hookdeck.Connection, error) {
	// 1. Filter to only include CLI destination
	var cliDestinationConnections []*hookdeck.Connection
	for _, connection := range connections {
		cliPath := connection.Destination.GetCLIPath()
		if cliPath != nil && *cliPath != "" {
			cliDestinationConnections = append(cliDestinationConnections, connection)
		}
	}

	if connectionFilterString == "" {
		return cliDestinationConnections, nil
	}

	// 2. Apply connectionFilterString
	isPath, err := isPath(connectionFilterString)
	if err != nil {
		return connections, err
	}
	var filteredConnections []*hookdeck.Connection
	for _, connection := range cliDestinationConnections {
		cliPath := connection.Destination.GetCLIPath()
		if (isPath && cliPath != nil && strings.Contains(*cliPath, connectionFilterString)) || (connection.Name != nil && *connection.Name == connectionFilterString) {
			filteredConnections = append(filteredConnections, connection)
		}
	}

	return filteredConnections, nil
}

// When users want to listen to a single source but there is no connection for that source,
// we can help user set up a new connection for it.
func ensureConnections(client *hookdeck.Client, connections []*hookdeck.Connection, sources []*hookdeck.Source, isMultiSource bool, connectionFilterString string, path string) ([]*hookdeck.Connection, error) {
	if len(connections) > 0 || isMultiSource {
		log.Debug(fmt.Sprintf("Connection exists for Source \"%s\", Connection \"%s\", and path \"%s\"", sources[0].Name, connectionFilterString, path))

		return connections, nil
	}

	// If a connection filter was specified and no match found, don't auto-create
	if connectionFilterString != "" {
		return connections, fmt.Errorf("no connection found matching filter \"%s\" for source \"%s\"", connectionFilterString, sources[0].Name)
	}

	log.Debug(fmt.Sprintf("No connection found. Creating a connection for Source \"%s\", Connection \"%s\", and path \"%s\"", sources[0].Name, connectionFilterString, path))

	connectionDetails := struct {
		ConnectionName  string
		DestinationName string
		Path            string
	}{}

	connectionDetails.DestinationName = fmt.Sprintf("%s-%s", "cli", sources[0].Name)
	connectionDetails.ConnectionName = connectionDetails.DestinationName // Use same name as destination

	if len(path) == 0 {
		connectionDetails.Path = "/"
	} else {
		connectionDetails.Path = path
	}

	// Print message to user about creating the connection
	fmt.Printf("\nThere's no CLI destination connected to %s, creating one named %s\n", sources[0].Name, connectionDetails.DestinationName)

	connection, err := client.CreateConnection(context.Background(), &hookdeck.ConnectionCreateRequest{
		Name:     &connectionDetails.ConnectionName,
		SourceID: &sources[0].ID,
		Destination: &hookdeck.DestinationCreateInput{
			Name: connectionDetails.DestinationName,
			Type: "CLI",
			Config: map[string]interface{}{
				"path": connectionDetails.Path,
			},
		},
	})
	if err != nil {
		return connections, err
	}
	connections = append(connections, connection)

	return connections, nil
}

// toConnectionPtrs converts a slice of Connection values to a slice of Connection pointers.
func toConnectionPtrs(connections []hookdeck.Connection) []*hookdeck.Connection {
	ptrs := make([]*hookdeck.Connection, len(connections))
	for i := range connections {
		ptrs[i] = &connections[i]
	}
	return ptrs
}
