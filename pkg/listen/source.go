package listen

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gosimple/slug"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func getSource(client *hookdeck.Client, source_alias string) (hookdeck.Source, error) {
	var source hookdeck.Source
	if source_alias != "" {
		source, _ = client.GetSourceByAlias(source_alias)
		if source.Id == "" {
			// TODO: Prompt here?
			source, _ = client.CreateSource(hookdeck.CreateSourceInput{
				Alias: source_alias,
				// TODO: labelized alias
				Label: source_alias,
			})
		}
	} else {
		sources, _ := client.ListSources()
		if len(sources) > 0 {
			var sources_alias []string
			for _, temp_source := range sources {
				sources_alias = append(sources_alias, temp_source.Alias)
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
				for _, temp_source := range sources {
					if temp_source.Alias == answers.SourceAlias {
						source = temp_source
					}
				}
			}
		}

		if source.Id == "" {
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

			source, _ = client.CreateSource(hookdeck.CreateSourceInput{
				Alias: slug.Make(answers.Label),
				Label: answers.Label,
			})
		}
	}
	return source, nil
}
