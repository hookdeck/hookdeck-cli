package hookdeck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/useragent"
)

// DefaultAPIBaseURL is the default base URL for API requests
const DefaultAPIBaseURL = "https://api.hookdeck.com"

// DefaultDashboardURL is the default base URL for web links
const DefaultDashboardURL = "https://dashboard.hookdeck.com"

// DefaultDashboardBaseURL is the default base URL for dashboard requests
const DefaultDashboardBaseURL = "http://dashboard.hookdeck.com"

const DefaultConsoleBaseURL = "http://console.hookdeck.com"

const DefaultWebsocektURL = "wss://ws.hookdeck.com"

// Client is the API client used to sent requests to Hookdeck.
type Client struct {
	// The base URL (protocol + hostname) used for all requests sent by this
	// client.
	BaseURL *url.URL

	// API key used to authenticate requests sent by this client. If left
	// empty, the `Authorization` header will be omitted.
	APIKey string

	// When this is enabled, request and response headers will be printed to
	// stdout.
	Verbose bool

	// Cached HTTP client, lazily created the first time the Client is used to
	// send a request.
	httpClient *http.Client
}

type ErrorResponse struct {
	Handled bool   `json:"Handled"`
	Message string `json:"message"`
}

// PerformRequest sends a request to Hookdeck and returns the response.
func (c *Client) PerformRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req.Header == nil {
		req.Header = http.Header{}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", useragent.GetEncodedUserAgent())
	req.Header.Set("X-Hookdeck-Client-User-Agent", useragent.GetEncodedHookdeckUserAgent())

	if !telemetryOptedOut(os.Getenv("HOOKDECK_CLI_TELEMETRY_OPTOUT")) {
		telemetryHdr, err := getTelemetryHeader()
		if err == nil {
			req.Header.Set("Hookdeck-CLI-Telemetry", telemetryHdr)
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
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	err = checkAndPrintError(resp)
	if err != nil {
		return nil, err
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
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		response := &ErrorResponse{}
		err = json.Unmarshal(body, &response)
		if err != nil {
			// Not a valid JSON response, just use body
			return fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, body)
		}
		if response.Message != "" {
			return fmt.Errorf("error: %s", response.Message)
		}
		return fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, body)
	}
	return nil
}

func postprocessJsonResponse(res *http.Response, target interface{}) (interface{}, error) {
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
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
