package hookdeck

import (
	"context"
	"fmt"
)

type Project struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Product string `json:"product"`
}

func (c *Client) ListProjects() ([]Project, error) {
	res, err := c.clientForCLIAuthValidate().Get(context.Background(), APIPathPrefix+"/projects", "", nil)
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
