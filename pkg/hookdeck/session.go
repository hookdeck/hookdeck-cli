package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Session struct {
	Id string
}

type SessionFilters struct {
	Body    *json.RawMessage `json:"body,omitempty"`
	Headers *json.RawMessage `json:"headers,omitempty"`
	Query   *json.RawMessage `json:"query,omitempty"`
	Path    *json.RawMessage `json:"path,omitempty"`
}

type CreateSessionInput struct {
	ConnectionIds []string        `json:"webhook_ids"`
	Filters       *SessionFilters `json:"filters,omitempty"`
}

func (c *Client) CreateSession(input CreateSessionInput) (Session, error) {
	input_bytes, err := json.Marshal(input)
	if err != nil {
		return Session{}, err
	}
	res, err := c.Post(context.Background(), "/cli-sessions", input_bytes, nil)
	if err != nil {
		return Session{}, err
	}
	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		body, _ := ioutil.ReadAll(res.Body)
		return Session{}, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, string(body))
	}
	session := Session{}
	postprocessJsonResponse(res, &session)
	return session, nil
}
