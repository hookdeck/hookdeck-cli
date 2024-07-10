package listen

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gosimple/slug"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
	hookdeckclient "github.com/hookdeck/hookdeck-go-sdk/client"
)

func getConnections(client *hookdeckclient.Client, sources []*hookdecksdk.Source, connectionFilterString string, isMultiSource bool) ([]*hookdecksdk.Connection, error) {
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

	connections, err = ensureConnections(client, connections, sources, isMultiSource)
	if err != nil {
		return []*hookdecksdk.Connection{}, err
	}

	return connections, nil
}

func filterConnections(connections []*hookdecksdk.Connection, connectionFilterString string) ([]*hookdecksdk.Connection, error) {
	if connectionFilterString == "" {
		return connections, nil
	}

	isPath, err := isPath(connectionFilterString)
	if err != nil {
		return connections, err
	}
	var filteredConnections []*hookdecksdk.Connection
	for _, connection := range connections {
		if (isPath && connection.Destination.CliPath != nil && strings.Contains(*connection.Destination.CliPath, connectionFilterString)) || (connection.Name != nil && *connection.Name == connectionFilterString) {
			filteredConnections = append(filteredConnections, connection)
		}
	}

	return filteredConnections, nil
}

// When users want to listen to a single source but there is no connection for that source,
// we can help user set up a new connection for it.
func ensureConnections(client *hookdeckclient.Client, connections []*hookdecksdk.Connection, sources []*hookdecksdk.Source, isMultiSource bool) ([]*hookdecksdk.Connection, error) {
	if len(connections) > 0 || isMultiSource {
		return connections, nil
	}

	answers := struct {
		Label string `survey:"label"`
		Path  string `survey:"path"`
	}{}
	var qs = []*survey.Question{
		{
			Name:   "path",
			Prompt: &survey.Input{Message: "What path should the events be forwarded to (ie: /webhooks)?"},
			Validate: func(val interface{}) error {
				str, ok := val.(string)
				isPath, err := isPath(str)
				if !ok || !isPath || err != nil {
					return errors.New("invalid path")
				}
				return nil
			},
		},
		{
			Name:     "label",
			Prompt:   &survey.Input{Message: "What's your connection label (ie: My API)?"},
			Validate: survey.Required,
		},
	}

	err := survey.Ask(qs, &answers)
	if err != nil {
		fmt.Println(err.Error())
		return connections, err
	}
	alias := slug.Make(answers.Label)
	connection, err := client.Connection.Create(context.Background(), &hookdecksdk.ConnectionCreateRequest{
		Name:     hookdecksdk.OptionalOrNull(&alias),
		SourceId: hookdecksdk.OptionalOrNull(&sources[0].Id),
		Destination: hookdecksdk.OptionalOrNull(&hookdecksdk.ConnectionCreateRequestDestination{
			Name:    alias,
			CliPath: &answers.Path,
		}),
	})
	if err != nil {
		return connections, err
	}
	connections = append(connections, connection)

	return connections, nil
}
