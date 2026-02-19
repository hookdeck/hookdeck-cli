package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type CIClient struct {
	Claimed          bool   `json:"claimed"`
	UserID           string `json:"user_id"`
	UserName         string `json:"user_name"`
	OrganizationName string `json:"organization_name"`
	OrganizationID   string `json:"organization_id"`
	ProjectID        string `json:"team_id"`
	ProjectName      string `json:"team_name"`
	ProjectMode      string `json:"team_mode"`
	APIKey           string `json:"key"`
	ClientID         string `json:"client_id"`
}

type CreateCIClientInput struct {
	DeviceName string `json:"device_name"`
}

func (c *Client) CreateCIClient(input CreateCIClientInput) (CIClient, error) {
	input_bytes, err := json.Marshal(input)
	if err != nil {
		return CIClient{}, err
	}
	res, err := c.Post(context.Background(), APIPathPrefix+"/cli-auth/ci", input_bytes, nil)
	if err != nil {
		return CIClient{}, err
	}
	if res.StatusCode != http.StatusOK {
		return CIClient{}, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, err)
	}
	ciClient := CIClient{}
	postprocessJsonResponse(res, &ciClient)
	return ciClient, nil
}
