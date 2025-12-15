package hookdeck

import (
	"context"
	"fmt"
)

// RetryEvent retries an event by ID
func (c *Client) RetryEvent(eventID string) error {
	retryURL := fmt.Sprintf("/events/%s/retry", eventID)
	resp, err := c.Post(context.Background(), retryURL, []byte("{}"), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
