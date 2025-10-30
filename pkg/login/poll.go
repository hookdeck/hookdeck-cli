package login

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"strings"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	log "github.com/sirupsen/logrus"
)

const maxAttemptsDefault = 2 * 60
const intervalDefault = 2 * time.Second
const maxBackoffInterval = 30 * time.Second

// PollAPIKeyResponse returns the data of the polling client login
type PollAPIKeyResponse struct {
	Claimed          bool   `json:"claimed"`
	UserID           string `json:"user_id"`
	UserName         string `json:"user_name"`
	UserEmail        string `json:"user_email"`
	OrganizationName string `json:"organization_name"`
	OrganizationID   string `json:"organization_id"`
	ProjectID        string `json:"team_id"`
	ProjectName      string `json:"team_name"`
	ProjectMode      string `json:"team_mode"`
	APIKey           string `json:"key"`
	ClientID         string `json:"client_id"`
}

// PollForKey polls Hookdeck at the specified interval until either the API key is available or we've reached the max attempts.
func PollForKey(pollURL string, interval time.Duration, maxAttempts int) (*PollAPIKeyResponse, error) {
	if maxAttempts == 0 {
		maxAttempts = maxAttemptsDefault
	}

	if interval == 0 {
		interval = intervalDefault
	}

	parsedURL, err := url.Parse(pollURL)
	if err != nil {
		return nil, err
	}

	baseURL := &url.URL{Scheme: parsedURL.Scheme, Host: parsedURL.Host}

	client := &hookdeck.Client{
		BaseURL:                 baseURL,
		SuppressRateLimitErrors: true, // Rate limiting is expected during polling
	}

	var count = 0
	currentInterval := interval
	consecutiveRateLimits := 0

	for count < maxAttempts {
		res, err := client.Get(context.TODO(), parsedURL.Path, parsedURL.Query().Encode(), nil)

		// Check if error is due to rate limiting (429)
		if err != nil && isRateLimitError(err) {
			consecutiveRateLimits++
			backoffInterval := calculateBackoff(currentInterval, consecutiveRateLimits)

			log.WithFields(log.Fields{
				"attempt":          count + 1,
				"max_attempts":     maxAttempts,
				"backoff_interval": backoffInterval,
				"rate_limits":      consecutiveRateLimits,
			}).Debug("Rate limited while polling, waiting before retry...")

			time.Sleep(backoffInterval)
			currentInterval = backoffInterval
			count++
			continue
		}

		// Reset back-off on successful request
		if err == nil {
			consecutiveRateLimits = 0
			currentInterval = interval
		}

		// Handle other errors (non-429)
		if err != nil {
			return nil, err
		}

		var response PollAPIKeyResponse

		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, err
		}

		if response.Claimed {
			return &response, nil
		}

		count++
		time.Sleep(currentInterval)
	}

	return nil, errors.New("exceeded max attempts")
}

// isRateLimitError checks if an error is a 429 rate limit error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "429") || strings.Contains(errMsg, "Too Many Requests")
}

// calculateBackoff implements exponential back-off with a maximum cap
func calculateBackoff(baseInterval time.Duration, consecutiveFailures int) time.Duration {
	// Exponential: baseInterval * 2^consecutiveFailures
	backoff := baseInterval * time.Duration(1<<uint(consecutiveFailures))

	// Cap at maxBackoffInterval
	if backoff > maxBackoffInterval {
		backoff = maxBackoffInterval
	}

	return backoff
}
