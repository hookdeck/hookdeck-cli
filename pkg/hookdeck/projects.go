package hookdeck

import (
	"context"
	"fmt"
	"net/http"
)

type Project struct {
	Id   string
	Name string
	Mode string
}

func (c *Client) ListProjects() ([]Project, error) {
	res, err := c.Get(context.Background(), "/2025-07-01/teams", "", nil)
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
