package hookdeck

import (
	"context"
	"fmt"
)

type Project struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (c *Client) ListProjects() ([]Project, error) {
	return c.ListProjectsContext(context.Background())
}

// ListProjectsContext is ListProjects bounded by ctx, for callers that must not
// wait indefinitely on the API.
func (c *Client) ListProjectsContext(ctx context.Context) ([]Project, error) {
	res, err := c.clientForCLIAuthValidate().Get(ctx, APIPathPrefix+"/projects", "", nil)
	if err != nil {
		return []Project{}, err
	}
	if err := checkAndPrintError(res); err != nil {
		return []Project{}, err
	}
	projects := []Project{}
	// A shape mismatch here used to return an empty list and a nil error, so a
	// renamed field or a wrapped envelope read as "you have no projects".
	if _, err := postprocessJsonResponse(res, &projects); err != nil {
		return []Project{}, fmt.Errorf("failed to parse project list response: %w", err)
	}

	return projects, nil
}
