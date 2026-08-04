package listen

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/slug"
	"golang.org/x/term"
)

// There are 4 cases:
//
// 1. search all sources (query string == '*')
// 2. search multiple sources
// 3. search 1 source
// 4. no specific source
//
// For case 1 & 2, we'll simply query the sources data and return.
// If no source is found, we'll show an error message and exit.
//
// For case 3, we'll search for the 1 source.
// If that source is not found, we'll create it and move forward.
//
// For case 4, we'll get available sources and ask the user which ones
// they'd like to use. They will also have an option to create a new source.

func getSources(client *hookdeck.Client, sourceQuery []string) ([]*hookdeck.Source, error) {
	// case 1
	if len(sourceQuery) == 1 && sourceQuery[0] == "*" {
		resp, err := client.ListSources(context.Background(), nil)
		if err != nil {
			return []*hookdeck.Source{}, err
		}
		if resp == nil || len(resp.Models) == 0 {
			return []*hookdeck.Source{}, errors.New("unable to find any matching sources")
		}
		return validateSources(toSourcePtrs(resp.Models))

		// case 2
	} else if len(sourceQuery) > 1 {
		searchedSources, err := listMultipleSources(client, sourceQuery)
		if err != nil {
			return []*hookdeck.Source{}, err
		}
		return validateSources(searchedSources)

		// case 3
	} else if len(sourceQuery) == 1 {
		searchedSources, err := listMultipleSources(client, sourceQuery)
		if err != nil {
			return []*hookdeck.Source{}, err
		}
		if len(searchedSources) > 0 {
			return validateSources(searchedSources)
		}

		// Source not found, ask user if they want to create it
		fmt.Printf("\nSource \"%s\" not found.\n", sourceQuery[0])

		createConfirm := false

		// Check if stdin is a TTY (interactive terminal)
		// If not (e.g., in CI or piped input), auto-accept source creation
		isInteractive := term.IsTerminal(int(os.Stdin.Fd()))

		if isInteractive {
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Do you want to create a new source named \"%s\"?", sourceQuery[0]),
			}
			err = survey.AskOne(prompt, &createConfirm)
			if err != nil {
				// If survey fails (e.g., in background process or broken pipe), auto-accept in non-interactive scenarios
				// Check if it's a terminal-related error
				if err.Error() == "interrupt" {
					// User pressed Ctrl+C, exit cleanly
					os.Exit(0)
				}
				// For other errors (like broken pipe, EOF), assume non-interactive and auto-accept
				fmt.Printf("Cannot prompt for confirmation. Automatically creating source \"%s\".\n", sourceQuery[0])
				createConfirm = true
			} else if !createConfirm {
				// User declined to create source, exit cleanly without error message
				os.Exit(0)
			}
		} else {
			// Non-interactive mode: auto-accept source creation
			fmt.Printf("Non-interactive mode detected. Automatically creating source \"%s\".\n", sourceQuery[0])
			createConfirm = true
		}

		// Create source with provided name
		source, err := createSource(client, &sourceQuery[0])
		if err != nil {
			return []*hookdeck.Source{}, err
		}

		return validateSources([]*hookdeck.Source{source})

		// case 4
	} else {
		sources := []*hookdeck.Source{}

		availableSources, err := client.ListSources(context.Background(), map[string]string{
			"limit": "255",
		})

		if err != nil {
			return []*hookdeck.Source{}, err
		}

		if len(availableSources.Models) > 0 {
			selectedSources, err := selectSources(toSourcePtrs(availableSources.Models))
			if err != nil {
				return []*hookdeck.Source{}, err
			}
			sources = append(sources, selectedSources...)
		}

		if len(sources) == 0 {
			source, err := createSource(client, nil)
			if err != nil {
				return []*hookdeck.Source{}, err
			}
			sources = append(sources, source)
		}

		return validateSources(sources)
	}
}

func listMultipleSources(client *hookdeck.Client, sourceQuery []string) ([]*hookdeck.Source, error) {
	sources := []*hookdeck.Source{}

	for _, sourceName := range sourceQuery {
		resp, err := client.ListSources(context.Background(), map[string]string{
			"name": sourceName,
		})
		if err != nil {
			return []*hookdeck.Source{}, err
		}
		if len(resp.Models) > 0 {
			src := resp.Models[0]
			sources = append(sources, &src)
		}
	}

	return sources, nil
}

func selectSources(availableSources []*hookdeck.Source) ([]*hookdeck.Source, error) {
	sources := []*hookdeck.Source{}

	var sourceAliases []string
	for _, temp_source := range availableSources {
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
		return []*hookdeck.Source{}, err
	}

	if answers.SourceAlias != "Create new source" {
		for _, currentSource := range availableSources {
			if currentSource.Name == answers.SourceAlias {
				sources = append(sources, currentSource)
			}
		}
	}

	return sources, nil
}

func createSource(client *hookdeck.Client, name *string) (*hookdeck.Source, error) {
	var sourceName string

	fmt.Println("\033[2mA source represents where requests originate from (ie. Github, Stripe, Shopify, etc.). Each source has it's own unique URL that you can use to send requests to.\033[0m")

	if name != nil {
		sourceName = *name
	} else {
		answers := struct {
			Label string `survey:"label"` // or you can tag fields to match a specific name
		}{}
		var qs = []*survey.Question{
			{
				Name:     "label",
				Prompt:   &survey.Input{Message: "What should be the name of your first source?"},
				Validate: survey.Required,
			},
		}

		err := survey.Ask(qs, &answers)
		if err != nil {
			return nil, err
		}
		sourceName = answers.Label
	}

	source, err := client.CreateSource(context.Background(), &hookdeck.SourceCreateRequest{
		Name: slug.Make(sourceName),
	})

	return source, err
}

func validateSources(sources []*hookdeck.Source) ([]*hookdeck.Source, error) {
	if len(sources) == 0 {
		return []*hookdeck.Source{}, errors.New("unable to find any matching sources")
	}

	return sources, nil
}

// toSourcePtrs converts a slice of Source values to a slice of Source pointers.
func toSourcePtrs(sources []hookdeck.Source) []*hookdeck.Source {
	ptrs := make([]*hookdeck.Source, len(sources))
	for i := range sources {
		ptrs[i] = &sources[i]
	}
	return ptrs
}
