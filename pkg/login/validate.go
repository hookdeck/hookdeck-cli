package login

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/url"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// PollAPIKeyResponse returns the data of the polling client login
type ValidateAPIKeyResponse struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name"`
	TeamMode string `json:"team_mode"`
	ClientID string `json:"client_id"`
}

// PollForKey polls Hookdeck at the specified interval until either the API key is available or we've reached the max attempts.
func ValidateKey(baseURL string, key string) (*ValidateAPIKeyResponse, error) {

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	client := &hookdeck.Client{
		BaseURL: parsedBaseURL,
		APIKey:  key,
	}

	res, err := client.Get(context.Background(), "/cli-auth/validate", "", nil)
	if err != nil {
		return nil, err
	}

	var response ValidateAPIKeyResponse

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}