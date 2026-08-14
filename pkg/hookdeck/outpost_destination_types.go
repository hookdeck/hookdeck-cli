package hookdeck

import (
	"context"
	"fmt"
)

// OutpostDestinationTypeOption is one choice for a select field.
//
// Label is for display; Value is what the API expects to be sent.
type OutpostDestinationTypeOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// OutpostDestinationTypeField describes one configurable field of a destination
// type. The API returns these so clients can build and validate input without
// hardcoding a schema per type.
type OutpostDestinationTypeField struct {
	Key       string                         `json:"key"`
	Type      string                         `json:"type"` // text, checkbox, key_value_map, select
	Label     string                         `json:"label"`
	Required  bool                           `json:"required"`
	Sensitive bool                           `json:"sensitive"`
	Default   string                         `json:"default,omitempty"`
	MinLength int                            `json:"minlength,omitempty"`
	MaxLength int                            `json:"maxlength,omitempty"`
	Pattern   string                         `json:"pattern,omitempty"`
	Options   []OutpostDestinationTypeOption `json:"options,omitempty"`

	Description string `json:"description,omitempty"`
}

// OptionValues returns the accepted values for a select field, for validation
// and error messages.
func (f OutpostDestinationTypeField) OptionValues() []string {
	values := make([]string, 0, len(f.Options))
	for _, option := range f.Options {
		values = append(values, option.Value)
	}
	return values
}

// OutpostDestinationTypeSetupLink points at provider documentation for a type.
type OutpostDestinationTypeSetupLink struct {
	Href string `json:"href,omitempty"`
	CTA  string `json:"cta,omitempty"`
}

// OutpostDestinationTypeSchema is the full schema for one destination type.
//
// Icon and Instructions are intended for rendering a setup UI and are large, so
// CLI output should generally omit them.
type OutpostDestinationTypeSchema struct {
	Type             string                          `json:"type"`
	Label            string                          `json:"label"`
	Description      string                          `json:"description"`
	Icon             string                          `json:"icon,omitempty"`
	Instructions     string                          `json:"instructions,omitempty"`
	SetupLink        OutpostDestinationTypeSetupLink `json:"setup_link,omitempty"`
	ConfigFields     []OutpostDestinationTypeField   `json:"config_fields"`
	CredentialFields []OutpostDestinationTypeField   `json:"credential_fields"`
}

// ListOutpostDestinationTypes returns the schemas for every available
// destination type. The endpoint is not paginated.
func (c *Client) ListOutpostDestinationTypes(ctx context.Context) ([]OutpostDestinationTypeSchema, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/destination-types", "", nil)
	if err != nil {
		return nil, err
	}

	var schemas []OutpostDestinationTypeSchema
	if _, err := postprocessJsonResponse(resp, &schemas); err != nil {
		return nil, fmt.Errorf("failed to parse destination type list response: %w", err)
	}

	return schemas, nil
}

// GetOutpostDestinationType returns the schema for a single destination type.
func (c *Client) GetOutpostDestinationType(ctx context.Context, destinationType string) (*OutpostDestinationTypeSchema, error) {
	resp, err := c.Get(ctx, outpostPath("destination-types", destinationType), "", nil)
	if err != nil {
		return nil, err
	}

	var schema OutpostDestinationTypeSchema
	if _, err := postprocessJsonResponse(resp, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse destination type response: %w", err)
	}

	return &schema, nil
}

// ListOutpostTopics returns the topics configured for the project. Topics are
// operator configuration, so there is no create endpoint — they are set through
// the managed config.
func (c *Client) ListOutpostTopics(ctx context.Context) ([]string, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/topics", "", nil)
	if err != nil {
		return nil, err
	}

	var topics []string
	if _, err := postprocessJsonResponse(resp, &topics); err != nil {
		return nil, fmt.Errorf("failed to parse topic list response: %w", err)
	}

	return topics, nil
}
