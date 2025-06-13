package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Project struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Mode           string `json:"mode"`
	OrganizationID string `json:"organization_id"` // Added field, removed omitempty
}

type createProjectPayload struct {
	Name           string `json:"name"`
	IsPrivate      bool   `json:"private"`
	OrganizationID string `json:"organization_id"`
}

func (c *Client) ListProjects() ([]Project, error) {
	res, err := c.Get(context.Background(), "/teams", "", nil)
	if err != nil {
		return []Project{}, err
	}
	if res.StatusCode != http.StatusOK {
		return []Project{}, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, err)
	}
	projects := []Project{}
	postprocessJsonResponse(res, &projects)

	return projects, nil
}

func (c *Client) CreateProject(ctx context.Context, name string, organizationID string, isPrivate bool) (Project, error) {
	payloadStruct := createProjectPayload{
		Name:           name,
		IsPrivate:      isPrivate,
		OrganizationID: organizationID,
	}

	payloadBytes, err := json.Marshal(payloadStruct)
	if err != nil {
		return Project{}, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	res, err := c.Post(ctx, "/teams", payloadBytes, nil)
	if err != nil {
		return Project{}, err
	}

	if res.StatusCode != http.StatusCreated {
		return Project{}, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, res.Status)
	}

	var project Project
	// Assuming postprocessJsonResponse populates project and returns (value, error)
	_, err = postprocessJsonResponse(res, &project)
	if err != nil {
		return Project{}, fmt.Errorf("failed to unmarshal project response: %w", err)
	}

	return project, nil
}
