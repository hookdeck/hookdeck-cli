package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Source struct {
	Id    string
	Alias string
	Label string
	Url   string
}

type CreateSourceInput struct {
	Alias string `json:"alias"`
	Label string `json:"label"`
}

type SourceList struct {
	Count  int
	Models []Source
}

func (c *Client) GetSourceByAlias(alias string) (Source, error) {
	res, err := c.Get(context.Background(), "/sources", "alias="+alias, nil)
	if err != nil {
		return Source{}, err
	}
	if res.StatusCode != http.StatusOK {
		return Source{}, fmt.Errorf("Unexpected http status code: %d %s", res.StatusCode)
	}
	sources := SourceList{}
	postprocessJsonResponse(res, &sources)

	if len(sources.Models) > 0 {
		return sources.Models[0], nil
	}
	return Source{}, nil
}

func (c *Client) ListSources() ([]Source, error) {
	res, err := c.Get(context.Background(), "/sources", "", nil)
	if err != nil {
		return []Source{}, err
	}
	if res.StatusCode != http.StatusOK {
		return []Source{}, fmt.Errorf("Unexpected http status code: %d %s", res.StatusCode)
	}
	sources := SourceList{}
	postprocessJsonResponse(res, &sources)

	return sources.Models, nil
}

func (c *Client) CreateSource(input CreateSourceInput) (Source, error) {
	input_bytes, err := json.Marshal(input)
	if err != nil {
		return Source{}, err
	}
	res, err := c.Post(context.Background(), "/sources", input_bytes, nil)
	if err != nil {
		return Source{}, err
	}
	if res.StatusCode != http.StatusOK {
		return Source{}, fmt.Errorf("Unexpected http status code: %d %s", res.StatusCode)
	}
	source := Source{}
	postprocessJsonResponse(res, &source)
	return source, nil
}
