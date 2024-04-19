package forms

import (
	"fmt"

	"github.com/charmbracelet/huh"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

type ConnectionForm struct{}

var Connection ConnectionForm

type ConnectionCreateFormInput struct {
	Sources      []*hookdecksdk.Source
	Destinations []*hookdecksdk.Destination
}

func (c ConnectionForm) Create(input ConnectionCreateFormInput) (*hookdecksdk.ConnectionCreateRequest, error) {
	var sourceId string
	var destinationId string

	var sourceOptions []huh.Option[string]
	for _, source := range input.Sources {
		sourceOptions = append(sourceOptions, huh.NewOption(source.Name, source.Id))
	}
	var destinationOptions []huh.Option[string]
	for _, destination := range input.Destinations {
		destinationOptions = append(destinationOptions, huh.NewOption(destination.Name, destination.Id))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose your source").
				Options(
					sourceOptions...,
				).
				Value(&sourceId),

			huh.NewSelect[string]().
				Title("Choose your destination").
				Options(
					destinationOptions...,
				).
				Value(&destinationId),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, err
	}

	fmt.Println(sourceId, destinationId)

	return &hookdecksdk.ConnectionCreateRequest{
		SourceId:      hookdecksdk.OptionalOrNull(&sourceId),
		DestinationId: hookdecksdk.OptionalOrNull(&destinationId),
	}, nil
}
