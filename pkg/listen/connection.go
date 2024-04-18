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

func getConnections(client *hookdeckclient.Client, source *hookdecksdk.Source, connection_query string) ([]*hookdecksdk.Connection, error) {
	// TODO: Filter connections using connection_query
	var connections []*hookdecksdk.Connection
	connectionList, err := client.Connection.List(context.Background(), &hookdecksdk.ConnectionListRequest{
		SourceId: &source.Id,
	})
	if err != nil {
		return connectionList.Models, err
	}

	var filtered_connections []*hookdecksdk.Connection
	for _, connection := range connections {
		if *connection.Destination.CliPath != "" {
			filtered_connections = append(filtered_connections, connection)
		}
	}
	connections = filtered_connections

	if connection_query != "" {
		is_path, err := isPath(connection_query)
		if err != nil {
			return connections, err
		}
		var filtered_connections []*hookdecksdk.Connection
		for _, connection := range connections {
			if (is_path && strings.Contains(*connection.Destination.CliPath, connection_query)) || *connection.Name == connection_query {
				filtered_connections = append(filtered_connections, connection)
			}
		}
		connections = filtered_connections
	}

	if len(connections) == 0 {
		answers := struct {
			Label string `survey:"label"`
			Path  string `survey:"path"`
		}{}
		var qs = []*survey.Question{
			{
				Name:   "path",
				Prompt: &survey.Input{Message: "What path should the webhooks be forwarded to (ie: /webhooks)?"},
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
