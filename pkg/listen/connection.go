package listen

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gosimple/slug"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func getConnections(client *hookdeck.Client, source hookdeck.Source, connection_query string) ([]hookdeck.Connection, error) {
	// TODO: Filter connections using connection_query
	var connections []hookdeck.Connection
	connections, err := client.ListConnectionsBySource(source.Id)
	if err != nil {
		return connections, err
	}

	var filtered_connections []hookdeck.Connection
	for _, connection := range connections {
		if connection.Destination.CliPath != "" {
			filtered_connections = append(filtered_connections, connection)
		}
	}
	connections = filtered_connections

	if connection_query != "" {
		is_path, err := isPath(connection_query)
		if err != nil {
			return connections, err
		}
		var filtered_connections []hookdeck.Connection
		for _, connection := range connections {
			if (is_path && strings.Contains(connection.Destination.CliPath, connection_query)) || connection.Alias == connection_query {
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
		connection, err := client.CreateConnection(hookdeck.CreateConnectionInput{
			Alias:    alias,
			Label:    answers.Label,
			SourceId: source.Id,
			Destination: hookdeck.CreateDestinationInput{
				Alias:   alias,
				Label:   answers.Label,
				CliPath: answers.Path,
			},
		})
		if err != nil {
			return connections, err
		}
		connections = append(connections, connection)
	}

	return connections, nil
}
