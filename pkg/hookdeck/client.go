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
	"sort"
	"strings"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/useragent"
	log "github.com/sirupsen/logrus"
)

// DefaultAPIBaseURL is the default base URL for API requests
const DefaultAPIBaseURL = "https://api.hookdeck.com"

// DefaultOutpostAPIBaseURL is the default base URL for Hookdeck Outpost API
// requests. Outpost is served from its own host, not from DefaultAPIBaseURL,
// but shares the same calendar-versioned path prefix (APIPathPrefix) and the
// same authentication, so the same Client type serves both.
const DefaultOutpostAPIBaseURL = "https://api.outpost.hookdeck.com"

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

	// AcceptAnySuccessStatus treats any 2xx as success rather than 200 alone.
	//
	// The Event Gateway API answers 200 to every successful request, so the
	// default keeps that stricter check. The Outpost API uses the full range —
	// 201 when a resource is created, 202 when a publish or retry is accepted,
	// 204 on delete — and reporting those as errors would fail every write.
	AcceptAnySuccessStatus bool

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
		AcceptAnySuccessStatus:  c.AcceptAnySuccessStatus,
		Telemetry:               t,
		TelemetryDisabled:       c.TelemetryDisabled,
		httpClient:              c.httpClient,
	}
}

type ErrorResponse struct {
	Handled bool   `json:"Handled"`
	Message string `json:"message"`

	// Data carries per-field detail on a validation failure. Without it a 422
	// surfaces as a bare "validation error", which says nothing about what to
	// change — the Outpost API, for instance, returns
	// {"message":"validation error","data":["topic is invalid"]}.
	Data []string `json:"data,omitempty"`
}

// UnmarshalJSON decodes an error body, accepting every shape "data" is returned
// in rather than only the array of strings a validation failure uses.
//
// This is not cosmetic. The caller falls back to dumping the raw response body
// whenever this decode fails, so a body whose "data" is an object — which is
// how not-found and several rejected-value errors come back — reached the user
// as a wall of JSON with the readable message buried inside it. Being liberal
// here is what turns those into a sentence.
func (e *ErrorResponse) UnmarshalJSON(data []byte) error {
	// A distinct type avoids recursing into this method.
	var raw struct {
		Handled bool            `json:"Handled"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	e.Handled = raw.Handled
	e.Message = raw.Message
	e.Data = flattenErrorData(raw.Data)

	return nil
}

// flattenErrorData renders the "data" member as lines of detail.
func flattenErrorData(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}

	switch value := decoded.(type) {
	case string:
		return []string{value}

	case []interface{}:
		lines := make([]string, 0, len(value))
		for _, item := range value {
			lines = append(lines, errorDataScalar(item))
		}
		return lines

	case map[string]interface{}:
		// A nested message is the whole story; the surrounding keys are
		// bookkeeping, so repeating them would only add noise.
		if message, ok := value["message"].(string); ok && message != "" {
			return []string{message}
		}
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			lines = append(lines, key+": "+errorDataScalar(value[key]))
		}
		return lines

	default:
		return []string{errorDataScalar(decoded)}
	}
}

// errorDataScalar renders one detail value, keeping strings unquoted.
func errorDataScalar(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}

// Detail returns the message with any field-level detail appended.
func (e *ErrorResponse) Detail() string {
	if len(e.Data) == 0 {
		return e.Message
	}
	if e.Message == "" {
		return strings.Join(e.Data, "; ")
	}
	return e.Message + ": " + strings.Join(e.Data, "; ")
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

// IsUnauthorizedError reports whether err is an HTTP 401 from the Hookdeck API
// (invalid or rejected credentials). Non-JSON 401 bodies still become *APIError
// with StatusCode 401; a plain error string containing "status code: 401" is
// treated as unauthorized for wrapped failures.
func IsUnauthorizedError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status code: 401")
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
			"headers": redactHeadersForLog(req.Header),
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
				logFields["body"] = redactRequestBodyForLog(string(bodyBytes))
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

	err = c.checkResponseStatus(resp)
	if err != nil {
		// Allow callers to suppress rate limit error logging for polling scenarios
		if c.SuppressRateLimitErrors && resp.StatusCode == http.StatusTooManyRequests {
			log.WithFields(log.Fields{
				"prefix": "client.Client.PerformRequest",
				"method": req.Method,
				"url":    req.URL.String(),
				"status": resp.StatusCode,
			}).Debug("Rate limited")
		} else if resp.StatusCode == http.StatusUnauthorized {
			// Invalid or expired keys are common; avoid ERROR-level noise (e.g. whoami, agents).
			log.WithFields(log.Fields{
				"prefix": "client.Client.PerformRequest",
				"method": req.Method,
				"url":    req.URL.String(),
				"status": resp.StatusCode,
			}).Debug("Unauthorized response")
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

// ErrRequestPathRejected reports that the URL a request resolved to is not the
// one the caller asked for, or does not address this API at all. It is distinct
// from ErrInvalidResourceID: this is the request layer's last check, and it
// firing means a call site built its path without apiPath.
var ErrRequestPathRejected = errors.New("request path rejected")

// resolveRequestURL resolves a request path against the client's base URL and
// checks the result before anything is sent.
func (c *Client) resolveRequestURL(path string) (*url.URL, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	resolved := c.BaseURL.ResolveReference(ref)

	if err := checkResolvedPath(c.BaseURL, ref, resolved); err != nil {
		return nil, err
	}

	return resolved, nil
}

// checkResolvedPath is the backstop against a path segment that changes which
// resource a request addresses.
//
// Resolving a reference against a base URL normalises "." and ".." segments, so
// an id such as "src_1/../../destinations/des_2" yields a perfectly valid URL
// for a different resource of a different type — while the caller goes on
// reporting the id it was given. Call sites build their paths with apiPath,
// which rejects those values; this check exists so that a call site which does
// not still cannot send the request.
func checkResolvedPath(base, ref, resolved *url.URL) error {
	// The host comes first, because every check below it is about paths and a
	// reference carrying its own authority keeps a path that passes all of them.
	// "//evil.example.com/<prefix>/sources" resolves to a different host with an
	// untouched, correct-looking path — and the request would carry the caller's
	// API key there. Compared against the client's own base rather than an
	// allowlist: this asks "is this the server we were configured to talk to",
	// which stays true for self-hosted and test servers alike.
	if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
		return fmt.Errorf("%w: %q would have been sent to %s://%s rather than %s://%s",
			ErrRequestPathRejected, ref.String(), resolved.Scheme, resolved.Host, base.Scheme, base.Host)
	}

	got := resolved.EscapedPath()

	if got != APIPathPrefix && !strings.HasPrefix(got, APIPathPrefix+"/") {
		return fmt.Errorf("%w: %q does not address the %s API", ErrRequestPathRejected, got, APIPathPrefix)
	}

	// Every path this package sends is built by apiPath, which joins non-empty
	// validated segments. An empty segment therefore cannot come from a correct
	// call site, and "//" is meaningful to some routers, so it is rejected here
	// rather than passed on for a server to interpret.
	for _, segment := range strings.Split(strings.TrimPrefix(got, "/"), "/") {
		if segment == "" {
			return fmt.Errorf("%w: %q contains an empty path segment", ErrRequestPathRejected, got)
		}
	}

	// Any difference between what was asked for and what would be sent means the
	// path was rewritten by reference resolution — "." or ".." normalising into
	// a different resource. Relative references were previously exempt from this
	// comparison, which left the rewrite they are most likely to perform
	// unchecked; they are now resolved against the prefix and compared the same way.
	want := ref.EscapedPath()
	if !strings.HasPrefix(want, "/") {
		want = APIPathPrefix + "/" + want
	}
	if want != got {
		return fmt.Errorf("%w: %q would have been sent as %q", ErrRequestPathRejected, want, got)
	}

	return nil
}

func (c *Client) Get(ctx context.Context, path string, params string, configure func(*http.Request)) (*http.Response, error) {
	url, err := c.resolveRequestURL(path)
	if err != nil {
		return nil, err
	}

	url.RawQuery = params

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}

	return c.PerformRequest(ctx, req)
}

func (c *Client) Post(ctx context.Context, path string, data []byte, configure func(*http.Request)) (*http.Response, error) {
	url, err := c.resolveRequestURL(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return c.PerformRequest(ctx, req)
}

func (c *Client) Put(ctx context.Context, path string, data []byte, configure func(*http.Request)) (*http.Response, error) {
	url, err := c.resolveRequestURL(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, url.String(), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return c.PerformRequest(ctx, req)
}

// checkResponseStatus applies the client's success-status policy. It exists so
// AcceptAnySuccessStatus can widen what counts as success without changing
// checkAndPrintError, which other callers still use directly.
func (c *Client) checkResponseStatus(res *http.Response) error {
	if c.AcceptAnySuccessStatus && res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	return checkAndPrintError(res)
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
		if detail := response.Detail(); detail != "" {
			return &APIError{
				StatusCode: res.StatusCode,
				Message:    detail,
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
