package forms

import (
	"github.com/charmbracelet/huh"
	"github.com/gosimple/slug"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

type DestinationForm struct{}

var Destination DestinationForm

func (s DestinationForm) Create() (*hookdecksdk.DestinationCreateRequest, error) {
	var name string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What should be your new destination name?").
				Value(&name),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, err
	}

	return &hookdecksdk.DestinationCreateRequest{
		Name: slug.Make(name),
	}, nil
}
