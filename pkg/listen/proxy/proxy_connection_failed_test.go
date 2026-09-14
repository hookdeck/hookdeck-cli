package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

// recordingRenderer records the lifecycle calls in the order they arrive, so a
// test can assert not just that the renderer was told something but when.
type recordingRenderer struct {
	mu     sync.Mutex
	calls  []string
	failed error
	doneCh chan struct{}
}

func newRecordingRenderer() *recordingRenderer {
	return &recordingRenderer{doneCh: make(chan struct{})}
}

func (r *recordingRenderer) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
}

func (r *recordingRenderer) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *recordingRenderer) connectionFailure() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.failed
}

func (r *recordingRenderer) OnConnecting()   { r.record("OnConnecting") }
func (r *recordingRenderer) OnConnected()    { r.record("OnConnected") }
func (r *recordingRenderer) OnDisconnected() { r.record("OnDisconnected") }
func (r *recordingRenderer) OnError(err error) {
	r.record("OnError")
}

func (r *recordingRenderer) OnConnectionFailed(err error) {
	r.mu.Lock()
	r.failed = err
	r.mu.Unlock()
	r.record("OnConnectionFailed")
}

func (r *recordingRenderer) OnEventPending(string, *websocket.Attempt, time.Time) {}
func (r *recordingRenderer) OnEventComplete(string, *websocket.Attempt, *EventResponse, time.Time) {
}
func (r *recordingRenderer) OnEventError(string, *websocket.Attempt, error, time.Time) {}
func (r *recordingRenderer) OnConnectionWarning(int32, int)                            {}
func (r *recordingRenderer) OnServerHealthChanged(bool, error)                         {}
func (r *recordingRenderer) Cleanup()                                                  { r.record("Cleanup") }
func (r *recordingRenderer) Done() <-chan struct{}                                     { return r.doneCh }
func (r *recordingRenderer) Err() error                                                { return nil }

// TestRunReportsGivingUpToTheRendererBeforeTeardown covers the failure half of
// #399 end to end.
//
// When the CLI exhausts its connection attempts it has to say so inside the
// renderer before tearing it down. The interactive renderer otherwise spends
// the whole attempt budget looking live, then vanishes into the shell with the
// alt-screen already gone - so the order matters as much as the call.
func TestRunReportsGivingUpToTheRendererBeforeTeardown(t *testing.T) {
	// One attempt, no backoff: the production budget is 10 attempts two seconds
	// apart, which is the same code path twenty seconds slower.
	restoreAttempts, restoreBackoff := maxConnectAttempts, fixedConnectBackoffMS
	maxConnectAttempts, fixedConnectBackoffMS = 1, 1
	t.Cleanup(func() { maxConnectAttempts, fixedConnectBackoffMS = restoreAttempts, restoreBackoff })

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == hookdeck.APIPathPrefix+"/cli-sessions" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cli_sess_1"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(api.Close)

	// A plain HTTP server never completes the websocket handshake, so every
	// connect attempt fails immediately and for a reportable reason.
	ws := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(ws.Close)

	apiURL, err := url.Parse(api.URL)
	require.NoError(t, err)
	targetURL, err := url.Parse("http://127.0.0.1:1")
	require.NoError(t, err)

	renderer := newRecordingRenderer()
	p := New(&Config{
		DeviceName:    "test-device",
		Key:           "sk_test_123456789012",
		URL:           targetURL,
		APIBaseURL:    api.URL,
		WSBaseURL:     "ws://" + strings.TrimPrefix(ws.URL, "http://"),
		NoWSS:         true,
		NoHealthcheck: true,
		APIClient:     &hookdeck.Client{BaseURL: apiURL, APIKey: "sk_test_123456789012"},
	}, nil, renderer)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	runErr := p.Run(ctx)
	require.Error(t, runErr, "giving up must be reported as a command failure")
	assert.Contains(t, runErr.Error(), "Could not connect")

	calls := renderer.recorded()
	failedAt := indexOfCall(calls, "OnConnectionFailed")
	cleanupAt := indexOfCall(calls, "Cleanup")
	require.GreaterOrEqual(t, failedAt, 0,
		"the renderer must be told the CLI gave up, not just the shell: %v", calls)
	require.GreaterOrEqual(t, cleanupAt, 0, "the renderer must still be torn down: %v", calls)
	assert.Less(t, failedAt, cleanupAt,
		"the failure has to be shown before the renderer is torn down: %v", calls)

	failure := renderer.connectionFailure()
	require.Error(t, failure, "the renderer needs the reason, not just the fact")
	assert.Equal(t, runErr.Error(), failure.Error(),
		"the renderer must be given the same give-up error the command returns")
}

func indexOfCall(calls []string, want string) int {
	for i, c := range calls {
		if c == want {
			return i
		}
	}
	return -1
}
