package proxy

import (
	"context"
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
	Key string
	// EndpointsMap is a mapping of local webhook endpoint urls to the events they consume
	URL              *url.URL
	APIBaseURL       string
	DashboardBaseURL string
	WSBaseURL        string
	// Indicates whether to print full JSON objects to stdout
	PrintJSON bool
	Log       *log.Logger
	// Force use of unencrypted ws:// protocol instead of wss://
	NoWSS bool
}

// A Proxy opens a websocket connection with Hookdeck, listens for incoming
// webhook events, forwards them to the local endpoint and sends the response
// back to Hookdeck.
type Proxy struct {
	cfg               *Config
	source            hookdeck.Source
	connections       []hookdeck.Connection
	connections_paths map[string]string
	webSocketClient   *websocket.Client
	connectionTimer   *time.Timer
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

func (p *Proxy) Run(parentCtx context.Context) error {
	const maxConnectAttempts = 3
	nAttempts := 0
	// Track whether or not we have connected at least once successfully
	hasConnectedOnce := false

	canConnect := func() bool {
		// Once we have connected we should always retry
		// until the user cancels the program
		if hasConnectedOnce {
			return true
		} else {
			return nAttempts < maxConnectAttempts
		}
	}

	s := ansi.StartNewSpinner("Getting ready...", p.cfg.Log.Out)

	signalCtx := withSIGTERMCancel(parentCtx, func() {
		log.WithFields(log.Fields{
			"prefix": "proxy.Proxy.Run",
		}).Debug("Ctrl+C received, cleaning up...")
	})

	session, err := p.createSession(signalCtx)
	if err != nil {
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		p.cfg.Log.Fatalf("Error while authenticating with Hookdeck: %v", err)
	}

	if session.Id == "" {
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		p.cfg.Log.Fatalf("Error while starting a new session")
	}

	for canConnect() {
		p.webSocketClient = websocket.NewClient(
			p.cfg.WSBaseURL,
			session.Id,
			p.cfg.Key,
			&websocket.Config{
				Log:          p.cfg.Log,
				NoWSS:        p.cfg.NoWSS,
				EventHandler: websocket.EventHandlerFunc(p.processAttempt),
			},
		)

		go func() {
			<-p.webSocketClient.Connected()
			msg := "Ready! (^C to quit)"
			if hasConnectedOnce {
				msg = "Reconnected!"
			}
			ansi.StopSpinner(s, msg, p.cfg.Log.Out)
			hasConnectedOnce = true
		}()

		go p.webSocketClient.Run(signalCtx)
		nAttempts++

		select {
		case <-signalCtx.Done():
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
			return nil
		case <-p.webSocketClient.NotifyExpired:
			if canConnect() {
				ansi.StartSpinner(s, "Connection lost, reconnecting...", p.cfg.Log.Out)
			} else {
				p.cfg.Log.Fatalf("Session expired. Terminating after %d failed attempts to reauthorize", nAttempts)
			}
		}

		attemptsOverMax := math.Max(0, float64(nAttempts-maxConnectAttempts))
		if canConnect() && attemptsOverMax > 0 {
			// Determine the time to wait to reconnect, maximum of 10 second intervals
			sleepDurationMS := int(math.Round(math.Min(100, math.Pow(attemptsOverMax, 2)) * 100))
			log.WithField(
				"prefix", "proxy.Proxy.Run",
			).Debugf(
				"Connect backoff (%dms)", sleepDurationMS,
			)

			// Stop the timer
			p.connectionTimer.Stop()

			// Reset the timer to the next duration
			p.connectionTimer.Reset(time.Duration(sleepDurationMS) * time.Millisecond)

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
		BaseURL: parsedBaseURL,
		APIKey:  p.cfg.Key,
	}

	var connection_ids []string
	for _, connection := range p.connections {
		connection_ids = append(connection_ids, connection.Id)
	}

	for i := 0; i <= 5; i++ {
		session, err = client.CreateSession(hookdeck.CreateSessionInput{SourceId: p.source.Id,
			ConnectionIds: connection_ids})

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
		p.cfg.Log.Debug("WebSocket specified for Webhooks received non-webhook event")
		return
	}

	webhookEvent := msg.Attempt

	p.cfg.Log.WithFields(log.Fields{
		"prefix": "proxy.Proxy.processAttempt",
	}).Debugf("Processing webhook event")

	if p.cfg.PrintJSON {
		fmt.Println(webhookEvent.Body.Request.DataString)
	} else {
		url := p.cfg.URL.Scheme + "://" + p.cfg.URL.Host + p.cfg.URL.Path + webhookEvent.Body.Path

		timeout := webhookEvent.Body.Request.Timeout
		if timeout == 0 {
			timeout = 1000 * 30
		}
		client := &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
		}

		req, err := http.NewRequest(webhookEvent.Body.Request.Method, url, nil)
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

			fmt.Println(errStr)
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
	outputStr := fmt.Sprintf("%s [%d] %s %s | %s/cli/events/%s",
		color.Faint(localTime),
		ansi.ColorizeStatus(resp.StatusCode),
		resp.Request.Method,
		resp.Request.URL,
		p.cfg.DashboardBaseURL,
		webhookEvent.Body.EventID,
	)
	fmt.Println(outputStr)

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

	// body := truncate(string(buf), 5000, true)

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
func New(cfg *Config, source hookdeck.Source, connections []hookdeck.Connection) *Proxy {
	if cfg.Log == nil {
		cfg.Log = &log.Logger{Out: ioutil.Discard}
	}

	connections_paths := make(map[string]string)

	for _, connection := range connections {
		connections_paths[connection.Id] = connection.Destination.CliPath
	}

	p := &Proxy{
		cfg:               cfg,
		connections:       connections,
		connections_paths: connections_paths,
		source:            source,
		connectionTimer:   time.NewTimer(0), // Defaults to no delay
	}

	return p
}

//
// Private constants
//

const (
	maxBodySize        = 5000
	maxNumHeaders      = 20
	maxHeaderKeySize   = 50
	maxHeaderValueSize = 200
)

//
// Private functions
//

// truncate will truncate str to be less than or equal to maxByteLength bytes.
// It will respect UTF8 and truncate the string at a code point boundary.
// If ellipsis is true, we'll append "..." to the truncated string if the string
// was in fact truncated, and if there's enough room. Note that the
// full string returned will always be <= maxByteLength bytes long, even with ellipsis.
func truncate(str string, maxByteLength int, ellipsis bool) string {
	if len(str) <= maxByteLength {
		return str
	}

	bytes := []byte(str)

	if ellipsis && maxByteLength > 3 {
		maxByteLength -= 3
	} else {
		ellipsis = false
	}

	for maxByteLength > 0 && maxByteLength < len(bytes) && isUTF8ContinuationByte(bytes[maxByteLength]) {
		maxByteLength--
	}

	result := string(bytes[0:maxByteLength])
	if ellipsis {
		result += "..."
	}

	return result
}

func isUTF8ContinuationByte(b byte) bool {
	return (b & 0xC0) == 0x80
}
