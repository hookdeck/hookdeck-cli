package listen

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gosimple/slug"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
	hookdeckclient "github.com/hookdeck/hookdeck-go-sdk/client"
)

func getSource(sdkClient *hookdeckclient.Client, source_alias string) (*hookdecksdk.Source, error) {
	var source *hookdecksdk.Source
	if source_alias != "" {
		sources, _ := sdkClient.Source.List(context.Background(), &hookdecksdk.SourceListRequest{
			Name: &source_alias,
		})
		if *sources.Count > 0 {
			source = sources.Models[0]
		}
		if source == nil {
			// TODO: Prompt here?
			source, _ = sdkClient.Source.Create(context.Background(), &hookdecksdk.SourceCreateRequest{
				Name: slug.Make(source_alias),
			})
		}
	} else {
		sources, _ := sdkClient.Source.List(context.Background(), &hookdecksdk.SourceListRequest{})
		if *sources.Count > 0 {
			var sources_alias []string
			for _, temp_source := range sources.Models {
				sources_alias = append(sources_alias, temp_source.Name)
			}

			answers := struct {
				SourceAlias string `survey:"source"`
			}{}

			var qs = []*survey.Question{
				{
					Name: "source",
					Prompt: &survey.Select{
						Message: "Select a source",
						Options: append(sources_alias, "Create new source"),
					},
				},
			}

			err := survey.Ask(qs, &answers)
			if err != nil {
				fmt.Println(err.Error())
				return source, err
			}

			if answers.SourceAlias != "Create new source" {
				for _, temp_source := range sources.Models {
					if temp_source.Name == answers.SourceAlias {
						source = temp_source
					}
				}
			}
		}

		if source == nil {
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
				return source, err
			}

			source, _ = sdkClient.Source.Create(context.Background(), &hookdecksdk.SourceCreateRequest{
				Name: slug.Make(answers.Label),
			})
		}
	}
	return source, nil
}
