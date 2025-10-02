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
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

const timeLayout = "2006-01-02 15:04:05"

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
	cfg                *Config
	connections        []*hookdecksdk.Connection
	webSocketClient    *websocket.Client
	connectionTimer    *time.Timer
	latestEventID      string
	latestEventStatus  int
	latestEventSuccess bool
	latestEventTime    time.Time
	latestEventData    *websocket.Attempt
	hasReceivedEvent   bool
	statusLineShown    bool
	terminalMutex      sync.Mutex
	rawModeState       *term.State
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

// safePrintf temporarily disables raw mode, prints the message, then re-enables raw mode
func (p *Proxy) safePrintf(format string, args ...interface{}) {
	p.terminalMutex.Lock()
	defer p.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
	}

	// Print the message
	fmt.Printf(format, args...)

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// printEventAndUpdateStatus prints the event log and updates the status line in one operation
func (p *Proxy) printEventAndUpdateStatus(eventLog string) {
	p.terminalMutex.Lock()
	defer p.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
	}

	// If we have a previous status line, clear it and the blank line above it
	if p.statusLineShown {
		fmt.Printf("\033[2A\033[K\033[1B\033[K")
	}

	// Print the event log
	fmt.Printf("%s\n", eventLog)

	// Add a blank line for visual separation
	fmt.Println()

	// Generate and print the new status message
	var statusMsg string
	color := ansi.Color(os.Stdout)
	if p.latestEventSuccess {
		statusMsg = fmt.Sprintf("> %s Last event succeeded with status %d ⌨️ [r] Retry • [o] Open link • [d] Details • [q] Quit",
			color.Green("✓"), p.latestEventStatus)
	} else {
		statusMsg = fmt.Sprintf("> %s Last event failed with status %d ⌨️ [r] Retry • [o] Open link • [d] Details • [q] Quit",
			color.Red("⨯"), p.latestEventStatus)
	}

	fmt.Printf("%s\n", statusMsg)
	p.statusLineShown = true

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// updateStatusLine updates the bottom status line with the latest event information
func (p *Proxy) updateStatusLine() {
	p.terminalMutex.Lock()
	defer p.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
	}

	var statusMsg string
	if !p.hasReceivedEvent {
		statusMsg = "> Ready..."
	} else {
		color := ansi.Color(os.Stdout)
		if p.latestEventSuccess {
			statusMsg = fmt.Sprintf("> %s Last event succeeded (%d) ⌨️ [r] Retry • [o] Open link • [d] Details • [q] Quit",
				color.Green("✓"), p.latestEventStatus)
		} else {
			statusMsg = fmt.Sprintf("> %s Last event failed (%d) ⌨️ [r] Retry • [o] Open link • [d] Details • [q] Quit",
				color.Red("⚠️"), p.latestEventStatus)
		}
	}

	if p.statusLineShown {
		// If we've shown a status before, move up one line and clear it
		fmt.Printf("\033[1A\033[K%s\n", statusMsg)
	} else {
		// First time showing status
		fmt.Printf("%s\n", statusMsg)
		p.statusLineShown = true
	}

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

func (p *Proxy) startKeyboardListener(ctx context.Context) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

	go func() {
		// Enter raw mode once and keep it
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return
		}

		// Store the raw mode state for use in safePrintf
		p.rawModeState = oldState

		// Ensure we restore terminal state when this goroutine exits
		defer func() {
			p.terminalMutex.Lock()
			term.Restore(int(os.Stdin.Fd()), oldState)
			p.terminalMutex.Unlock()
		}()

		// Create a buffered channel for reading stdin
		inputCh := make(chan byte, 1)

		// Start a separate goroutine to read from stdin
		go func() {
			defer close(inputCh)
			buf := make([]byte, 1)
			for {
				select {
				case <-ctx.Done():
					return
				default:
					n, err := os.Stdin.Read(buf)
					if err != nil || n != 1 {
						return
					}
					select {
					case inputCh <- buf[0]:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		// Main loop to process keyboard input
		for {
			select {
			case <-ctx.Done():
				return
			case key, ok := <-inputCh:
				if !ok {
					return
				}

				// Process the key
				switch key {
				case 0x72, 0x52: // 'r' or 'R'
					p.retryLatestEvent()
				case 0x6F, 0x4F: // 'o' or 'O'
					p.openLatestEventURL()
				case 0x64, 0x44: // 'd' or 'D'
					p.showLatestEventDetails()
				case 0x03: // Ctrl+C
					proc, _ := os.FindProcess(os.Getpid())
					proc.Signal(os.Interrupt)
					return
				case 0x71, 0x51: // 'q' or 'Q'
					proc, _ := os.FindProcess(os.Getpid())
					proc.Signal(os.Interrupt)
					return
				}
			}
		}
	}()
}

// Run manages the connection to Hookdeck.
// The connection is established in phases:
//   - Create a new CLI session
//   - Create a new websocket connection
func (p *Proxy) Run(parentCtx context.Context) error {
	const maxConnectAttempts = 3
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

	// Start keyboard listener for Ctrl+R/Cmd+R shortcuts
	p.startKeyboardListener(signalCtx)

	s := ansi.StartNewSpinner("Getting ready...", p.cfg.Log.Out)

	session, err := p.createSession(signalCtx)
	if err != nil {
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		p.cfg.Log.Fatalf("Error while authenticating with Hookdeck: %v", err)
	}

	if session.Id == "" {
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		p.cfg.Log.Fatalf("Error while starting a new session")
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

		// Monitor the websocket for connection and update the spinner appropriately.
		go func() {
			<-p.webSocketClient.Connected()
			if hasConnectedOnce {
				// Stop the spinner and print the message safely
				p.terminalMutex.Lock()
				if p.rawModeState != nil {
					term.Restore(int(os.Stdin.Fd()), p.rawModeState)
				}
				ansi.StopSpinner(s, "Reconnected!", p.cfg.Log.Out)
				if p.rawModeState != nil {
					term.MakeRaw(int(os.Stdin.Fd()))
				}
				p.terminalMutex.Unlock()
			} else {
				// Stop the spinner without a message and use our status line
				p.terminalMutex.Lock()
				if p.rawModeState != nil {
					term.Restore(int(os.Stdin.Fd()), p.rawModeState)
				}
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
				if p.rawModeState != nil {
					term.MakeRaw(int(os.Stdin.Fd()))
				}
				p.terminalMutex.Unlock()
				p.updateStatusLine()
			}
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
			if canConnect() {
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
				s = ansi.StartNewSpinner("Connection lost, reconnecting...", p.cfg.Log.Out)
			} else {
				p.cfg.Log.Fatalf("Session expired. Terminating after %d failed attempts to reauthorize", nAttempts)
			}
		}

		// Determine if we should backoff the connection retries.
		attemptsOverMax := math.Max(0, float64(nAttempts-maxConnectAttempts))
		if canConnect() && attemptsOverMax > 0 {
			// Determine the time to wait to reconnect, maximum of 10 second intervals
			sleepDurationMS := int(math.Round(math.Min(100, math.Pow(attemptsOverMax, 2)) * 100))
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

	// Store the latest event ID and data for retry/open/details functionality
	p.latestEventID = webhookEvent.Body.EventID
	p.latestEventData = webhookEvent

	p.cfg.Log.WithFields(log.Fields{
		"prefix": "proxy.Proxy.processAttempt",
	}).Debugf("Processing webhook event")

	if p.cfg.PrintJSON {
		p.safePrintf("%s\n", webhookEvent.Body.Request.DataString)
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
			p.safePrintf("Error: %s\n", err)
			return
		}
		x := make(map[string]json.RawMessage)
		err = json.Unmarshal(webhookEvent.Body.Request.Headers, &x)
		if err != nil {
			p.safePrintf("Error: %s\n", err)
			return
		}

		for key, value := range x {
			unquoted_value, _ := strconv.Unquote(string(value))
			req.Header.Set(key, unquoted_value)
		}

		req.Body = ioutil.NopCloser(strings.NewReader(webhookEvent.Body.Request.DataString))
		req.ContentLength = int64(len(webhookEvent.Body.Request.DataString))

		res, err := client.Do(req)

		if err != nil {
			color := ansi.Color(os.Stdout)
			localTime := time.Now().Format(timeLayout)

			errStr := fmt.Sprintf("%s [%s] Failed to %s: %v",
				color.Faint(localTime),
				color.Red("ERROR"),
				webhookEvent.Body.Request.Method,
				err,
			)

			// Track the failed event first
			p.latestEventStatus = 0 // Use 0 for connection errors
			p.latestEventSuccess = false
			p.latestEventTime = time.Now()
			p.hasReceivedEvent = true

			// Print the error and update status line in one operation
			p.printEventAndUpdateStatus(errStr)

			p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
				ErrorAttemptResponse: &websocket.ErrorAttemptResponse{
					Event: "attempt_response",
					Body: websocket.ErrorAttemptBody{
						AttemptId: webhookEvent.Body.AttemptId,
						Error:     true,
					},
				}})
		} else {
			p.processEndpointResponse(webhookEvent, res)
		}
	}
}

func (p *Proxy) processEndpointResponse(webhookEvent *websocket.Attempt, resp *http.Response) {
	localTime := time.Now().Format(timeLayout)
	color := ansi.Color(os.Stdout)
	var url = p.cfg.DashboardBaseURL + "/events/" + webhookEvent.Body.EventID
	if p.cfg.ProjectMode == "console" {
		url = p.cfg.ConsoleBaseURL + "/?event_id=" + webhookEvent.Body.EventID
	}
	outputStr := fmt.Sprintf("%s [%d] %s %s %s %s",
		color.Faint(localTime),
		ansi.ColorizeStatus(resp.StatusCode),
		resp.Request.Method,
		resp.Request.URL,
		color.Faint("→"),
		color.Faint(url),
	)
	// Track the event status first
	p.latestEventStatus = resp.StatusCode
	p.latestEventSuccess = resp.StatusCode >= 200 && resp.StatusCode < 300
	p.latestEventTime = time.Now()
	p.hasReceivedEvent = true

	// Print the event log and update status line in one operation
	p.printEventAndUpdateStatus(outputStr)

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errStr := fmt.Sprintf("%s [%s] Failed to read response from endpoint, error = %v\n",
			color.Faint(localTime),
			color.Red("ERROR"),
			err,
		)
		log.Errorf(errStr)

		return
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

func (p *Proxy) retryLatestEvent() {
	eventID := p.latestEventID

	if eventID == "" {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] No event to retry\n",
			color.Yellow("WARN"),
		)
		return
	}

	// Create HTTP client for retry request
	parsedBaseURL, err := url.Parse(p.cfg.APIBaseURL)
	if err != nil {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Failed to parse API URL for retry: %v\n",
			color.Red("ERROR"),
			err,
		)
		return
	}

	client := &hookdeck.Client{
		BaseURL:   parsedBaseURL,
		APIKey:    p.cfg.Key,
		ProjectID: p.cfg.ProjectID,
	}

	// Make retry request to Hookdeck API
	retryURL := fmt.Sprintf("/events/%s/retry", eventID)
	resp, err := client.Post(context.Background(), retryURL, []byte("{}"), nil)
	if err != nil {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Failed to retry event %s\n",
			color.Red("ERROR"),
			eventID,
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Failed to retry event %s\n",
			color.Red("ERROR"),
			eventID,
		)
	}
}

func (p *Proxy) openLatestEventURL() {
	eventID := p.latestEventID

	if eventID == "" {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] No event to open\n",
			color.Yellow("WARN"),
		)
		return
	}

	// Build event URL based on project mode
	var eventURL string
	if p.cfg.ProjectMode == "console" {
		eventURL = p.cfg.ConsoleBaseURL + "/?event_id=" + eventID
	} else {
		eventURL = p.cfg.DashboardBaseURL + "/events/" + eventID
	}

	// Open URL in browser
	err := p.openBrowser(eventURL)
	if err != nil {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Failed to open browser: %v\n",
			color.Red("ERROR"),
			err,
		)
		return
	}
}

func (p *Proxy) openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

func (p *Proxy) showLatestEventDetails() {
	if p.latestEventData == nil {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] No event to show details for\n",
			color.Yellow("WARN"),
		)
		return
	}

	webhookEvent := p.latestEventData

	p.terminalMutex.Lock()
	defer p.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
	}

	// Clear the status line
	if p.statusLineShown {
		fmt.Printf("\033[2A\033[K\033[1B\033[K")
	}

	// Print the event details with title
	color := ansi.Color(os.Stdout)
	fmt.Printf("│  %s %s%s\n", color.Bold(webhookEvent.Body.Request.Method), color.Bold(p.cfg.URL.String()), color.Bold(webhookEvent.Body.Path))
	fmt.Printf("│\n")

	// Parse and display headers (no title)
	if len(webhookEvent.Body.Request.Headers) > 0 {
		var headers map[string]json.RawMessage
		if err := json.Unmarshal(webhookEvent.Body.Request.Headers, &headers); err == nil {
			// Get keys and sort them alphabetically
			keys := make([]string, 0, len(headers))
			for key := range headers {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			// Print headers in alphabetical order
			for _, key := range keys {
				unquoted, _ := strconv.Unquote(string(headers[key]))
				fmt.Printf("│  %s: %s\n", strings.ToLower(key), unquoted)
			}
		}
	}

	// Add blank line before body
	fmt.Printf("│\n")

	// Display body (no title)
	if len(webhookEvent.Body.Request.DataString) > 0 {
		// Split body into lines and add left border to each line
		bodyLines := strings.Split(webhookEvent.Body.Request.DataString, "\n")
		for _, line := range bodyLines {
			fmt.Printf("│  %s\n", line)
		}
	}

	// Restore the status line
	fmt.Println()
	var statusMsg string
	if p.latestEventSuccess {
		statusMsg = fmt.Sprintf("> %s Last event succeeded (%d) ⌨️ [r] Retry now • [o] Open link • [d] Details • [q] Quit",
			color.Green("✓"), p.latestEventStatus)
	} else {
		statusMsg = fmt.Sprintf("> %s Last event failed (%d) ⌨️ [r] Retry now • [o] Open link • [d] Details • [q] Quit",
			color.Red("⚠️"), p.latestEventStatus)
	}
	fmt.Printf("%s\n", statusMsg)

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
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

	p := &Proxy{
		cfg:             cfg,
		connections:     connections,
		connectionTimer: time.NewTimer(0), // Defaults to no delay
	}

	return p
}
