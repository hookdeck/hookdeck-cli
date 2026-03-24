package hookdeck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/useragent"
	log "github.com/sirupsen/logrus"
)

// DefaultAPIBaseURL is the default base URL for API requests
const DefaultAPIBaseURL = "https://api.hookdeck.com"

// DefaultDashboardURL is the default base URL for web links
const DefaultDashboardURL = "https://dashboard.hookdeck.com"

// DefaultDashboardBaseURL is the default base URL for dashboard requests
const DefaultDashboardBaseURL = "https://dashboard.hookdeck.com"

const DefaultConsoleBaseURL = "https://console.hookdeck.com"

const DefaultWebsocektURL = "wss://ws.hookdeck.com"

const DefaultProfileName = "default"

// APIPathPrefix is the versioned path prefix for all REST API requests.
// Used by connections, sources, destinations, events, auth, etc.
// Change in one place when the API version is updated.
const APIPathPrefix = "/2025-07-01"

// Client is the API client used to sent requests to Hookdeck.
type Client struct {
	// The base URL (protocol + hostname) used for all requests sent by this
	// client.
	BaseURL *url.URL

	// API key used to authenticate requests sent by this client. If left
	// empty, the `Authorization` header will be omitted.
	APIKey string

	ProjectID string

	// ProjectOrg is the organization segment for the active project (MCP meta),
	// when applicable. Not sent on API requests.
	ProjectOrg string

	// ProjectName is the short project name (not including org). Used for MCP
	// meta and display composition with ProjectOrg. Not sent on API requests.
	ProjectName string

	// When this is enabled, request and response headers will be printed to
	// stdout.
	Verbose bool

	// When this is enabled, HTTP 429 (rate limit) errors will be logged at
	// DEBUG level instead of ERROR level. Useful for polling scenarios where
	// rate limiting is expected.
	SuppressRateLimitErrors bool

	// Per-request telemetry override. When non-nil, this is used instead of
	// the global telemetry singleton. Used by MCP tool handlers to set
	// per-invocation context.
	Telemetry *CLITelemetry

	// TelemetryDisabled mirrors the config-based telemetry opt-out flag.
	TelemetryDisabled bool

	// Cached HTTP client, lazily created the first time the Client is used to
	// send a request.
	httpClient *http.Client
}

// WithTelemetry returns a shallow clone of the client with the given
// per-request telemetry override. The underlying http.Client (and its
// connection pool) is shared.
func (c *Client) WithTelemetry(t *CLITelemetry) *Client {
	return &Client{
		BaseURL:                 c.BaseURL,
		APIKey:                  c.APIKey,
		ProjectID:               c.ProjectID,
		ProjectOrg:              c.ProjectOrg,
		ProjectName:             c.ProjectName,
		Verbose:                 c.Verbose,
		SuppressRateLimitErrors: c.SuppressRateLimitErrors,
		Telemetry:               t,
		TelemetryDisabled:       c.TelemetryDisabled,
		httpClient:              c.httpClient,
	}
}

type ErrorResponse struct {
	Handled bool   `json:"Handled"`
	Message string `json:"message"`
}

// APIError is a structured error returned by the Hookdeck API.
// It preserves the HTTP status code so callers can distinguish
// between different error types (e.g. 404 Not Found vs 500 Server Error)
// without resorting to string matching.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("error: %s", e.Message)
	}
	return fmt.Sprintf("unexpected http status code: %d", e.StatusCode)
}

// IsNotFoundError reports whether the error is an API "not found" response.
// Hookdeck may return 404 (Not Found) or 410 (Gone) for resources that have
// been deleted.
func IsNotFoundError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusGone)
}

// PerformRequest sends a request to Hookdeck and returns the response.
func (c *Client) PerformRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req.Header == nil {
		req.Header = http.Header{}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", useragent.GetEncodedUserAgent())
	req.Header.Set("X-Hookdeck-Client-User-Agent", useragent.GetEncodedHookdeckUserAgent())

	if c.ProjectID != "" {
		req.Header.Set("X-Team-ID", c.ProjectID)
		req.Header.Set("X-Project-ID", c.ProjectID)
	}

	singletonDisabled := GetTelemetryInstance().Disabled
	if !telemetryOptedOut(os.Getenv("HOOKDECK_CLI_TELEMETRY_DISABLED"), c.TelemetryDisabled || singletonDisabled) {
		var telemetryHdr string
		var telErr error
		if c.Telemetry != nil {
			b, e := json.Marshal(c.Telemetry)
			telemetryHdr, telErr = string(b), e
		} else {
			telemetryHdr, telErr = getTelemetryHeader()
		}
		if telErr == nil {
			req.Header.Set(TelemetryHeaderName, telemetryHdr)
		}
	}

	if c.APIKey != "" {
		req.SetBasicAuth(c.APIKey, "")
	}

	if c.httpClient == nil {
		c.httpClient = newHTTPClient(c.Verbose, os.Getenv("HOOKDECK_CLI_UNIX_SOCKET"))
	}

	if ctx != nil {
		req = req.WithContext(ctx)
		logFields := log.Fields{
			"prefix":  "client.Client.PerformRequest",
			"method":  req.Method,
			"url":     req.URL.String(),
			"headers": req.Header,
		}

		if req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				// Log the error and potentially return or handle it
				log.WithFields(logFields).WithError(err).Error("Failed to read request body")
				// Depending on desired behavior, you might want to return an error here
				// or proceed without the body in logFields.
				// For now, just log and continue.
			} else {
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				logFields["body"] = string(bodyBytes)
			}
		}
		log.WithFields(logFields).Debug("Performing request")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"prefix": "client.Client.PerformRequest 1",
			"method": req.Method,
			"url":    req.URL.String(),
			"error":  err.Error(),
		}).Error("Failed to perform request")
		return nil, err
	}

	err = checkAndPrintError(resp)
	if err != nil {
		// Allow callers to suppress rate limit error logging for polling scenarios
		if c.SuppressRateLimitErrors && resp.StatusCode == http.StatusTooManyRequests {
			log.WithFields(log.Fields{
				"prefix": "client.Client.PerformRequest",
				"method": req.Method,
				"url":    req.URL.String(),
				"status": resp.StatusCode,
			}).Debug("Rate limited")
		} else {
			log.WithFields(log.Fields{
				"prefix": "client.Client.PerformRequest 2",
				"method": req.Method,
				"url":    req.URL.String(),
				"error":  err.Error(),
				"status": resp.StatusCode,
			}).Error("Unexpected response")
		}
		return nil, err
	}

	if ctx != nil {
		logFields := log.Fields{
			"prefix":     "client.Client.PerformRequest",
			"statusCode": resp.StatusCode,
			"headers":    resp.Header,
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			logFields["body"] = string(bodyBytes)
		}

		log.WithFields(logFields).Debug("Received response")
	}

	return resp, nil
}

func (c *Client) Get(ctx context.Context, path string, params string, configure func(*http.Request)) (*http.Response, error) {
	url, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	url = c.BaseURL.ResolveReference(url)

	url.RawQuery = params

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}

	return c.PerformRequest(ctx, req)
}

func (c *Client) Post(ctx context.Context, path string, data []byte, configure func(*http.Request)) (*http.Response, error) {
	url, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	url = c.BaseURL.ResolveReference(url)
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return c.PerformRequest(ctx, req)
}

func (c *Client) Put(ctx context.Context, path string, data []byte, configure func(*http.Request)) (*http.Response, error) {
	url, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	url = c.BaseURL.ResolveReference(url)
	req, err := http.NewRequest(http.MethodPut, url.String(), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return c.PerformRequest(ctx, req)
}

func checkAndPrintError(res *http.Response) error {
	if res.StatusCode != http.StatusOK {
		if res.Body != nil {
			defer res.Body.Close()
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		response := &ErrorResponse{}
		err = json.Unmarshal(body, &response)
		if err != nil {
			// Not a valid JSON response, return structured error with raw body
			return &APIError{
				StatusCode: res.StatusCode,
				Message:    fmt.Sprintf("unexpected http status code: %d, raw response body: %s", res.StatusCode, body),
			}
		}
		if response.Message != "" {
			return &APIError{
				StatusCode: res.StatusCode,
				Message:    response.Message,
			}
		}
		return &APIError{
			StatusCode: res.StatusCode,
			Message:    fmt.Sprintf("unexpected http status code: %d %s", res.StatusCode, body),
		}
	}
	return nil
}

func postprocessJsonResponse(res *http.Response, target interface{}) (interface{}, error) {
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, target)
	return target, err
}

func newHTTPClient(verbose bool, unixSocket string) *http.Client {
	var httpTransport *http.Transport

	if unixSocket != "" {
		dialFunc := func(network, addr string) (net.Conn, error) {
			return net.Dial("unix", unixSocket)
		}
		dialContext := func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", unixSocket)
		}
		httpTransport = &http.Transport{
			DialContext:           dialContext,
			DialTLS:               dialFunc,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
		}
	} else {
		httpTransport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		}
	}

	tr := &verboseTransport{
		Transport: httpTransport,
		Verbose:   verbose,
		Out:       os.Stderr,
	}

	return &http.Client{
		Transport: tr,
	}
}
