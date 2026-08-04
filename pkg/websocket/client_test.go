package websocket

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
)

// upgradeTestServer starts an httptest server that upgrades websocket requests and captures
// the connect headers. The server-side connection is handed to onConn when provided.
func upgradeTestServer(t *testing.T, captured *http.Header, onConn func(conn *ws.Conn)) *httptest.Server {
	t.Helper()
	upgrader := ws.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*captured = r.Header.Clone()
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		if onConn != nil {
			onConn(conn)
		}
	}))
}

func wsURL(s *httptest.Server) string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

// startClient runs the client and waits for the websocket connection to be
// established. Tests must go through Run() rather than calling connect()
// directly: connect() starts the read/write pumps, and without Run's select
// loop draining notifyClose, readPump can block forever after Stop().
func startClient(t *testing.T, client *Client) {
	t.Helper()
	go client.Run(context.Background())
	select {
	case <-client.Connected():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the client to connect")
	}
}

func TestConnectSendsSessionRecreationHeaders(t *testing.T) {
	filtersJSON := `{"body":{"name":"héllo wörld — テスト"}}`
	var captured http.Header
	server := upgradeTestServer(t, &captured, nil)
	defer server.Close()

	client := NewClient(
		wsURL(server),
		"cses_test",
		"cli-key",
		"tm_test",
		[]string{"web_abc", "web_def"},
		filtersJSON,
		&Config{},
	)
	startClient(t, client)
	defer client.Stop()

	if got := captured.Get("Websocket-Id"); got != "cses_test" {
		t.Errorf("Websocket-Id = %q, want %q", got, "cses_test")
	}
	if got := captured.Get("X-Webhook-Ids"); got != "web_abc,web_def" {
		t.Errorf("X-Webhook-Ids = %q, want %q", got, "web_abc,web_def")
	}

	encoded := captured.Get("X-Session-Filters")
	if encoded == "" {
		t.Fatal("X-Session-Filters header not sent")
	}
	// Base64 keeps the header value ASCII-safe: raw UTF-8 bytes would be decoded as latin-1
	// by the Node server and silently corrupt non-ASCII filter values.
	for i := 0; i < len(encoded); i++ {
		if encoded[i] > 127 {
			t.Fatalf("X-Session-Filters contains non-ASCII byte at %d: %q", i, encoded)
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("X-Session-Filters is not valid base64: %v", err)
	}
	if string(decoded) != filtersJSON {
		t.Errorf("decoded filters = %q, want %q", decoded, filtersJSON)
	}
}

func TestConnectOmitsSessionHeadersWhenUnset(t *testing.T) {
	var captured http.Header
	server := upgradeTestServer(t, &captured, nil)
	defer server.Close()

	client := NewClient(wsURL(server), "cses_test", "cli-key", "tm_test", nil, "", &Config{})
	startClient(t, client)
	defer client.Stop()

	if _, ok := captured["X-Webhook-Ids"]; ok {
		t.Error("X-Webhook-Ids should not be sent when there are no connection IDs")
	}
	if _, ok := captured["X-Session-Filters"]; ok {
		t.Error("X-Session-Filters should not be sent when there are no filters")
	}
}

func TestStopSendsCleanClose(t *testing.T) {
	closeCode := make(chan int, 1)
	var captured http.Header
	server := upgradeTestServer(t, &captured, func(conn *ws.Conn) {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if ce, ok := err.(*ws.CloseError); ok {
					closeCode <- ce.Code
				} else {
					closeCode <- -1
				}
				return
			}
		}
	})
	defer server.Close()

	client := NewClient(wsURL(server), "cses_test", "cli-key", "tm_test", nil, "", &Config{})
	startClient(t, client)

	client.Stop()

	// The server must see a clean close (1000) — that's what lets it tombstone the session
	// instead of holding it for the reconnect grace window.
	select {
	case code := <-closeCode:
		if code != ws.CloseNormalClosure {
			t.Errorf("server saw close code %d, want %d (normal closure)", code, ws.CloseNormalClosure)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive a close frame")
	}
}

// The server intentionally closes with 1001 (pod restart during a deploy) and 4001
// (session expired; recreated on reconnect via the session headers). Both are part of
// normal operation and must not produce error-level logs that alarm the user.
func TestServerCloseCodesReconnectQuietly(t *testing.T) {
	codes := map[string]int{
		"server_shutdown_1001": ws.CloseGoingAway,
		"session_expired_4001": closeCodeSessionExpired,
	}
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			var captured http.Header
			server := upgradeTestServer(t, &captured, func(conn *ws.Conn) {
				_ = conn.WriteControl(
					ws.CloseMessage,
					ws.FormatCloseMessage(code, "test"),
					time.Now().Add(time.Second),
				)
			})
			defer server.Close()

			logger, hook := logtest.NewNullLogger()
			client := NewClient(wsURL(server), "cses_test", "cli-key", "tm_test", nil, "", &Config{Log: logger})

			go client.Run(context.Background())

			select {
			case <-client.NotifyExpired:
			case <-time.After(5 * time.Second):
				t.Fatal("client did not report connection loss after server close")
			}

			if !client.HasConnected() {
				t.Error("HasConnected() = false, want true after a successful connect that later dropped")
			}

			for _, entry := range hook.AllEntries() {
				if entry.Level <= logrus.ErrorLevel {
					t.Errorf("close code %d logged at %s level: %s", code, entry.Level, entry.Message)
				}
			}
		})
	}
}

// A connection that dies without a close handshake (network blip, laptop sleep, load
// balancer timeout, ungracefully killed pod) surfaces as a synthesized 1006 CloseError.
// The reconnect loop handles it, so it must not tell the user to file a bug report.
func TestAbruptDisconnectReconnectsQuietly(t *testing.T) {
	var captured http.Header
	server := upgradeTestServer(t, &captured, func(conn *ws.Conn) {
		// Drop the TCP connection with no close frame.
		_ = conn.UnderlyingConn().Close()
	})
	defer server.Close()

	logger, hook := logtest.NewNullLogger()
	client := NewClient(wsURL(server), "cses_test", "cli-key", "tm_test", nil, "", &Config{Log: logger})

	go client.Run(context.Background())

	select {
	case <-client.NotifyExpired:
	case <-time.After(5 * time.Second):
		t.Fatal("client did not report connection loss after the connection dropped")
	}

	for _, entry := range hook.AllEntries() {
		if entry.Level <= logrus.ErrorLevel {
			t.Errorf("abrupt disconnect logged at %s level: %s", entry.Level, entry.Message)
		}
	}
}

func TestStopIsIdempotentWithoutConnection(t *testing.T) {
	client := NewClient("ws://127.0.0.1:1", "cses_test", "cli-key", "tm_test", nil, "", &Config{})

	// Stop before any connection, twice: must not panic (doneOnce) and must close done.
	client.Stop()
	client.Stop()

	select {
	case <-client.done:
	default:
		t.Fatal("done channel not closed after Stop")
	}
}

// Exercises the stateMu paths: Stop can run on the signal-handler goroutine while the connect
// goroutine writes conn/isConnected. Meaningful under `go test -race`.
func TestStopConcurrentWithConnect(t *testing.T) {
	var captured http.Header
	server := upgradeTestServer(t, &captured, nil)
	defer server.Close()

	client := NewClient(wsURL(server), "cses_test", "cli-key", "tm_test", nil, "", &Config{})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = client.connect(context.Background())
	}()
	go func() {
		defer wg.Done()
		client.Stop()
	}()
	wg.Wait()
}
