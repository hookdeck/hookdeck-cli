package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
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
	Port       string
	APIBaseURL string
	WSBaseURL  string
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

const maxConnectAttempts = 3

func (p *Proxy) Run(ctx context.Context) error {
	s := ansi.StartNewSpinner("Getting ready...", p.cfg.Log.Out)

	ctx = withSIGTERMCancel(ctx, func() {
		log.WithFields(log.Fields{
			"prefix": "proxy.Proxy.Run",
		}).Debug("Ctrl+C received, cleaning up...")
	})

	session, err := p.createSession(ctx)
	if err != nil {
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		p.cfg.Log.Fatalf("Error while authenticating with Hookdeck: %v", err)
	}

	if session.Id == "" {
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		p.cfg.Log.Fatalf("Error while starting a new session")
	}

	var nAttempts int = 0

	for nAttempts < maxConnectAttempts {
		p.webSocketClient = websocket.NewClient(
			p.cfg.WSBaseURL,
			session.Id,
			p.cfg.Key,
			&websocket.Config{
				Log:               p.cfg.Log,
				NoWSS:             p.cfg.NoWSS,
				ReconnectInterval: time.Duration(100000) * time.Second,
				EventHandler:      websocket.EventHandlerFunc(p.processAttempt),
			},
		)

		go func() {
			<-p.webSocketClient.Connected()
			nAttempts = 0
			ansi.StopSpinner(s, fmt.Sprintf("Ready! (^C to quit)"), p.cfg.Log.Out)
		}()

		go p.webSocketClient.Run(ctx)
		nAttempts++

		select {
		case <-ctx.Done():
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
			return nil
		case <-p.webSocketClient.NotifyExpired:
			if nAttempts < maxConnectAttempts {
				ansi.StartSpinner(s, "Connection lost, reconnecting...", p.cfg.Log.Out)
			} else {
				p.cfg.Log.Fatalf("Session expired. Terminating after %d failed attempts to reauthorize", nAttempts)
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

	var err error

	exitCh := make(chan struct{})

	go func() {
		parsedBaseURL, err := url.Parse(p.cfg.APIBaseURL)
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
				exitCh <- struct{}{}
				return
			}

			select {
			case <-ctx.Done():
				exitCh <- struct{}{}
				return
			case <-time.After(1 * time.Second):
			}
		}

		exitCh <- struct{}{}
	}()
	<-exitCh

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
		fmt.Println(webhookEvent.Body.Request.Data)
	} else {
		url := "http://localhost:" + p.cfg.Port + webhookEvent.Body.Path

		timeout := webhookEvent.Body.Request.Timeout
		if timeout == 0 {
			timeout = 1000 * 30
		}
		client := &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
		}
		req, err := http.NewRequest(webhookEvent.Body.Request.Method, url, bytes.NewBuffer(webhookEvent.Body.Request.Data))
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
	outputStr := fmt.Sprintf("%s [%d] %s %s | https://dashboard.hookdeck.com/cli/events/%s",
		color.Faint(localTime),
		ansi.ColorizeStatus(resp.StatusCode),
		resp.Request.Method,
		resp.Request.URL,
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
