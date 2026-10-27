package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
)

// refusingServer answers every websocket connect with status and body, the way
// Hookdeck refuses a key.
func refusingServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

// runOnce makes one connect attempt and waits for the client to report it lost.
func runOnce(t *testing.T, client *Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go client.Run(ctx)
	select {
	case <-client.NotifyExpired:
	case <-ctx.Done():
		t.Fatal("client did not report the connection as lost")
	}
}

func TestRefusedConnectEndsAccess(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		message string
	}{
		{
			name:    "revoked key with the server's reason",
			status:  http.StatusUnauthorized,
			body:    `{"error":{"code":"UNAUTHORIZED","message":"This CLI key is no longer valid."}}`,
			message: "This CLI key is no longer valid.",
		},
		{
			name:    "no access to the project with the server's reason",
			status:  http.StatusForbidden,
			body:    `{"error":{"code":"NO_PROJECT_ACCESS","message":"This CLI key can't reach this project."}}`,
			message: "This CLI key can't reach this project.",
		},
		{
			name:    "revoked key from a server that gives no reason",
			status:  http.StatusUnauthorized,
			message: "This CLI key is no longer valid. Run `hookdeck login` to log in again.",
		},
		{
			name:    "refused key from a server that gives no reason",
			status:  http.StatusForbidden,
			message: "Hookdeck refused the connection: this CLI key can't use this project. Run `hookdeck project use` to pick another one, or `hookdeck login`.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := refusingServer(t, tc.status, tc.body)
			client := NewClient(wsURL(server), "cses_test", "cli-key", "tm_test", nil, "", &Config{Log: logrus.New()})

			runOnce(t, client)

			accessErr := client.AccessEndedErr()
			if accessErr == nil {
				t.Fatalf("a %d on connect must end access", tc.status)
			}
			if accessErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", accessErr.StatusCode, tc.status)
			}
			if accessErr.Error() != tc.message {
				t.Errorf("Error() = %q, want %q", accessErr.Error(), tc.message)
			}
		})
	}
}

// 503 means Hookdeck is busy or restarting, so the CLI must keep retrying.
func TestUnavailableConnectKeepsRetrying(t *testing.T) {
	server := refusingServer(t, http.StatusServiceUnavailable, "")
	client := NewClient(wsURL(server), "cses_test", "cli-key", "tm_test", nil, "", &Config{Log: logrus.New()})

	runOnce(t, client)

	if accessErr := client.AccessEndedErr(); accessErr != nil {
		t.Fatalf("a 503 must not end access, got %v", accessErr)
	}
	if client.LastConnectErr() == nil {
		t.Error("the failed connect must still be recorded")
	}
}

func TestRevokedWhileConnectedEndsAccess(t *testing.T) {
	var captured http.Header
	server := upgradeTestServer(t, &captured, func(conn *ws.Conn) {
		_ = conn.WriteControl(
			ws.CloseMessage,
			ws.FormatCloseMessage(closeCodeCLIKeyRevoked, "CLI_KEY_REVOKED"),
			time.Now().Add(time.Second),
		)
	})
	defer server.Close()

	logger, hook := logtest.NewNullLogger()
	client := NewClient(wsURL(server), "cses_test", "cli-key", "tm_test", nil, "", &Config{Log: logger})

	runOnce(t, client)

	accessErr := client.AccessEndedErr()
	if accessErr == nil {
		t.Fatal("a 4003 close must end access")
	}
	if want := "Your CLI key was revoked. Run `hookdeck login` to log in again."; accessErr.Error() != want {
		t.Errorf("Error() = %q, want %q", accessErr.Error(), want)
	}
	// A revoke is expected, so it must not tell the user to file a bug report.
	for _, entry := range hook.AllEntries() {
		if entry.Level <= logrus.ErrorLevel {
			t.Errorf("4003 logged at %s level: %s", entry.Level, entry.Message)
		}
	}
}
