package proxy

import (
	"context"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
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
}

// withSIGTERMCancel creates a context that will be canceled when Ctrl+C is pressed
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
