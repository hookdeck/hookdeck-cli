package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Connection struct {
	Id          string
	Alias       string
	Label       string
	Destination Destination
}

type CreateConnectionInput struct {
	Alias       string                 `json:"alias"`
	Label       string                 `json:"label"`
	SourceId    string                 `json:"source_id"`
	Destination CreateDestinationInput `json:"destination"`
}

type ConnectionList struct {
	Count  int
	Models []Connection
}

func (c *Client) ListConnectionsBySource(source_id string) ([]Connection, error) {
	res, err := c.Get(context.Background(), "/connections", "source_id="+source_id, nil)
	if err != nil {
		return []Connection{}, err
	}
	if res.StatusCode != http.StatusOK {
		return []Connection{}, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, err)
	}
	sources := ConnectionList{}
	postprocessJsonResponse(res, &sources)

	return sources.Models, nil
}

func (c *Client) CreateConnection(input CreateConnectionInput) (Connection, error) {
	input_bytes, err := json.Marshal(input)
	if err != nil {
		return Connection{}, err
	}
	res, err := c.Post(context.Background(), "/connections", input_bytes, nil)
	if err != nil {
		return Connection{}, err
	}
	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		body, _ := ioutil.ReadAll(res.Body)
		return Connection{}, fmt.Errorf("Unexpected http status code: %d %s", res.StatusCode, string(body))
	}
	source := Connection{}
	postprocessJsonResponse(res, &source)
	return source, nil
}
