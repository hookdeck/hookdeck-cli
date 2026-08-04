package websocket

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
)

func TestConnectHeadersIncludeSessionRecreationData(t *testing.T) {
	filtersJSON := []byte(`{"body":{"event_type":"PAYOUT_NORMAL_FAILED"}}`)

	client := NewClient("wss://example.com", "cses_123", "key_123", "tm_123", &Config{
		WebhookIDs:         []string{"web_1", "web_2"},
		SessionFiltersJSON: filtersJSON,
	})

	header := client.connectHeaders()

	if got := header.Get("Websocket-Id"); got != "cses_123" {
		t.Errorf("Websocket-Id = %q, want %q", got, "cses_123")
	}

	if got := header.Get("X-Webhook-Ids"); got != "web_1,web_2" {
		t.Errorf("X-Webhook-Ids = %q, want %q", got, "web_1,web_2")
	}

	encoded := header.Get("X-Session-Filters")
	if encoded == "" {
		t.Fatal("X-Session-Filters header is missing")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("X-Session-Filters is not valid base64: %v", err)
	}
	if string(decoded) != string(filtersJSON) {
		t.Errorf("X-Session-Filters decodes to %q, want %q", decoded, filtersJSON)
	}
}

func TestConnectHeadersOmitEmptySessionRecreationData(t *testing.T) {
	client := NewClient("wss://example.com", "cses_123", "key_123", "tm_123", &Config{})

	header := client.connectHeaders()

	if got := header.Get("X-Webhook-Ids"); got != "" {
		t.Errorf("X-Webhook-Ids = %q, want empty", got)
	}
	if got := header.Get("X-Session-Filters"); got != "" {
		t.Errorf("X-Session-Filters = %q, want empty", got)
	}
}

// runClientAgainstServer connects a client to a test websocket server whose
// handler is given the upgraded connection, then waits for the client to
// report the connection as lost. It returns the client for assertions.
func runClientAgainstServer(t *testing.T, handler func(conn *ws.Conn)) *Client {
	t.Helper()

	upgrader := ws.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(url, "cses_123", "key_123", "tm_123", &Config{
		WebhookIDs: []string{"web_1"},
	})

	go client.Run(context.Background())

	select {
	case <-client.NotifyExpired:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the client to report the connection as lost")
	}

	return client
}

func TestSessionExpiredCloseSetsSessionExpired(t *testing.T) {
	client := runClientAgainstServer(t, func(conn *ws.Conn) {
		deadline := time.Now().Add(time.Second)
		if err := conn.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(closeSessionExpired, "SESSION_EXPIRED"), deadline); err != nil {
			t.Errorf("failed to send close message: %v", err)
			return
		}
		// Wait for the client to echo the close frame before tearing down.
		conn.SetReadDeadline(time.Now().Add(time.Second))
		conn.ReadMessage()
	})

	if !client.SessionExpired() {
		t.Error("SessionExpired() = false after a 4001 close, want true")
	}
}

func TestNormalCloseDoesNotSetSessionExpired(t *testing.T) {
	client := runClientAgainstServer(t, func(conn *ws.Conn) {
		deadline := time.Now().Add(time.Second)
		if err := conn.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseGoingAway, "SERVER_SHUTDOWN"), deadline); err != nil {
			t.Errorf("failed to send close message: %v", err)
			return
		}
		conn.SetReadDeadline(time.Now().Add(time.Second))
		conn.ReadMessage()
	})

	if client.SessionExpired() {
		t.Error("SessionExpired() = true after a 1001 close, want false")
	}
}
