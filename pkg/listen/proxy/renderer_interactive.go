package proxy

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	log "github.com/sirupsen/logrus"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/listen/tui"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

const interactiveTimeLayout = "2006-01-02 15:04:05"

// InteractiveRenderer renders events using Bubble Tea TUI
type InteractiveRenderer struct {
	cfg        *RendererConfig
	teaProgram *tea.Program
	teaModel   *tui.Model
	doneCh     chan struct{}
}

// NewInteractiveRenderer creates a new interactive renderer with Bubble Tea
func NewInteractiveRenderer(cfg *RendererConfig) *InteractiveRenderer {
	tuiCfg := &tui.Config{
		DeviceName:       cfg.DeviceName,
		APIKey:           cfg.APIKey,
		APIBaseURL:       cfg.APIBaseURL,
		DashboardBaseURL: cfg.DashboardBaseURL,
		ConsoleBaseURL:   cfg.ConsoleBaseURL,
		ProjectMode:      cfg.ProjectMode,
		ProjectID:        cfg.ProjectID,
		GuestURL:         cfg.GuestURL,
		TargetURL:        cfg.TargetURL,
		Sources:          cfg.Sources,
		Connections:      cfg.Connections,
		Filters:          cfg.Filters,
	}

	model := tui.NewModel(tuiCfg)
	program := tea.NewProgram(&model, tea.WithAltScreen())

	r := &InteractiveRenderer{
		cfg:        cfg,
		teaProgram: program,
		teaModel:   &model,
		doneCh:     make(chan struct{}),
	}

	// Start TUI in background
	go func() {
		if _, err := r.teaProgram.Run(); err != nil {
			log.WithField("prefix", "proxy.InteractiveRenderer").
				Errorf("Bubble Tea error: %v", err)
		}
		// Signal that TUI has exited
		close(r.doneCh)
	}()

	return r
}

// OnConnecting is called when starting to connect
func (r *InteractiveRenderer) OnConnecting() {
	if r.teaProgram != nil {
		r.teaProgram.Send(tui.ConnectingMsg{})
	}
}

// OnConnected is called when websocket connects
func (r *InteractiveRenderer) OnConnected() {
	if r.teaProgram != nil {
		r.teaProgram.Send(tui.ConnectedMsg{})
	}
}

// OnDisconnected is called when websocket disconnects
func (r *InteractiveRenderer) OnDisconnected() {
	if r.teaProgram != nil {
		r.teaProgram.Send(tui.DisconnectedMsg{})
	}
}

// OnError is called when an error occurs
func (r *InteractiveRenderer) OnError(err error) {
	// Errors are handled through OnEventError
}

// OnEventPending is called when an event starts (after 100ms delay)
func (r *InteractiveRenderer) OnEventPending(eventID string, attempt *websocket.Attempt, startTime time.Time) {
	r.showPendingEvent(eventID, attempt, startTime)
}

// OnEventComplete is called when an event completes successfully
func (r *InteractiveRenderer) OnEventComplete(eventID string, attempt *websocket.Attempt, response *EventResponse, startTime time.Time) {
	eventTime := time.Now()
	localTime := eventTime.Format(interactiveTimeLayout)
	color := ansi.Color(os.Stdout)

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

	eventStatus := response.StatusCode
	eventSuccess := response.StatusCode >= 200 && response.StatusCode < 300

	// Send update message to TUI (will update existing pending event or create new if not found)
	if r.teaProgram != nil {
		r.teaProgram.Send(tui.UpdateEventMsg{
			EventID:          eventID,
			AttemptID:        attempt.Body.AttemptId,
			Time:             startTime,
			Data:             attempt,
			Status:           eventStatus,
			Success:          eventSuccess,
			LogLine:          outputStr,
			ResponseStatus:   eventStatus,
			ResponseHeaders:  response.Headers,
			ResponseBody:     response.Body,
			ResponseDuration: response.Duration,
		})
	}
}

// showPendingEvent shows a pending event (waiting for response)
func (r *InteractiveRenderer) showPendingEvent(eventID string, attempt *websocket.Attempt, eventTime time.Time) {
	color := ansi.Color(os.Stdout)
	localTime := eventTime.Format(interactiveTimeLayout)

	pendingStr := fmt.Sprintf("%s [%s] %s %s %s",
		color.Faint(localTime),
		color.Faint("..."),
		attempt.Body.Request.Method,
		fmt.Sprintf("http://localhost%s", attempt.Body.Path),
		color.Faint("(Waiting for response)"),
	)

	event := tui.EventInfo{
		ID:               eventID,
		AttemptID:        attempt.Body.AttemptId,
		Status:           0,
		Success:          false,
		Time:             eventTime,
		Data:             attempt,
		LogLine:          pendingStr,
		ResponseStatus:   0,
		ResponseDuration: 0,
	}

	if r.teaProgram != nil {
		r.teaProgram.Send(tui.NewEventMsg{Event: event})
	}
}

// OnEventError is called when an event encounters an error
func (r *InteractiveRenderer) OnEventError(eventID string, attempt *websocket.Attempt, err error, startTime time.Time) {
	color := ansi.Color(os.Stdout)
	localTime := time.Now().Format(interactiveTimeLayout)

	errStr := fmt.Sprintf("%s [%s] Failed to %s: %v",
		color.Faint(localTime),
		color.Red("ERROR").Bold(),
		attempt.Body.Request.Method,
		err,
	)

	event := tui.EventInfo{
		ID:               eventID,
		AttemptID:        attempt.Body.AttemptId,
		Status:           0,
		Success:          false,
		Time:             time.Now(),
		Data:             attempt,
		LogLine:          errStr,
		ResponseStatus:   0,
		ResponseDuration: 0,
	}

	if r.teaProgram != nil {
		r.teaProgram.Send(tui.NewEventMsg{Event: event})
	}
}

// OnConnectionWarning is called when approaching connection limits
func (r *InteractiveRenderer) OnConnectionWarning(activeRequests int32, maxConns int) {
	// In interactive mode, warnings could be shown in TUI
	// Use structured logging to avoid format-string mismatches and make logs machine-readable
	log.WithFields(log.Fields{
		"prefix":          "proxy.InteractiveRenderer",
		"active_requests": activeRequests,
		"max_connections": maxConns,
	}).Warn("High connection load detected; consider increasing --max-connections")
}

// Cleanup gracefully stops the TUI and restores terminal
func (r *InteractiveRenderer) Cleanup() {
	if r.teaProgram != nil {
		r.teaProgram.Quit()
		// Wait a moment for graceful shutdown
		select {
		case <-r.doneCh:
			// TUI exited cleanly
		case <-time.After(100 * time.Millisecond):
			// Timeout, force kill
			r.teaProgram.Kill()
		}
		// Give terminal a moment to fully restore after alt screen exit
		time.Sleep(50 * time.Millisecond)
	}
}

// Done returns a channel that is closed when the renderer wants to quit
func (r *InteractiveRenderer) Done() <-chan struct{} {
	return r.doneCh
}
