package proxy

import (
	"net/url"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

// Renderer is the interface for handling proxy output
// Implementations handle different output modes (interactive, compact, quiet)
type Renderer interface {
	// Lifecycle events
	OnConnecting()
	OnConnected()
	OnDisconnected()
	OnError(err error)

	// Event handling
	OnEventPending(eventID string, attempt *websocket.Attempt, startTime time.Time) // For interactive mode (100ms delay)
	OnEventComplete(eventID string, attempt *websocket.Attempt, response *EventResponse, startTime time.Time)
	OnEventError(eventID string, attempt *websocket.Attempt, err error, startTime time.Time)

	// Connection warnings
	OnConnectionWarning(activeRequests int32, maxConns int)

	// Server health monitoring
	OnServerHealthChanged(healthy bool, err error)

	// Cleanup is called before exit to clean up resources (e.g., stop TUI, stop spinner)
	Cleanup()

	// Done returns a channel that signals when user wants to quit
	Done() <-chan struct{}

	// Err reports why the renderer stopped. It returns nil for a normal
	// user-initiated quit, and non-nil when the renderer failed — for example
	// when the interactive renderer cannot open a terminal. Callers must treat a
	// non-nil value as a command failure: a renderer that dies at startup
	// forwards no events, and reporting success would hide that entirely (#333).
	Err() error
}

// EventResponse contains the HTTP response data
type EventResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       string
	Duration   time.Duration
}

// RendererConfig contains configuration for creating renderers
type RendererConfig struct {
	DeviceName       string
	APIKey           string
	APIBaseURL       string
	DashboardBaseURL string
	ConsoleBaseURL   string
	ProjectMode      string
	ProjectID        string
	GuestURL         string
	TargetURL        *url.URL
	Output           string
	Sources          []*hookdeck.Source
	Connections      []*hookdeck.Connection
	Filters          *hookdeck.SessionFilters
	APIClient        *hookdeck.Client
	AppConfig        *config.Config
}

// NewRenderer creates the appropriate renderer based on output mode
func NewRenderer(cfg *RendererConfig) Renderer {
	switch cfg.Output {
	case "interactive":
		return NewInteractiveRenderer(cfg)
	case "compact":
		return NewSimpleRenderer(cfg, false) // verbose mode
	case "quiet":
		return NewSimpleRenderer(cfg, true) // quiet mode
	default:
		return NewSimpleRenderer(cfg, false)
	}
}
