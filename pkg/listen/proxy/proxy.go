package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

const timeLayout = "2006-01-02 15:04:05"

// Config provides the configuration of a Proxy
type Config struct {
	// DeviceName is the name of the device sent to Hookdeck to help identify the device
	DeviceName string
	// Key is the API key used to authenticate with Hookdeck
	Key              string
	ProjectID        string
	ProjectMode      string
	URL              *url.URL
	APIBaseURL       string
	DashboardBaseURL string
	ConsoleBaseURL   string
	WSBaseURL        string
	Log              *log.Logger
	// Force use of unencrypted ws:// protocol instead of wss://
	NoWSS    bool
	Insecure bool
	// Output mode: interactive, compact, quiet
	Output   string
	GuestURL string
	// MaxConnections allows tuning the maximum concurrent connections per host.
	// Default: 50 concurrent connections
	// This can be increased for high-volume testing scenarios where the local
	// endpoint can handle more concurrent requests.
	// Example: Set to 100+ when load testing with many parallel webhooks.
	// Warning: Setting this too high may cause resource exhaustion.
	MaxConnections int
	// Filters for this CLI session
	Filters *hookdeck.SessionFilters
}

// A Proxy opens a websocket connection with Hookdeck, listens for incoming
// webhook events, forwards them to the local endpoint and sends the response
// back to Hookdeck.
type Proxy struct {
	cfg             *Config
	connections     []*hookdecksdk.Connection
	webSocketClient *websocket.Client
	connectionTimer *time.Timer
	httpClient      *http.Client
	transport       *http.Transport
	activeRequests  int32
	maxConnWarned   bool // Track if we've warned about connection limit
	renderer        Renderer
}

func withSIGTERMCancel(ctx context.Context, onCancel func()) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interruptCh
		onCancel()
		cancel()
	}()
	return ctx
}

// Run manages the connection to Hookdeck.
// The connection is established in phases:
//   - Create a new CLI session
//   - Create a new websocket connection
func (p *Proxy) Run(parentCtx context.Context) error {
	const maxConnectAttempts = 10
	nAttempts := 0

	// Track whether or not we have connected successfully.
	// Once we have connected we no longer limit the number
	// of connection attempts that will be made and will retry
	// until the connection is successful or the user terminates
	// the program.
	hasConnectedOnce := false
	canConnect := func() bool {
		if hasConnectedOnce {
			return true
		} else {
			return nAttempts < maxConnectAttempts
		}
	}

	signalCtx := withSIGTERMCancel(parentCtx, func() {
		log.WithFields(log.Fields{
			"prefix": "proxy.Proxy.Run",
		}).Debug("Ctrl+C received, cleaning up...")
	})

	// Notify renderer we're connecting
	p.renderer.OnConnecting()

	session, err := p.createSession(signalCtx)
	if err != nil {
		p.renderer.OnError(err)
		p.renderer.Cleanup()
		return fmt.Errorf("error while authenticating with Hookdeck: %v", err)
	}

	if session.Id == "" {
		p.renderer.OnError(fmt.Errorf("error while starting a new session"))
		p.renderer.Cleanup()
		return fmt.Errorf("error while starting a new session")
	}

	// Main loop to keep attempting to connect to Hookdeck once
	// we have created a session.
	for canConnect() {
		p.webSocketClient = websocket.NewClient(
			p.cfg.WSBaseURL,
			session.Id,
			p.cfg.Key,
			p.cfg.ProjectID,
			&websocket.Config{
				Log:          p.cfg.Log,
				NoWSS:        p.cfg.NoWSS,
				EventHandler: websocket.EventHandlerFunc(p.processAttempt),
			},
		)

		// Monitor the websocket for connection
		go func() {
			<-p.webSocketClient.Connected()
			p.renderer.OnConnected()
			hasConnectedOnce = true
		}()

		// Run the websocket in the background
		go p.webSocketClient.Run(signalCtx)
		nAttempts++

		// Block until ctrl+c, renderer quit, or websocket connection is interrupted
		select {
		case <-signalCtx.Done():
			return nil
		case <-p.renderer.Done():
			// Renderer wants to quit (user pressed q or similar)
			if p.webSocketClient != nil {
				p.webSocketClient.Stop()
			}
			p.renderer.Cleanup()
			return nil
		case <-p.webSocketClient.NotifyExpired:
			p.renderer.OnDisconnected()
			if !canConnect() {
				p.renderer.Cleanup()
				return fmt.Errorf("Could not connect. Terminating after %d failed attempts to establish a connection.", nAttempts)
			}
		}

		// Add backoff delay between all retry attempts
		if canConnect() {
			var sleepDurationMS int

			if nAttempts <= maxConnectAttempts {
				// First 10 attempts: use a fixed 2 second delay
				sleepDurationMS = 2000
			} else {
				// After max attempts: exponential backoff, maximum of 10 second intervals
				attemptsOverMax := float64(nAttempts - maxConnectAttempts)
				sleepDurationMS = int(math.Round(math.Min(100, math.Pow(attemptsOverMax, 2)) * 100))
			}

			log.WithField(
				"prefix", "proxy.Proxy.Run",
			).Debugf(
				"Connect backoff (%dms)", sleepDurationMS,
			)

			// Reset the timer to the next duration
			p.connectionTimer.Stop()
			p.connectionTimer.Reset(time.Duration(sleepDurationMS) * time.Millisecond)

			// Block until the timer completes or we get interrupted by the user
			select {
			case <-p.connectionTimer.C:
			case <-signalCtx.Done():
				p.connectionTimer.Stop()
				return nil
			}
		}
	}

	if p.webSocketClient != nil {
		p.webSocketClient.Stop()
	}

	// Clean up renderer
	p.renderer.Cleanup()

	log.WithFields(log.Fields{
		"prefix": "proxy.Proxy.Run",
	}).Debug("Bye!")

	return nil
}

func (p *Proxy) createSession(ctx context.Context) (hookdeck.Session, error) {
	var session hookdeck.Session

	parsedBaseURL, err := url.Parse(p.cfg.APIBaseURL)
	if err != nil {
		return session, err
	}

	client := &hookdeck.Client{
		BaseURL:   parsedBaseURL,
		APIKey:    p.cfg.Key,
		ProjectID: p.cfg.ProjectID,
	}

	var connectionIDs []string
	for _, connection := range p.connections {
		connectionIDs = append(connectionIDs, connection.Id)
	}

	for i := 0; i <= 5; i++ {
		session, err = client.CreateSession(hookdeck.CreateSessionInput{
			ConnectionIds: connectionIDs,
			Filters:       p.cfg.Filters,
		})

		if err == nil {
			return session, nil
		}

		select {
		case <-ctx.Done():
			return session, errors.New("canceled by context")
		case <-time.After(1 * time.Second):
		}
	}

	return session, err
}

func (p *Proxy) processAttempt(msg websocket.IncomingMessage) {
	if msg.Attempt == nil {
		p.cfg.Log.Debug("WebSocket specified for Events received unexpected event")
		return
	}

	webhookEvent := msg.Attempt
	eventID := webhookEvent.Body.EventID

	p.cfg.Log.WithFields(log.Fields{
		"prefix": "proxy.Proxy.processAttempt",
	}).Debugf("Processing webhook event")

	url := p.cfg.URL.Scheme + "://" + p.cfg.URL.Host + p.cfg.URL.Path + webhookEvent.Body.Path

	// Create request with context for timeout control
	timeout := webhookEvent.Body.Request.Timeout
	if timeout == 0 {
		timeout = 1000 * 30
	}

	// Track active requests
	atomic.AddInt32(&p.activeRequests, 1)
	defer atomic.AddInt32(&p.activeRequests, -1)

	activeCount := atomic.LoadInt32(&p.activeRequests)

	// Calculate warning thresholds proportionally to max connections
	maxConns := int32(p.transport.MaxConnsPerHost)
	warningThreshold := int32(float64(maxConns) * 0.8) // Warn at 80% capacity
	resetThreshold := int32(float64(maxConns) * 0.6)   // Reset warning at 60% capacity

	// Warn when approaching connection limit
	if activeCount > warningThreshold && !p.maxConnWarned {
		p.maxConnWarned = true
		p.renderer.OnConnectionWarning(activeCount, p.transport.MaxConnsPerHost)
	} else if activeCount < resetThreshold && p.maxConnWarned {
		// Reset warning flag when load decreases
		p.maxConnWarned = false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, webhookEvent.Body.Request.Method, url, nil)
	if err != nil {
		p.renderer.OnEventError(eventID, webhookEvent, err, time.Now())
		return
	}
	x := make(map[string]json.RawMessage)
	err = json.Unmarshal(webhookEvent.Body.Request.Headers, &x)
	if err != nil {
		p.renderer.OnEventError(eventID, webhookEvent, err, time.Now())
		return
	}

	for key, value := range x {
		unquoted_value, _ := strconv.Unquote(string(value))
		req.Header.Set(key, unquoted_value)
	}

	req.Body = ioutil.NopCloser(strings.NewReader(webhookEvent.Body.Request.DataString))
	req.ContentLength = int64(len(webhookEvent.Body.Request.DataString))

	// For interactive mode: start 100ms timer and HTTP request concurrently
	requestStartTime := time.Now()

	// Channel to receive HTTP response or error
	type httpResult struct {
		res *http.Response
		err error
	}
	responseCh := make(chan httpResult, 1)

	// Make HTTP request in goroutine
	go func() {
		res, err := p.httpClient.Do(req)
		responseCh <- httpResult{res: res, err: err}
	}()

	// For interactive mode, wait 100ms before showing pending event
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

	var eventShown bool
	var result httpResult

	select {
	case result = <-responseCh:
		// Response came back within 100ms - show final event immediately
		timer.Stop()
		if result.err != nil {
			p.renderer.OnEventError(eventID, webhookEvent, result.err, requestStartTime)
			p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
				ErrorAttemptResponse: &websocket.ErrorAttemptResponse{
					Event: "attempt_response",
					Body: websocket.ErrorAttemptBody{
						AttemptId: webhookEvent.Body.AttemptId,
						Error:     true,
					},
				}})
		} else {
			p.processEndpointResponse(eventID, webhookEvent, result.res, requestStartTime)
			result.res.Body.Close()
		}
		return

	case <-timer.C:
		// 100ms passed - show pending event (interactive mode only)
		eventShown = true
		p.renderer.OnEventPending(eventID, webhookEvent, requestStartTime)

		// Wait for HTTP response to complete
		result = <-responseCh
	}

	// If we showed pending event, now handle the final result
	if eventShown {
		if result.err != nil {
			p.renderer.OnEventError(eventID, webhookEvent, result.err, requestStartTime)
			p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
				ErrorAttemptResponse: &websocket.ErrorAttemptResponse{
					Event: "attempt_response",
					Body: websocket.ErrorAttemptBody{
						AttemptId: webhookEvent.Body.AttemptId,
						Error:     true,
					},
				}})
		} else {
			p.processEndpointResponse(eventID, webhookEvent, result.res, requestStartTime)
			result.res.Body.Close()
		}
	}
}

func (p *Proxy) processEndpointResponse(eventID string, webhookEvent *websocket.Attempt, resp *http.Response, requestStartTime time.Time) {
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read response from endpoint, error = %v\n", err)
		return
	}

	// Calculate response duration
	responseDuration := time.Since(requestStartTime)

	// Prepare response headers
	responseHeaders := make(map[string][]string)
	for key, values := range resp.Header {
		responseHeaders[key] = values
	}

	// Call renderer with response data
	p.renderer.OnEventComplete(eventID, webhookEvent, &EventResponse{
		StatusCode: resp.StatusCode,
		Headers:    responseHeaders,
		Body:       string(buf),
		Duration:   responseDuration,
	}, requestStartTime)

	// Send response back to Hookdeck
	if p.webSocketClient != nil {
		p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
			AttemptResponse: &websocket.AttemptResponse{
				Event: "attempt_response",
				Body: websocket.AttemptResponseBody{
					AttemptId: webhookEvent.Body.AttemptId,
					CLIPath:   webhookEvent.Body.Path,
					Status:    resp.StatusCode,
					Data:      string(buf),
				},
			}})
	}
}

//
// Public functions
//

// New creates a new Proxy
func New(cfg *Config, connections []*hookdecksdk.Connection, renderer Renderer) *Proxy {
	if cfg.Log == nil {
		cfg.Log = &log.Logger{Out: ioutil.Discard}
	}

	// Default to 50 connections if not specified
	maxConns := cfg.MaxConnections
	if maxConns <= 0 {
		maxConns = 50
	}

	// Create a shared HTTP transport with connection pooling
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Insecure},
		// Connection pool settings - sensible defaults for typical usage
		MaxIdleConns:        20,               // Total idle connections across all hosts
		MaxIdleConnsPerHost: 10,               // Keep some idle connections for reuse
		IdleConnTimeout:     30 * time.Second, // Clean up idle connections
		DisableKeepAlives:   false,
		// Limit concurrent connections to prevent resource exhaustion
		MaxConnsPerHost:       maxConns, // User-configurable (default: 50)
		ResponseHeaderTimeout: 60 * time.Second,
	}

	p := &Proxy{
		cfg:             cfg,
		connections:     connections,
		connectionTimer: time.NewTimer(0), // Defaults to no delay
		transport:       tr,
		httpClient: &http.Client{
			Transport: tr,
			// Timeout is controlled per-request via context in processAttempt
		},
		renderer: renderer,
	}

	return p
}
