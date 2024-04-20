package forms

import (
	"github.com/charmbracelet/huh"
	"github.com/gosimple/slug"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

type SourceForm struct{}

var Source SourceForm

func (s SourceForm) Create() (*hookdecksdk.SourceCreateRequest, error) {
	var name string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What should be your new source name?").
				Value(&name),
		),
	)

	err := form.Run()
	if err != nil {
		return nil, err
	}

	return &hookdecksdk.SourceCreateRequest{
		Name: slug.Make(name),
	}, nil
}
