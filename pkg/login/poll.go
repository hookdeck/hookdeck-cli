package login

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

const maxAttemptsDefault = 2 * 60
const intervalDefault = 1 * time.Second

// PollAPIKeyResponse returns the data of the polling client login
type PollAPIKeyResponse struct {
	Claimed  bool   `json:"claimed"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name"`
	TeamMode string `json:"team_mode"`
	APIKey   string `json:"key"`
	ClientID string `json:"client_id"`
}

// PollForKey polls Hookdeck at the specified interval until either the API key is available or we've reached the max attempts.
func PollForKey(pollURL string, interval time.Duration, maxAttempts int) (*PollAPIKeyResponse, error) {
	if maxAttempts == 0 {
		maxAttempts = maxAttemptsDefault
	}

	if interval == 0 {
		interval = intervalDefault
	}

	parsedURL, err := url.Parse(pollURL)
	if err != nil {
		return nil, err
	}

	baseURL := &url.URL{Scheme: parsedURL.Scheme, Host: parsedURL.Host}

	client := &hookdeck.Client{
		BaseURL: baseURL,
	}

	var count = 0
	for count < maxAttempts {
		res, err := client.Get(context.TODO(), parsedURL.Path, parsedURL.Query().Encode(), nil)
		if err != nil {
			return nil, err
		}

		var response PollAPIKeyResponse

		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, err
		}

		if response.Claimed {
			return &response, nil
		}

		count++
		time.Sleep(interval)
	}

	return nil, errors.New("exceeded max attempts")
}
