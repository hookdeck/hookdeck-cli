package hookdeck

import (
	"context"
)

type Project struct {
	Id   string
	Name string
	Mode string
}

func (c *Client) ListProjects() ([]Project, error) {
	res, err := c.Get(context.Background(), APIPathPrefix+"/teams", "", nil)
	if err != nil {
		return []Project{}, err
	}
	if err := checkAndPrintError(res); err != nil {
		return []Project{}, err
	}
	projects := []Project{}
	postprocessJsonResponse(res, &projects)

	return projects, nil
}
