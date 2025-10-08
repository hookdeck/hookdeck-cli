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
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

const timeLayout = "2006-01-02 15:04:05"
const maxHistorySize = 50     // Maximum events to keep in memory
const maxNavigableEvents = 10 // Only last 10 events are navigable

//
// Public types
//

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
	// Indicates whether to print full JSON objects to stdout
	PrintJSON bool
	Log       *log.Logger
	// Force use of unencrypted ws:// protocol instead of wss://
	NoWSS    bool
	Insecure bool
}

// A Proxy opens a websocket connection with Hookdeck, listens for incoming
// webhook events, forwards them to the local endpoint and sends the response
// back to Hookdeck.
type Proxy struct {
	cfg                  *Config
	connections          []*hookdecksdk.Connection
	webSocketClient      *websocket.Client
	connectionTimer      *time.Timer
	hasReceivedEvent     bool
	stopWaitingAnimation chan bool
	// UI and event management
	ui              *TerminalUI
	eventHistory    *EventHistory
	keyboardHandler *KeyboardHandler
	eventActions    *EventActions
	// Details view
	showingDetails bool // Track if we're in alternate screen showing details
	// Connection state
	isConnected bool // Track if we're currently connected (disable actions during reconnection)
}

func withSIGTERMCancel(ctx context.Context, onCancel func()) context.Context {
	// Create a context that will be canceled when Ctrl+C is pressed
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

// printEventAndUpdateStatus prints the event log and updates the status line in one operation
func (p *Proxy) printEventAndUpdateStatus(eventID string, status int, success bool, eventTime time.Time, eventData *websocket.Attempt, eventLog string, responseStatus int, responseHeaders map[string][]string, responseBody string, responseDuration time.Duration) {
	// Create event info with all data passed as parameters (no shared state)
	eventInfo := EventInfo{
		ID:               eventID,
		Status:           status,
		Success:          success,
		Time:             eventTime,
		Data:             eventData,
		LogLine:          eventLog,
		ResponseStatus:   responseStatus,
		ResponseHeaders:  responseHeaders,
		ResponseBody:     responseBody,
		ResponseDuration: responseDuration,
	}

	// Delegate rendering to UI (pass showingDetails flag to block rendering while less is open)
	p.ui.PrintEventAndUpdateStatus(eventInfo, p.hasReceivedEvent, p.showingDetails)
}

// startWaitingAnimation starts an animation for the waiting indicator
func (p *Proxy) startWaitingAnimation(ctx context.Context) {
	p.stopWaitingAnimation = make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopWaitingAnimation:
				return
			case <-ticker.C:
				if !p.hasReceivedEvent {
					p.ui.UpdateStatusLine(p.hasReceivedEvent)
				}
			}
		}
	}()
}

// quit handles Ctrl+C and q key to exit the application
func (p *Proxy) quit() {
	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(os.Interrupt)
}

// toggleDetailsView shows event details (blocking until user exits less with 'q')
func (p *Proxy) toggleDetailsView() {
	// Set flag BEFORE calling ShowEventDetails, since it blocks until less exits
	p.showingDetails = true

	// Pause keyboard handler to prevent it from processing keypresses meant for less
	p.keyboardHandler.Pause()

	// Ensure cleanup happens after less exits
	defer func() {
		// Drain any buffered input that was meant for less but leaked to our keyboard handler
		p.keyboardHandler.DrainBufferedInput()

		// Resume normal keyboard processing
		p.keyboardHandler.Resume()
		p.showingDetails = false
	}()

	shown, _ := p.eventActions.ShowEventDetails()

	if shown {
		// After less exits, we need to redraw the entire event list
		// because less has taken over the screen
		p.ui.RedrawAfterDetailsView(p.hasReceivedEvent)
	}
}

// navigateEvents moves the selection up or down in the event history (within navigable events)
func (p *Proxy) navigateEvents(direction int) {
	// Delegate to EventHistory and redraw if selection changed
	if p.eventHistory.Navigate(direction) {
		p.ui.RedrawEventsWithSelection(p.hasReceivedEvent)
	}
}

// Run manages the connection to Hookdeck.
// The connection is established in phases:
//   - Create a new CLI session
//   - Create a new websocket connection
func (p *Proxy) Run(parentCtx context.Context) error {
	const maxConnectAttempts = 10
	const maxReconnectAttempts = 10 // Also limit reconnection attempts
	nAttempts := 0

	// Track whether or not we have connected successfully.
	hasConnectedOnce := false
	canConnect := func() bool {
		if hasConnectedOnce {
			// After first successful connection, allow limited reconnection attempts
			return nAttempts < maxReconnectAttempts
		} else {
			// Initial connection attempts
			return nAttempts < maxConnectAttempts
		}
	}

	signalCtx := withSIGTERMCancel(parentCtx, func() {
		log.WithFields(log.Fields{
			"prefix": "proxy.Proxy.Run",
		}).Debug("Ctrl+C received, cleaning up...")
	})

	// Start keyboard listener for keyboard shortcuts
	p.keyboardHandler.Start(signalCtx)

	// Start waiting animation
	p.startWaitingAnimation(signalCtx)

	s := ansi.StartNewSpinner("Getting ready...", p.cfg.Log.Out)

	session, err := p.createSession(signalCtx)
	if err != nil {
		// Stop spinner before fatal error (terminal will be restored by defer)
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		fmt.Print("\033[2K\r")
		p.cfg.Log.Fatalf("Error while authenticating with Hookdeck: %v", err)
	}

	if session.Id == "" {
		// Stop spinner before fatal error (terminal will be restored by defer)
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		fmt.Print("\033[2K\r")
		p.cfg.Log.Fatalf("Error while starting a new session")
	}

	// Main loop to keep attempting to connect to Hookdeck once
	// we have created a session.
	for canConnect() {
		// Apply backoff delay BEFORE creating new client (except for first attempt)
		if nAttempts > 0 {
			// Exponential backoff: 100ms * 2^(attempt-1), capped at 30 seconds
			// Attempt 1: 100ms, 2: 200ms, 3: 400ms, 4: 800ms, 5: 1.6s, 6: 3.2s, 7: 6.4s, 8: 12.8s, 9+: 30s
			backoffMS := math.Min(100*math.Pow(2, float64(nAttempts-1)), 30000)
			sleepDurationMS := int(backoffMS)

			log.WithField(
				"prefix", "proxy.Proxy.Run",
			).Debugf(
				"Connect backoff (%dms)", sleepDurationMS,
			)

			// Reset the timer to the next duration
			p.connectionTimer.Stop()
			p.connectionTimer.Reset(time.Duration(sleepDurationMS) * time.Millisecond)

			// Block with a spinner while waiting
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
			// Use different message based on whether we've connected before
			if hasConnectedOnce {
				s = ansi.StartNewSpinner("Connection lost, reconnecting...", p.cfg.Log.Out)
			} else {
				s = ansi.StartNewSpinner("Connecting...", p.cfg.Log.Out)
			}
			select {
			case <-p.connectionTimer.C:
				// Continue to retry
			case <-signalCtx.Done():
				p.connectionTimer.Stop()
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
				return nil
			}
		}

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

		// Monitor the websocket for connection and update the spinner appropriately.
		go func() {
			<-p.webSocketClient.Connected()
			// Mark as connected and reset attempt counter
			p.isConnected = true
			nAttempts = 0

			// Stop the spinner
			ansi.StopSpinner(s, "", p.cfg.Log.Out)

			// Always update the status line to show current state
			p.ui.UpdateStatusLine(p.hasReceivedEvent)

			hasConnectedOnce = true
		}()

		// Run the websocket in the background
		go p.webSocketClient.Run(signalCtx)
		nAttempts++

		// Block until ctrl+c or the websocket connection is interrupted
		select {
		case <-signalCtx.Done():
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
			return nil
		case <-p.webSocketClient.NotifyExpired:
			// Mark as disconnected
			p.isConnected = false

			if !canConnect() {
				// Stop the spinner before fatal error (terminal will be restored by defer)
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
				fmt.Print("\033[2K\r")

				// Print error without timestamp (use fmt instead of log to avoid formatter)
				color := ansi.Color(os.Stdout)
				fmt.Fprintf(os.Stderr, "%s Could not establish connection. Terminating after %d attempts to connect.\n",
					color.Red("FATAL"), nAttempts)
				os.Exit(1)
			}
			// Connection lost, loop will retry (backoff happens at start of next iteration)
		}
	}

	if p.webSocketClient != nil {
		p.webSocketClient.Stop()
	}

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

	if p.cfg.PrintJSON {
		p.ui.SafePrintf("%s\n", webhookEvent.Body.Request.DataString)
	} else {
		url := p.cfg.URL.Scheme + "://" + p.cfg.URL.Host + p.cfg.URL.Path + webhookEvent.Body.Path
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: p.cfg.Insecure},
		}

		timeout := webhookEvent.Body.Request.Timeout
		if timeout == 0 {
			timeout = 1000 * 30
		}

		client := &http.Client{
			Timeout:   time.Duration(timeout) * time.Millisecond,
			Transport: tr,
		}

		req, err := http.NewRequest(webhookEvent.Body.Request.Method, url, nil)
		if err != nil {
			p.ui.SafePrintf("Error: %s\n", err)
			return
		}
		x := make(map[string]json.RawMessage)
		err = json.Unmarshal(webhookEvent.Body.Request.Headers, &x)
		if err != nil {
			p.ui.SafePrintf("Error: %s\n", err)
			return
		}

		for key, value := range x {
			unquoted_value, _ := strconv.Unquote(string(value))
			req.Header.Set(key, unquoted_value)
		}

		req.Body = ioutil.NopCloser(strings.NewReader(webhookEvent.Body.Request.DataString))
		req.ContentLength = int64(len(webhookEvent.Body.Request.DataString))

		// Track request start time for duration calculation
		requestStartTime := time.Now()

		res, err := client.Do(req)

		if err != nil {
			color := ansi.Color(os.Stdout)
			localTime := time.Now().Format(timeLayout)

			errStr := fmt.Sprintf("%s [%s] Failed to %s: %v",
				color.Faint(localTime),
				color.Red("ERROR").Bold(),
				webhookEvent.Body.Request.Method,
				err,
			)

			// Mark as having received first event
			if !p.hasReceivedEvent {
				p.hasReceivedEvent = true
				// Stop the waiting animation
				if p.stopWaitingAnimation != nil {
					p.stopWaitingAnimation <- true
				}
			}

			// Print the error and update status line with event-specific data (no response data for errors)
			p.printEventAndUpdateStatus(eventID, 0, false, time.Now(), webhookEvent, errStr, 0, nil, "", 0)

			p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
				ErrorAttemptResponse: &websocket.ErrorAttemptResponse{
					Event: "attempt_response",
					Body: websocket.ErrorAttemptBody{
						AttemptId: webhookEvent.Body.AttemptId,
						Error:     true,
					},
				}})
		} else {
			p.processEndpointResponse(webhookEvent, res, requestStartTime)
		}
	}
}

func (p *Proxy) processEndpointResponse(webhookEvent *websocket.Attempt, resp *http.Response, requestStartTime time.Time) {
	eventTime := time.Now()
	localTime := eventTime.Format(timeLayout)
	color := ansi.Color(os.Stdout)
	var url = p.cfg.DashboardBaseURL + "/events/" + webhookEvent.Body.EventID
	if p.cfg.ProjectMode == "console" {
		url = p.cfg.ConsoleBaseURL + "/?event_id=" + webhookEvent.Body.EventID
	}

	// Calculate response duration
	responseDuration := eventTime.Sub(requestStartTime)
	durationMs := responseDuration.Milliseconds()

	outputStr := fmt.Sprintf("%s [%d] %s %s %s %s %s",
		color.Faint(localTime),
		ansi.ColorizeStatus(resp.StatusCode),
		resp.Request.Method,
		resp.Request.URL,
		color.Faint(fmt.Sprintf("(%dms)", durationMs)),
		color.Faint("→"),
		color.Faint(url),
	)

	// Calculate event status
	eventStatus := resp.StatusCode
	eventSuccess := resp.StatusCode >= 200 && resp.StatusCode < 300
	eventID := webhookEvent.Body.EventID

	// Read response body
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errStr := fmt.Sprintf("%s [%s] Failed to read response from endpoint, error = %v\n",
			color.Faint(localTime),
			color.Red("ERROR").Bold(),
			err,
		)
		log.Errorf(errStr)

		return
	}

	// Capture response data
	responseStatus := resp.StatusCode
	responseHeaders := make(map[string][]string)
	for key, values := range resp.Header {
		responseHeaders[key] = values
	}
	responseBody := string(buf)

	// Mark as having received first event
	if !p.hasReceivedEvent {
		p.hasReceivedEvent = true
		// Stop the waiting animation
		if p.stopWaitingAnimation != nil {
			p.stopWaitingAnimation <- true
		}
	}

	// Print the event log and update status line with event-specific data including response
	p.printEventAndUpdateStatus(eventID, eventStatus, eventSuccess, eventTime, webhookEvent, outputStr, responseStatus, responseHeaders, responseBody, responseDuration)

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
func New(cfg *Config, connections []*hookdecksdk.Connection) *Proxy {
	if cfg.Log == nil {
		cfg.Log = &log.Logger{Out: ioutil.Discard}
	}

	eventHistory := NewEventHistory()
	ui := NewTerminalUI(eventHistory)

	p := &Proxy{
		cfg:             cfg,
		connections:     connections,
		connectionTimer: time.NewTimer(0), // Defaults to no delay
		eventHistory:    eventHistory,
		ui:              ui,
	}

	// Create event actions handler
	p.eventActions = NewEventActions(cfg, eventHistory, ui)

	// Create keyboard handler and set up callbacks
	p.keyboardHandler = NewKeyboardHandler(ui, &p.hasReceivedEvent, &p.isConnected, &p.showingDetails)
	p.keyboardHandler.SetCallbacks(
		p.navigateEvents,
		p.eventActions.RetrySelectedEvent,
		p.eventActions.OpenSelectedEventURL,
		p.toggleDetailsView,
		p.quit,
	)

	return p
}
