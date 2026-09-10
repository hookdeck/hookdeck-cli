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
	ProjectType      string `json:"team_type"`
	// ProjectProduct and ProjectMode are earlier names for the same field:
	// team_product was served briefly before the rename to team_type, and
	// team_mode before that. Both are still read so a response from an API
	// that has not been updated yet resolves a project type instead of
	// blanking it. Drop ProjectProduct once 2026-09-01 is deployed.
	ProjectProduct string `json:"team_product"`
	ProjectMode    string `json:"team_mode"`
	APIKey         string `json:"key"`
	ClientID       string `json:"client_id"`
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
