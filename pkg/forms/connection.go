package forms

import (
	"encoding/json"

	"github.com/charmbracelet/huh"
	"github.com/gosimple/slug"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

type ConnectionForm struct{}

var Connection ConnectionForm

type ConnectionCreateFormInput struct {
	Sources      []*hookdecksdk.Source
	Destinations []*hookdecksdk.Destination
}

const OtherOption = "_other_option_"

func (c ConnectionForm) Create(input ConnectionCreateFormInput) (*hookdecksdk.ConnectionCreateRequest, error) {
	var connectionSourceRequest *hookdecksdk.ConnectionCreateRequestSource
	var connectionDestinationRequest *hookdecksdk.ConnectionCreateRequestDestination

	// Source section

	var sourceId string

	if len(input.Sources) > 0 {
		var sourceOptions []huh.Option[string]
		for _, source := range input.Sources {
			sourceOptions = append(sourceOptions, huh.NewOption(source.Name, source.Id))
		}
		sourceOptions = append(sourceOptions, huh.NewOption("... or create a new source", OtherOption))

		sourceSelect := huh.NewSelect[string]().
			Title("Choose your source").
			Options(
				sourceOptions...,
			).
			Value(&sourceId)

		if err := sourceSelect.Run(); err != nil {
			return nil, err
		}
	}

	if sourceId == "" || sourceId == OtherOption {
		sourceInput, err := Source.Create()
		if err != nil {
			return nil, err
		}
		sourceInputJSON, err := json.Marshal(sourceInput)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(sourceInputJSON, &connectionSourceRequest)
		if err != nil {
			return nil, err
		}
	}

	// Destination section

	var destinationId string
	if len(input.Destinations) > 0 {
		var destinationOptions []huh.Option[string]
		for _, destination := range input.Destinations {
			destinationOptions = append(destinationOptions, huh.NewOption(destination.Name, destination.Id))
		}
		destinationOptions = append(destinationOptions, huh.NewOption("... or create a new destination", OtherOption))

		destinationSelect := huh.NewSelect[string]().
			Title("Choose your destination").
			Options(
				destinationOptions...,
			).
			Value(&destinationId)

		if err := destinationSelect.Run(); err != nil {
			return nil, err
		}
	}

	if destinationId == "" || destinationId == OtherOption {
		destinationInput, err := Destination.Create()
		if err != nil {
			return nil, err
		}
		destinationInputJSON, err := json.Marshal(destinationInput)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(destinationInputJSON, &connectionDestinationRequest)
		if err != nil {
			return nil, err
		}
	}

	// Connection section
	var name string
	var description string
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("What should be your new connection name? (optional)").
			Value(&name),
		huh.NewInput().
			Title("What should be your new connection description? (optional)").
			Value(&description),
	))
	err := form.Run()
	if err != nil {
		return nil, err
	}

	// Construct request payload

	payload := &hookdecksdk.ConnectionCreateRequest{}

	if name != "" {
		name = slug.Make(name)
		payload.Name = hookdecksdk.OptionalOrNull(&name)
	}

	if description != "" {
		payload.Description = hookdecksdk.OptionalOrNull(&description)
	}

	if connectionSourceRequest != nil {
		payload.Source = hookdecksdk.OptionalOrNull(connectionSourceRequest)
	} else {
		payload.SourceId = hookdecksdk.OptionalOrNull(&sourceId)
	}

	if connectionDestinationRequest != nil {
		payload.Destination = hookdecksdk.OptionalOrNull(connectionDestinationRequest)
	} else {
		payload.DestinationId = hookdecksdk.OptionalOrNull(&destinationId)
	}

	return payload, nil
}
