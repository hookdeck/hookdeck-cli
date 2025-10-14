package proxy

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	log "github.com/sirupsen/logrus"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

const simpleTimeLayout = "2006-01-02 15:04:05"

// SimpleRenderer renders events to stdout for compact and quiet modes
type SimpleRenderer struct {
	cfg            *RendererConfig
	quietMode      bool
	doneCh         chan struct{}
	spinner        *spinner.Spinner
	hasConnected   bool // Track if we've successfully connected at least once
	isReconnecting bool // Track if we're currently in reconnection mode
}

// NewSimpleRenderer creates a new simple renderer
func NewSimpleRenderer(cfg *RendererConfig, quietMode bool) *SimpleRenderer {
	return &SimpleRenderer{
		cfg:       cfg,
		quietMode: quietMode,
		doneCh:    make(chan struct{}),
	}
}

// OnConnecting is called when starting to connect
func (r *SimpleRenderer) OnConnecting() {
	r.spinner = ansi.StartNewSpinner("Getting ready...", log.StandardLogger().Out)
}

// OnConnected is called when websocket connects
func (r *SimpleRenderer) OnConnected() {
	r.hasConnected = true
	r.isReconnecting = false // Reset reconnection state
	if r.spinner != nil {
		ansi.StopSpinner(r.spinner, "", log.StandardLogger().Out)
		r.spinner = nil
		color := ansi.Color(os.Stdout)

		// Display filter warning if filters are active
		if r.cfg.Filters != nil {
			fmt.Printf("\n%s Filters provided, only events matching the filter will be forwarded for this session\n", color.Yellow("⏺"))
			if r.cfg.Filters.Body != nil {
				fmt.Printf("  • Body: %s\n", color.Faint(string(*r.cfg.Filters.Body)))
			}
			if r.cfg.Filters.Headers != nil {
				fmt.Printf("  • Headers: %s\n", color.Faint(string(*r.cfg.Filters.Headers)))
			}
			if r.cfg.Filters.Query != nil {
				fmt.Printf("  • Query: %s\n", color.Faint(string(*r.cfg.Filters.Query)))
			}
			if r.cfg.Filters.Path != nil {
				fmt.Printf("  • Path: %s\n", color.Faint(string(*r.cfg.Filters.Path)))
			}
			fmt.Println()
		}

		fmt.Printf("%s\n\n", color.Faint("Connected. Waiting for events..."))
	}
}

// OnDisconnected is called when websocket disconnects
func (r *SimpleRenderer) OnDisconnected() {
	// Only show "Connection lost" if we've successfully connected before
	if r.hasConnected && !r.isReconnecting {
		// First disconnection - print newline for visual separation
		fmt.Println()
		// Stop any existing spinner first
		if r.spinner != nil {
			ansi.StopSpinner(r.spinner, "", log.StandardLogger().Out)
		}
		// Start new spinner with reconnection message
		r.spinner = ansi.StartNewSpinner("Connection lost, reconnecting...", log.StandardLogger().Out)
		r.isReconnecting = true
	}
	// If we haven't connected yet, the "Getting ready..." spinner is still showing
	// If already reconnecting, the spinner is already showing
}

// OnError is called when an error occurs
func (r *SimpleRenderer) OnError(err error) {
	color := ansi.Color(os.Stdout)
	fmt.Printf("%s %v\n", color.Red("ERROR:"), err)
}

// OnEventPending is called when an event starts (not used in simple renderer)
func (r *SimpleRenderer) OnEventPending(eventID string, attempt *websocket.Attempt, startTime time.Time) {
	// Simple renderer doesn't show pending events
}

// OnEventComplete is called when an event completes successfully
func (r *SimpleRenderer) OnEventComplete(eventID string, attempt *websocket.Attempt, response *EventResponse, startTime time.Time) {
	localTime := time.Now().Format(simpleTimeLayout)
	color := ansi.Color(os.Stdout)

	// Build display URL
	var displayURL string
	if r.cfg.ProjectMode == "console" {
		displayURL = r.cfg.ConsoleBaseURL + "/?event_id=" + eventID
	} else {
		displayURL = r.cfg.DashboardBaseURL + "/events/" + eventID
	}

	durationMs := response.Duration.Milliseconds()

	outputStr := fmt.Sprintf("%s [%d] %s %s %s %s %s",
		color.Faint(localTime),
		ansi.ColorizeStatus(response.StatusCode),
		attempt.Body.Request.Method,
		r.cfg.TargetURL.Scheme+"://"+r.cfg.TargetURL.Host+r.cfg.TargetURL.Path+attempt.Body.Path,
		color.Faint(fmt.Sprintf("(%dms)", durationMs)),
		color.Faint("→"),
		color.Faint(displayURL),
	)

	// In quiet mode, only print fatal errors
	if r.quietMode {
		// Only show if it's a fatal error (status 0 means connection error)
		if response.StatusCode == 0 {
			fmt.Println(outputStr)
		}
	} else {
		// Compact mode: print everything
		fmt.Println(outputStr)
	}
}

// OnEventError is called when an event encounters an error
func (r *SimpleRenderer) OnEventError(eventID string, attempt *websocket.Attempt, err error, startTime time.Time) {
	color := ansi.Color(os.Stdout)
	localTime := time.Now().Format(simpleTimeLayout)

	errStr := fmt.Sprintf("%s [%s] Failed to %s: %v",
		color.Faint(localTime),
		color.Red("ERROR").Bold(),
		attempt.Body.Request.Method,
		err,
	)

	// Always print errors (both compact and quiet modes show errors)
	fmt.Println(errStr)
}

// OnConnectionWarning is called when approaching connection limits
func (r *SimpleRenderer) OnConnectionWarning(activeRequests int32, maxConns int) {
	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s High connection load detected (%d active requests)\n",
		color.Yellow("⚠ WARNING:"), activeRequests)
	fmt.Printf("  The CLI is limited to %d concurrent connections per host.\n", maxConns)
	fmt.Printf("  Consider reducing request rate or increasing connection limit.\n")
	fmt.Printf("  Run with --max-connections=%d to increase the limit.\n\n", maxConns*2)
}

// Cleanup stops the spinner and cleans up resources
func (r *SimpleRenderer) Cleanup() {
	if r.spinner != nil {
		ansi.StopSpinner(r.spinner, "", log.StandardLogger().Out)
		r.spinner = nil
	}
}

// Done returns a channel that is closed when the renderer wants to quit
func (r *SimpleRenderer) Done() <-chan struct{} {
	return r.doneCh
}
