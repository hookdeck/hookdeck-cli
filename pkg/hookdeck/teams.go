package hookdeck

import (
	"context"
	"fmt"
	"net/http"
)

type Team struct {
	Id   string
	Name string
}

func (c *Client) ListTeams() ([]Team, error) {
	res, err := c.Get(context.Background(), "/teams", "", nil)
	if err != nil {
		return []Team{}, err
	}
	if res.StatusCode != http.StatusOK {
		return []Team{}, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, err)
	}
	teams := []Team{}
	postprocessJsonResponse(res, &teams)

	return teams, nil
}
