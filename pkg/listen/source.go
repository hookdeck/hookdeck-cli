package listen

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hookdeck/hookdeck-cli/pkg/slug"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
	hookdeckclient "github.com/hookdeck/hookdeck-go-sdk/client"
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

func getSources(sdkClient *hookdeckclient.Client, sourceQuery []string) ([]*hookdecksdk.Source, error) {
	limit := 255 // Hookdeck API limit

	// case 1
	if len(sourceQuery) == 1 && sourceQuery[0] == "*" {
		sources, err := sdkClient.Source.List(context.Background(), &hookdecksdk.SourceListRequest{})
		if err != nil {
			return []*hookdecksdk.Source{}, err
		}
		if sources == nil || *sources.Count == 0 {
			return []*hookdecksdk.Source{}, errors.New("unable to find any matching sources")
		}
		return validateSources(sources.Models)

		// case 2
	} else if len(sourceQuery) > 1 {
		searchedSources, err := listMultipleSources(sdkClient, sourceQuery)
		if err != nil {
			return []*hookdecksdk.Source{}, err
		}
		return validateSources(searchedSources)

		// case 3
	} else if len(sourceQuery) == 1 {
		searchedSources, err := listMultipleSources(sdkClient, sourceQuery)
		if err != nil {
			return []*hookdecksdk.Source{}, err
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
		source, err := createSource(sdkClient, &sourceQuery[0])
		if err != nil {
			return []*hookdecksdk.Source{}, err
		}

		return validateSources([]*hookdecksdk.Source{source})

		// case 4
	} else {
		sources := []*hookdecksdk.Source{}

		availableSources, err := sdkClient.Source.List(context.Background(), &hookdecksdk.SourceListRequest{
			Limit: &limit,
		})

		if err != nil {
			return []*hookdecksdk.Source{}, err
		}

		if *availableSources.Count > 0 {
			selectedSources, err := selectSources(availableSources.Models)
			if err != nil {
				return []*hookdecksdk.Source{}, err
			}
			sources = append(sources, selectedSources...)
		}

		if len(sources) == 0 {
			source, err := createSource(sdkClient, nil)
			if err != nil {
				return []*hookdecksdk.Source{}, err
			}
			sources = append(sources, source)
		}

		return validateSources(sources)
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

func selectSources(availableSources []*hookdecksdk.Source) ([]*hookdecksdk.Source, error) {
	sources := []*hookdecksdk.Source{}

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
		return []*hookdecksdk.Source{}, err
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

func createSource(sdkClient *hookdeckclient.Client, name *string) (*hookdecksdk.Source, error) {
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

	source, err := sdkClient.Source.Create(context.Background(), &hookdecksdk.SourceCreateRequest{
		Name: slug.Make(sourceName),
	})

	return source, err
}

func validateSources(sources []*hookdecksdk.Source) ([]*hookdecksdk.Source, error) {
	if len(sources) == 0 {
		return []*hookdecksdk.Source{}, errors.New("unable to find any matching sources")
	}

	return sources, nil
}
