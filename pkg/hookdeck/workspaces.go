package hookdeck

import (
	"context"
	"fmt"
	"net/http"
)

type Workspace struct {
	Id   string
	Name string
	Mode string
}

func (c *Client) ListWorkspaces() ([]Workspace, error) {
	res, err := c.Get(context.Background(), "/teams", "", nil)
	if err != nil {
		return []Workspace{}, err
	}
	if res.StatusCode != http.StatusOK {
		return []Workspace{}, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, err)
	}
	workspaces := []Workspace{}
	postprocessJsonResponse(res, &workspaces)

	return workspaces, nil
}
