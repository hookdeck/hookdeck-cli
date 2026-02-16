package hookdeck

import (
	"context"
)

// RetryEvent retries an event by ID
func (c *Client) RetryEvent(eventID string) error {
	retryURL := APIPathPrefix + "/events/" + eventID + "/retry"
	resp, err := c.Post(context.Background(), retryURL, []byte("{}"), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
