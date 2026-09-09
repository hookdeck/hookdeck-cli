package hookdeck

import (
	"context"
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
	postprocessJsonResponse(res, &projects)

	return projects, nil
}
