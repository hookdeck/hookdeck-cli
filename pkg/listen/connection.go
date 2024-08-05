package listen

import (
	"context"
	"fmt"
	"strings"

	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
	hookdeckclient "github.com/hookdeck/hookdeck-go-sdk/client"
	log "github.com/sirupsen/logrus"
)

func getConnections(client *hookdeckclient.Client, sources []*hookdecksdk.Source, connectionFilterString string, isMultiSource bool, path string) ([]*hookdecksdk.Connection, error) {
	sourceIDs := []*string{}

	for _, source := range sources {
		sourceIDs = append(sourceIDs, &source.Id)
	}

	connectionQuery, err := client.Connection.List(context.Background(), &hookdecksdk.ConnectionListRequest{
		SourceId: sourceIDs,
	})
	if err != nil {
		return []*hookdecksdk.Connection{}, err
	}

	connections, err := filterConnections(connectionQuery.Models, connectionFilterString)
	if err != nil {
		return []*hookdecksdk.Connection{}, err
	}

	connections, err = ensureConnections(client, connections, sources, isMultiSource, connectionFilterString, path)
	if err != nil {
		return []*hookdecksdk.Connection{}, err
	}

	return connections, nil
}

// 1. Filter to only include CLI destination
// 2. Apply connectionFilterString
func filterConnections(connections []*hookdecksdk.Connection, connectionFilterString string) ([]*hookdecksdk.Connection, error) {
	// 1. Filter to only include CLI destination
	var cliDestinationConnections []*hookdecksdk.Connection
	for _, connection := range connections {
		if connection.Destination.CliPath != nil && *connection.Destination.CliPath != "" {
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
	var filteredConnections []*hookdecksdk.Connection
	for _, connection := range cliDestinationConnections {
		if (isPath && connection.Destination.CliPath != nil && strings.Contains(*connection.Destination.CliPath, connectionFilterString)) || (connection.Name != nil && *connection.Name == connectionFilterString) {
			filteredConnections = append(filteredConnections, connection)
		}
	}

	return filteredConnections, nil
}

// When users want to listen to a single source but there is no connection for that source,
// we can help user set up a new connection for it.
func ensureConnections(client *hookdeckclient.Client, connections []*hookdecksdk.Connection, sources []*hookdecksdk.Source, isMultiSource bool, connectionFilterString string, path string) ([]*hookdecksdk.Connection, error) {
	if len(connections) > 0 || isMultiSource {
		log.Debug(fmt.Sprintf("Connection exists for Source \"%s\", Connection \"%s\", and path \"%s\"", sources[0].Name, connectionFilterString, path))

		return connections, nil
	}

	log.Debug(fmt.Sprintf("No connection found. Creating a connection for Source \"%s\", Connection \"%s\", and path \"%s\"", sources[0].Name, connectionFilterString, path))

	connectionDetails := struct {
		ConnectionName  string
		DestinationName string
		Path            string
	}{}

	connectionDetails.DestinationName = fmt.Sprintf("%s-%s", "cli", sources[0].Name)

	if len(connectionFilterString) == 0 {
		connectionDetails.ConnectionName = fmt.Sprintf("%s_to_%s", sources[0].Name, connectionDetails.DestinationName)
	} else {
		connectionDetails.ConnectionName = connectionFilterString
	}

	if len(path) == 0 {
		connectionDetails.Path = "/"
	} else {
		connectionDetails.Path = path
	}

	connection, err := client.Connection.Create(context.Background(), &hookdecksdk.ConnectionCreateRequest{
		Name:     hookdecksdk.OptionalOrNull(&connectionDetails.ConnectionName),
		SourceId: hookdecksdk.OptionalOrNull(&sources[0].Id),
		Destination: hookdecksdk.OptionalOrNull(&hookdecksdk.ConnectionCreateRequestDestination{
			Name:    connectionDetails.DestinationName,
			CliPath: &connectionDetails.Path,
		}),
	})
	if err != nil {
		return connections, err
	}
	connections = append(connections, connection)

	return connections, nil
}
