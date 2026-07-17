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
	if err := client.connect(context.Background()); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
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
	if err := client.connect(context.Background()); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
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
	if err := client.connect(context.Background()); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

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
