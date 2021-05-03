package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type GuestUser struct {
	Id         string `json:"id"`
	APIKey     string `json:"key"`
	Url        string `json:"link"`
	BrowserURL string `json:"browser_url"`
	PollURL    string `json:"poll_url"`
}

type CreateGuestUserInput struct {
	DeviceName string `json:"device_name"`
}

func (c *Client) CreateGuestUser(input CreateGuestUserInput) (GuestUser, error) {
	input_bytes, err := json.Marshal(input)
	if err != nil {
		return GuestUser{}, err
	}
	res, err := c.Post(context.Background(), "/cli/guest", input_bytes, nil)
	if err != nil {
		return GuestUser{}, err
	}
	if res.StatusCode != http.StatusOK {
		return GuestUser{}, fmt.Errorf("Unexpected http status code: %d %s", res.StatusCode)
	}
	guest_user := GuestUser{}
	postprocessJsonResponse(res, &guest_user)
	return guest_user, nil
}
