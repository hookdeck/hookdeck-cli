package hookdeck

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const maxAttemptsDefault = 2 * 60
const intervalDefault = 2 * time.Second
const maxBackoffInterval = 30 * time.Second

// ValidateAPIKeyResponse returns the user and team associated with a key
type ValidateAPIKeyResponse struct {
	UserID           string `json:"user_id"`
	UserName         string `json:"user_name"`
	UserEmail        string `json:"user_email"`
	OrganizationName string `json:"organization_name"`
	OrganizationID   string `json:"organization_id"`
	ProjectID        string `json:"team_id"`
	ProjectName      string `json:"team_name_no_org"`
	ProjectMode      string `json:"team_mode"`
	ClientID         string `json:"client_id"`
}

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

// UpdateClientInput represents the input for updating a CLI client
type UpdateClientInput struct {
	DeviceName string `json:"device_name"`
}

// LoginSession represents an in-progress login flow
type LoginSession struct {
	BrowserURL string
	pollURL    string
}

// GuestSession represents an in-progress guest login flow
type GuestSession struct {
	BrowserURL string
	GuestURL   string
	pollURL    string
}

// StartLogin initiates the login flow and returns a session to wait for completion
func (c *Client) StartLogin(deviceName string) (*LoginSession, error) {
	data := struct {
		DeviceName string `json:"device_name"`
	}{
		DeviceName: deviceName,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	res, err := c.Post(context.Background(), APIPathPrefix+"/cli-auth", jsonData, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var links struct {
		BrowserURL string `json:"browser_url"`
		PollURL    string `json:"poll_url"`
	}
	err = json.Unmarshal(body, &links)
	if err != nil {
		return nil, err
	}

	return &LoginSession{
		BrowserURL: links.BrowserURL,
		pollURL:    links.PollURL,
	}, nil
}

// StartGuestLogin initiates a guest login flow and returns a session to wait for completion
func (c *Client) StartGuestLogin(deviceName string) (*GuestSession, error) {
	guest, err := c.CreateGuestUser(CreateGuestUserInput{
		DeviceName: deviceName,
	})
	if err != nil {
		return nil, err
	}

	return &GuestSession{
		BrowserURL: guest.BrowserURL,
		GuestURL:   guest.Url,
		pollURL:    guest.PollURL,
	}, nil
}

// WaitForAPIKey polls until the user completes login and returns the API key response
func (s *LoginSession) WaitForAPIKey(interval time.Duration, maxAttempts int) (*PollAPIKeyResponse, error) {
	return pollForAPIKey(s.pollURL, interval, maxAttempts)
}

// WaitForAPIKey polls until the user completes login and returns the API key response
func (s *GuestSession) WaitForAPIKey(interval time.Duration, maxAttempts int) (*PollAPIKeyResponse, error) {
	return pollForAPIKey(s.pollURL, interval, maxAttempts)
}

// PollForAPIKeyWithKey polls for login completion using a CLI API key (for interactive login)
func (c *Client) PollForAPIKeyWithKey(apiKey string, interval time.Duration, maxAttempts int) (*PollAPIKeyResponse, error) {
	pollURL := c.BaseURL.String() + APIPathPrefix + "/cli-auth/poll?key=" + apiKey
	return pollForAPIKey(pollURL, interval, maxAttempts)
}

// ValidateAPIKey validates an API key and returns user/project information
func (c *Client) ValidateAPIKey() (*ValidateAPIKeyResponse, error) {
	res, err := c.Get(context.Background(), APIPathPrefix+"/cli-auth/validate", "", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var response ValidateAPIKeyResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// pollForAPIKey polls Hookdeck at the specified interval until either the API key is available or we've reached the max attempts.
// This is an internal function that creates its own client for polling with rate limit suppression.
func pollForAPIKey(pollURL string, interval time.Duration, maxAttempts int) (*PollAPIKeyResponse, error) {
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

	client := &Client{
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
		body, err := io.ReadAll(res.Body)
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

// UpdateClient updates a CLI client's device name
func (c *Client) UpdateClient(clientID string, input UpdateClientInput) error {
	jsonData, err := json.Marshal(input)
	if err != nil {
		return err
	}

	_, err = c.Put(context.Background(), APIPathPrefix+"/cli/"+clientID, jsonData, nil)
	return err
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
