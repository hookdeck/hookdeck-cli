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

type GuestSigninLinkResponse struct {
	// Id is the guest user id (not the sign-in token id).
	Id        string `json:"id"`
	Url       string `json:"link"`
	ExpiresAt string `json:"expires_at"`
}

type CreateGuestUserInput struct {
	DeviceName  string `json:"device_name"`
	LinkContext string `json:"link_context,omitempty"`
}

func (c *Client) CreateGuestUser(input CreateGuestUserInput) (GuestUser, error) {
	input_bytes, err := json.Marshal(input)
	if err != nil {
		return GuestUser{}, err
	}
	res, err := c.Post(context.Background(), APIPathPrefix+"/cli/guest", input_bytes, nil)
	if err != nil {
		return GuestUser{}, err
	}
	if res.StatusCode != http.StatusOK {
		return GuestUser{}, fmt.Errorf("unexpected http status code: %d", res.StatusCode)
	}
	guest_user := GuestUser{}
	postprocessJsonResponse(res, &guest_user)
	return guest_user, nil
}

func (c *Client) RefreshGuestSigninLink() (GuestSigninLinkResponse, error) {
	input_bytes, err := json.Marshal(CreateGuestUserInput{LinkContext: "signup"})
	if err != nil {
		return GuestSigninLinkResponse{}, err
	}
	res, err := c.Post(context.Background(), APIPathPrefix+"/cli/guest", input_bytes, nil)
	if err != nil {
		return GuestSigninLinkResponse{}, err
	}
	if res.StatusCode != http.StatusOK {
		return GuestSigninLinkResponse{}, fmt.Errorf("unexpected http status code: %d", res.StatusCode)
	}
	response := GuestSigninLinkResponse{}
	postprocessJsonResponse(res, &response)
	return response, nil
}
