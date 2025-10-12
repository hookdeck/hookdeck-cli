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
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/briandowns/spinner"
	tea "github.com/charmbracelet/bubbletea"
	log "github.com/sirupsen/logrus"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/tui"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

// ProxyTUI is a Proxy that uses Bubble Tea for interactive mode
type ProxyTUI struct {
	cfg             *Config
	connections     []*hookdecksdk.Connection
	webSocketClient *websocket.Client
	connectionTimer *time.Timer

	// HTTP client with connection pooling
	httpClient     *http.Client
	transport      *http.Transport
	activeRequests int32 // atomic counter
	maxConnWarned  bool  // Track if we've warned about connection limit

	// Bubble Tea program
	teaProgram *tea.Program
	teaModel   *tui.Model
}

// NewTUI creates a new Proxy with Bubble Tea UI
func NewTUI(cfg *Config, sources []*hookdecksdk.Source, connections []*hookdecksdk.Connection) *ProxyTUI {
	if cfg.Log == nil {
		cfg.Log = &log.Logger{Out: ioutil.Discard}
	}

	// Default to interactive mode if not specified
	if cfg.Output == "" {
		cfg.Output = "interactive"
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
		MaxIdleConns:        20,   // Total idle connections across all hosts
		MaxIdleConnsPerHost: 10,   // Keep some idle connections for reuse
		IdleConnTimeout:     30 * time.Second, // Clean up idle connections
		DisableKeepAlives:   false,
		// Limit concurrent connections to prevent resource exhaustion
		MaxConnsPerHost:       maxConns,  // User-configurable (default: 50)
		ResponseHeaderTimeout: 60 * time.Second,
	}

	p := &ProxyTUI{
		cfg:             cfg,
		connections:     connections,
		connectionTimer: time.NewTimer(0),
		transport:       tr,
		httpClient: &http.Client{
			Transport: tr,
			// Timeout is controlled per-request via context in processAttempt
		},
	}

	// Only create Bubble Tea program for interactive mode
	if cfg.Output == "interactive" {
		tuiCfg := &tui.Config{
			DeviceName:       cfg.DeviceName,
			APIKey:           cfg.Key,
			APIBaseURL:       cfg.APIBaseURL,
			DashboardBaseURL: cfg.DashboardBaseURL,
			ConsoleBaseURL:   cfg.ConsoleBaseURL,
			ProjectMode:      cfg.ProjectMode,
			ProjectID:        cfg.ProjectID,
			GuestURL:         cfg.GuestURL,
			TargetURL:        cfg.URL,
			Sources:          sources,
			Connections:      connections,
		}
		model := tui.NewModel(tuiCfg)
		p.teaModel = &model
		// Use alt screen to keep terminal clean
		p.teaProgram = tea.NewProgram(&model, tea.WithAltScreen())
	}

	return p
}

// Run manages the connection to Hookdeck with Bubble Tea UI
func (p *ProxyTUI) Run(parentCtx context.Context) error {
	const maxConnectAttempts = 10
	const maxReconnectAttempts = 10
	nAttempts := 0

	hasConnectedOnce := false
	canConnect := func() bool {
		if hasConnectedOnce {
			return nAttempts < maxReconnectAttempts
		}
		return nAttempts < maxConnectAttempts
	}

	signalCtx := withSIGTERMCancel(parentCtx, func() {
		log.WithFields(log.Fields{
			"prefix": "proxy.ProxyTUI.Run",
		}).Debug("Ctrl+C received, cleaning up...")
	})

	// Create a channel to signal when TUI exits
	tuiDoneCh := make(chan struct{})

	// Start Bubble Tea program in interactive mode immediately
	if p.cfg.Output == "interactive" && p.teaProgram != nil {
		go func() {
			if _, err := p.teaProgram.Run(); err != nil {
				log.WithField("prefix", "proxy.ProxyTUI.Run").
					Errorf("Bubble Tea error: %v", err)
			}
			// Signal that TUI has exited (user pressed q or Ctrl+C)
			close(tuiDoneCh)
		}()
	}

	// For non-interactive modes, show spinner
	var s *spinner.Spinner
	if p.cfg.Output != "interactive" {
		s = ansi.StartNewSpinner("Getting ready...", p.cfg.Log.Out)
	}

	// Send connecting message to TUI
	if p.teaProgram != nil {
		p.teaProgram.Send(tui.ConnectingMsg{})
	}

	session, err := p.createSession(signalCtx)
	if err != nil {
		if s != nil {
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
		}
		if p.teaProgram != nil {
			p.teaProgram.Kill()
		}
		p.cfg.Log.Fatalf("Error while authenticating with Hookdeck: %v", err)
	}

	if session.Id == "" {
		if s != nil {
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
		}
		if p.teaProgram != nil {
			p.teaProgram.Kill()
		}
		p.cfg.Log.Fatalf("Error while starting a new session")
	}

	// Main connection loop
	for canConnect() {
		// Apply backoff delay
		if nAttempts > 0 {
			backoffMS := math.Min(100*math.Pow(2, float64(nAttempts-1)), 30000)
			sleepDurationMS := int(backoffMS)

			log.WithField("prefix", "proxy.ProxyTUI.Run").
				Debugf("Connect backoff (%dms)", sleepDurationMS)

			p.connectionTimer.Stop()
			p.connectionTimer.Reset(time.Duration(sleepDurationMS) * time.Millisecond)

			// For non-interactive modes, update spinner
			if s != nil {
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
				if hasConnectedOnce {
					s = ansi.StartNewSpinner("Connection lost, reconnecting...", p.cfg.Log.Out)
				} else {
					s = ansi.StartNewSpinner("Connecting...", p.cfg.Log.Out)
				}
			}

			// For interactive mode, send reconnecting message to TUI
			if p.teaProgram != nil {
				if hasConnectedOnce {
					p.teaProgram.Send(tui.DisconnectedMsg{})
				} else {
					p.teaProgram.Send(tui.ConnectingMsg{})
				}
			}

			select {
			case <-p.connectionTimer.C:
				// Continue to retry
			case <-signalCtx.Done():
				p.connectionTimer.Stop()
				if s != nil {
					ansi.StopSpinner(s, "", p.cfg.Log.Out)
				}
				if p.teaProgram != nil {
					p.teaProgram.Kill()
				}
				return nil
			case <-tuiDoneCh:
				// TUI exited during backoff
				p.connectionTimer.Stop()
				if s != nil {
					ansi.StopSpinner(s, "", p.cfg.Log.Out)
				}
				if p.webSocketClient != nil {
					p.webSocketClient.Stop()
				}
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

		// Monitor websocket connection
		go func() {
			<-p.webSocketClient.Connected()
			nAttempts = 0

			// For non-interactive modes, stop spinner and show message
			if s != nil {
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
				color := ansi.Color(os.Stdout)
				fmt.Printf("%s\n\n", color.Faint("Connected. Waiting for events..."))
			}

			// Send connected message to TUI
			if p.teaProgram != nil {
				p.teaProgram.Send(tui.ConnectedMsg{})
			}

			hasConnectedOnce = true
		}()

		// Run websocket
		go p.webSocketClient.Run(signalCtx)
		nAttempts++

		// Block until ctrl+c, TUI exits, or connection lost
		select {
		case <-signalCtx.Done():
			if s != nil {
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
			}
			if p.teaProgram != nil {
				p.teaProgram.Kill()
			}
			return nil
		case <-tuiDoneCh:
			// TUI exited (user pressed q or Ctrl+C in TUI)
			if s != nil {
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
			}
			if p.webSocketClient != nil {
				p.webSocketClient.Stop()
			}
			return nil
		case <-p.webSocketClient.NotifyExpired:
			// Send disconnected message
			if p.teaProgram != nil {
				p.teaProgram.Send(tui.DisconnectedMsg{})
			}

			if !canConnect() {
				if s != nil {
					ansi.StopSpinner(s, "", p.cfg.Log.Out)
				}
				if p.teaProgram != nil {
					p.teaProgram.Quit()
					// Wait a moment for TUI to clean up properly
					select {
					case <-tuiDoneCh:
						// TUI exited cleanly
					case <-time.After(100 * time.Millisecond):
						// Timeout, force kill
						p.teaProgram.Kill()
					}
				}

				return fmt.Errorf("Could not establish connection. Terminating after %d attempts to connect", nAttempts)
			}
		}
	}

	if p.webSocketClient != nil {
		p.webSocketClient.Stop()
	}

	if p.teaProgram != nil {
		p.teaProgram.Kill()
	}

	log.WithFields(log.Fields{
		"prefix": "proxy.ProxyTUI.Run",
	}).Debug("Bye!")

	return nil
}

func (p *ProxyTUI) createSession(ctx context.Context) (hookdeck.Session, error) {
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

func (p *ProxyTUI) processAttempt(msg websocket.IncomingMessage) {
	if msg.Attempt == nil {
		p.cfg.Log.Debug("WebSocket specified for Events received unexpected event")
		return
	}

	webhookEvent := msg.Attempt
	eventID := webhookEvent.Body.EventID

	p.cfg.Log.WithFields(log.Fields{
		"prefix": "proxy.ProxyTUI.processAttempt",
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
		color := ansi.Color(os.Stdout)
		fmt.Printf("\n%s High connection load detected (%d active requests)\n",
			color.Yellow("⚠ WARNING:"), activeCount)
		fmt.Printf("  The CLI is limited to %d concurrent connections per host.\n", p.transport.MaxConnsPerHost)
		fmt.Printf("  Consider reducing request rate or increasing connection limit.\n")
		fmt.Printf("  Run with --max-connections=%d to increase the limit.\n\n", maxConns*2)
	} else if activeCount < resetThreshold && p.maxConnWarned {
		// Reset warning flag when load decreases
		p.maxConnWarned = false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, webhookEvent.Body.Request.Method, url, nil)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	x := make(map[string]json.RawMessage)
	err = json.Unmarshal(webhookEvent.Body.Request.Headers, &x)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	for key, value := range x {
		unquoted_value, _ := strconv.Unquote(string(value))
		req.Header.Set(key, unquoted_value)
	}

	req.Body = ioutil.NopCloser(strings.NewReader(webhookEvent.Body.Request.DataString))
	req.ContentLength = int64(len(webhookEvent.Body.Request.DataString))

	// Start 100ms timer and HTTP request concurrently
	requestStartTime := time.Now()
	eventTime := requestStartTime

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

	// Wait for either 100ms to pass or HTTP response to arrive
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

	var eventShown bool
	var result httpResult

	select {
	case result = <-responseCh:
		// Response came back within 100ms - show final event immediately
		timer.Stop()
		if result.err != nil {
			p.handleRequestError(eventID, webhookEvent, result.err)
		} else {
			p.processEndpointResponse(webhookEvent, result.res, requestStartTime)
		}
		return

	case <-timer.C:
		// 100ms passed - show pending event
		eventShown = true
		p.showPendingEvent(eventID, webhookEvent, eventTime)

		// Wait for HTTP response to complete
		result = <-responseCh
	}

	// If we showed pending event, now update it with final result
	if eventShown {
		if result.err != nil {
			p.updateEventWithError(eventID, webhookEvent, result.err, eventTime)
		} else {
			p.updateEventWithResponse(eventID, webhookEvent, result.res, requestStartTime, eventTime)
		}
	}
}

func (p *ProxyTUI) showPendingEvent(eventID string, webhookEvent *websocket.Attempt, eventTime time.Time) {
	color := ansi.Color(os.Stdout)
	localTime := eventTime.Format(timeLayout)

	pendingStr := fmt.Sprintf("%s [%s] %s %s %s",
		color.Faint(localTime),
		color.Faint("..."),
		webhookEvent.Body.Request.Method,
		fmt.Sprintf("http://localhost%s", webhookEvent.Body.Path),
		color.Faint("(Waiting for response)"),
	)

	// Send pending event to UI
	event := tui.EventInfo{
		ID:               eventID,
		AttemptID:        webhookEvent.Body.AttemptId,
		Status:           0,
		Success:          false,
		Time:             eventTime,
		Data:             webhookEvent,
		LogLine:          pendingStr,
		ResponseStatus:   0,
		ResponseDuration: 0,
	}

	switch p.cfg.Output {
	case "interactive":
		if p.teaProgram != nil {
			p.teaProgram.Send(tui.NewEventMsg{Event: event})
		}
	case "compact":
		fmt.Println(pendingStr)
	case "quiet":
		// Don't show pending events in quiet mode
	}
}

func (p *ProxyTUI) updateEventWithError(eventID string, webhookEvent *websocket.Attempt, err error, eventTime time.Time) {
	color := ansi.Color(os.Stdout)
	localTime := eventTime.Format(timeLayout)

	errStr := fmt.Sprintf("%s [%s] Failed to %s: %v",
		color.Faint(localTime),
		color.Red("ERROR").Bold(),
		webhookEvent.Body.Request.Method,
		err,
	)

	// Update event in UI
	switch p.cfg.Output {
	case "interactive":
		if p.teaProgram != nil {
			p.teaProgram.Send(tui.UpdateEventMsg{
				EventID:          eventID,
				Time:             eventTime,
				Status:           0,
				Success:          false,
				LogLine:          errStr,
				ResponseStatus:   0,
				ResponseHeaders:  nil,
				ResponseBody:     "",
				ResponseDuration: 0,
			})
		}
	case "compact":
		fmt.Println(errStr)
	case "quiet":
		fmt.Println(errStr)
	}

	p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
		ErrorAttemptResponse: &websocket.ErrorAttemptResponse{
			Event: "attempt_response",
			Body: websocket.ErrorAttemptBody{
				AttemptId: webhookEvent.Body.AttemptId,
				Error:     true,
			},
		}})
}

func (p *ProxyTUI) updateEventWithResponse(eventID string, webhookEvent *websocket.Attempt, resp *http.Response, requestStartTime time.Time, eventTime time.Time) {
	localTime := eventTime.Format(timeLayout)
	color := ansi.Color(os.Stdout)

	// Build display URL (without team_id for cleaner display)
	var displayURL string
	if p.cfg.ProjectMode == "console" {
		displayURL = p.cfg.ConsoleBaseURL + "/?event_id=" + webhookEvent.Body.EventID
	} else {
		displayURL = p.cfg.DashboardBaseURL + "/events/" + webhookEvent.Body.EventID
	}

	responseDuration := eventTime.Sub(requestStartTime)
	durationMs := responseDuration.Milliseconds()

	outputStr := fmt.Sprintf("%s [%d] %s %s %s %s %s",
		color.Faint(localTime),
		ansi.ColorizeStatus(resp.StatusCode),
		resp.Request.Method,
		resp.Request.URL,
		color.Faint(fmt.Sprintf("(%dms)", durationMs)),
		color.Faint("→"),
		color.Faint(displayURL),
	)

	eventStatus := resp.StatusCode
	eventSuccess := resp.StatusCode >= 200 && resp.StatusCode < 300

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errStr := fmt.Sprintf("%s [%s] Failed to read response from endpoint, error = %v\n",
			color.Faint(localTime),
			color.Red("ERROR").Bold(),
			err,
		)
		log.Errorf(errStr)
		resp.Body.Close()
		return
	}

	// Close the body - connection can be reused since body was fully read
	resp.Body.Close()

	responseHeaders := make(map[string][]string)
	for key, values := range resp.Header {
		responseHeaders[key] = values
	}
	responseBody := string(buf)

	// Update event in UI
	switch p.cfg.Output {
	case "interactive":
		if p.teaProgram != nil {
			p.teaProgram.Send(tui.UpdateEventMsg{
				EventID:          eventID,
				Time:             eventTime,
				Status:           eventStatus,
				Success:          eventSuccess,
				LogLine:          outputStr,
				ResponseStatus:   eventStatus,
				ResponseHeaders:  responseHeaders,
				ResponseBody:     responseBody,
				ResponseDuration: responseDuration,
			})
		}
	case "compact":
		fmt.Println(outputStr)
	case "quiet":
		// Only print fatal errors
		if !eventSuccess && eventStatus == 0 {
			fmt.Println(outputStr)
		}
	}

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

func (p *ProxyTUI) handleRequestError(eventID string, webhookEvent *websocket.Attempt, err error) {
	color := ansi.Color(os.Stdout)
	localTime := time.Now().Format(timeLayout)

	errStr := fmt.Sprintf("%s [%s] Failed to %s: %v",
		color.Faint(localTime),
		color.Red("ERROR").Bold(),
		webhookEvent.Body.Request.Method,
		err,
	)

	// Send event to UI
	event := tui.EventInfo{
		ID:               eventID,
		AttemptID:        webhookEvent.Body.AttemptId,
		Status:           0,
		Success:          false,
		Time:             time.Now(),
		Data:             webhookEvent,
		LogLine:          errStr,
		ResponseStatus:   0,
		ResponseDuration: 0,
	}

	switch p.cfg.Output {
	case "interactive":
		if p.teaProgram != nil {
			p.teaProgram.Send(tui.NewEventMsg{Event: event})
		}
	case "compact":
		fmt.Println(errStr)
	case "quiet":
		fmt.Println(errStr)
	}

	p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
		ErrorAttemptResponse: &websocket.ErrorAttemptResponse{
			Event: "attempt_response",
			Body: websocket.ErrorAttemptBody{
				AttemptId: webhookEvent.Body.AttemptId,
				Error:     true,
			},
		}})
}

func (p *ProxyTUI) processEndpointResponse(webhookEvent *websocket.Attempt, resp *http.Response, requestStartTime time.Time) {
	eventTime := time.Now()
	localTime := eventTime.Format(timeLayout)
	color := ansi.Color(os.Stdout)

	// Build display URL (without team_id for cleaner display)
	var displayURL string
	if p.cfg.ProjectMode == "console" {
		displayURL = p.cfg.ConsoleBaseURL + "/?event_id=" + webhookEvent.Body.EventID
	} else {
		displayURL = p.cfg.DashboardBaseURL + "/events/" + webhookEvent.Body.EventID
	}

	responseDuration := eventTime.Sub(requestStartTime)
	durationMs := responseDuration.Milliseconds()

	outputStr := fmt.Sprintf("%s [%d] %s %s %s %s %s",
		color.Faint(localTime),
		ansi.ColorizeStatus(resp.StatusCode),
		resp.Request.Method,
		resp.Request.URL,
		color.Faint(fmt.Sprintf("(%dms)", durationMs)),
		color.Faint("→"),
		color.Faint(displayURL),
	)

	eventStatus := resp.StatusCode
	eventSuccess := resp.StatusCode >= 200 && resp.StatusCode < 300
	eventID := webhookEvent.Body.EventID

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errStr := fmt.Sprintf("%s [%s] Failed to read response from endpoint, error = %v\n",
			color.Faint(localTime),
			color.Red("ERROR").Bold(),
			err,
		)
		log.Errorf(errStr)
		resp.Body.Close()
		return
	}

	// Close the body - connection can be reused since body was fully read
	resp.Body.Close()

	responseHeaders := make(map[string][]string)
	for key, values := range resp.Header {
		responseHeaders[key] = values
	}
	responseBody := string(buf)

	// Send event to UI
	event := tui.EventInfo{
		ID:               eventID,
		AttemptID:        webhookEvent.Body.AttemptId,
		Status:           eventStatus,
		Success:          eventSuccess,
		Time:             eventTime,
		Data:             webhookEvent,
		LogLine:          outputStr,
		ResponseStatus:   eventStatus,
		ResponseHeaders:  responseHeaders,
		ResponseBody:     responseBody,
		ResponseDuration: responseDuration,
	}

	switch p.cfg.Output {
	case "interactive":
		if p.teaProgram != nil {
			p.teaProgram.Send(tui.NewEventMsg{Event: event})
		}
	case "compact":
		fmt.Println(outputStr)
	case "quiet":
		// Only print fatal errors
		if !eventSuccess && eventStatus == 0 {
			fmt.Println(outputStr)
		}
	}

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
