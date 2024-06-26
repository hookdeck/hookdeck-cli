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

func getConnections(client *hookdeckclient.Client, source *hookdecksdk.Source, connectionQuery string) ([]*hookdecksdk.Connection, error) {
	// TODO: Filter connections using connectionQuery
	var connections []*hookdecksdk.Connection
	connectionList, err := client.Connection.List(context.Background(), &hookdecksdk.ConnectionListRequest{
		SourceId: &source.Id,
	})
	if err != nil {
		return nil, err
	}
	connections = connectionList.Models

	var filteredConnections []*hookdecksdk.Connection
	for _, connection := range connections {
		if connection.Destination.CliPath != nil && *connection.Destination.CliPath != "" {
			filteredConnections = append(filteredConnections, connection)
		}
	}
	connections = filteredConnections

	if connectionQuery != "" {
		is_path, err := isPath(connectionQuery)
		if err != nil {
			return connections, err
		}
		var filteredConnections []*hookdecksdk.Connection
		for _, connection := range connections {
			if (is_path && connection.Destination.CliPath != nil && strings.Contains(*connection.Destination.CliPath, connectionQuery)) || (connection.Name != nil && *connection.Name == connectionQuery) {
				filteredConnections = append(filteredConnections, connection)
			}
		}
		connections = filteredConnections
	}

	if len(connections) == 0 {
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
					is_path, err := isPath(str)
					if !ok || !is_path || err != nil {
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
			SourceId: hookdecksdk.OptionalOrNull(&source.Id),
			Destination: hookdecksdk.OptionalOrNull(&hookdecksdk.ConnectionCreateRequestDestination{
				Name:    alias,
				CliPath: &answers.Path,
			}),
		})
		if err != nil {
			return connections, err
		}
		connections = append(connections, connection)
	}

	return connections, nil
}
