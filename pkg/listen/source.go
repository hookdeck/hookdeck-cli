package listen

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gosimple/slug"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
	hookdeckclient "github.com/hookdeck/hookdeck-go-sdk/client"
)

func getSources(sdkClient *hookdeckclient.Client, sourceQuery []string) ([]*hookdecksdk.Source, error) {
	limit := 100
	var source *hookdecksdk.Source
	if len(sourceQuery) == 1 && sourceQuery[0] == "*" {
		// TODO: remove once we can support better limit
		temporaryLimit := 10
		sources, err := sdkClient.Source.List(context.Background(), &hookdecksdk.SourceListRequest{
			Limit: &temporaryLimit,
		})
		if err != nil {
			return []*hookdecksdk.Source{}, err
		}
		if sources == nil || *sources.Count == 0 {
			return []*hookdecksdk.Source{}, errors.New("unable to find any matching sources")
		}
		return sources.Models, nil
	} else if len(sourceQuery) > 0 {
		return listMultipleSources(sdkClient, sourceQuery)
	} else {
		sources := []*hookdecksdk.Source{}
		availableSources, _ := sdkClient.Source.List(context.Background(), &hookdecksdk.SourceListRequest{
			Limit: &limit,
		})
		if *availableSources.Count > 0 {
			var sourceAliases []string
			for _, temp_source := range availableSources.Models {
				sourceAliases = append(sourceAliases, temp_source.Name)
			}

			answers := struct {
				SourceAlias string `survey:"source"`
			}{}

			var qs = []*survey.Question{
				{
					Name: "source",
					Prompt: &survey.Select{
						Message: "Select a source",
						Options: append(sourceAliases, "Create new source"),
					},
				},
			}

			err := survey.Ask(qs, &answers)
			if err != nil {
				fmt.Println(err.Error())
				return []*hookdecksdk.Source{}, err
			}

			if answers.SourceAlias != "Create new source" {
				for _, currentSource := range availableSources.Models {
					if currentSource.Name == answers.SourceAlias {
						sources = append(sources, currentSource)
					}
				}
			}
		}

		if len(sources) == 0 {
			answers := struct {
				Label string `survey:"label"` // or you can tag fields to match a specific name
			}{}
			var qs = []*survey.Question{
				{
					Name:     "label",
					Prompt:   &survey.Input{Message: "What should be your new source label?"},
					Validate: survey.Required,
				},
			}

			err := survey.Ask(qs, &answers)
			if err != nil {
				return []*hookdecksdk.Source{}, err
			}

			source, _ = sdkClient.Source.Create(context.Background(), &hookdecksdk.SourceCreateRequest{
				Name: slug.Make(answers.Label),
			})
			sources = append(sources, source)
		}

		return sources, nil
	}
}

func listMultipleSources(sdkClient *hookdeckclient.Client, sourceQuery []string) ([]*hookdecksdk.Source, error) {
	sources := []*hookdecksdk.Source{}

	for _, sourceName := range sourceQuery {
		sourceQuery, err := sdkClient.Source.List(context.Background(), &hookdecksdk.SourceListRequest{
			Name: &sourceName,
		})
		if err != nil {
			return []*hookdecksdk.Source{}, err
		}
		if len(sourceQuery.Models) > 0 {
			sources = append(sources, sourceQuery.Models[0])
		}
	}

	return sources, nil
}
