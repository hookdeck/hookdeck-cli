package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

type receivedRequest struct {
	body          []byte
	contentType   string
	contentLength int64
}

// forwardAttempt runs one attempt through processAttempt against a local
// server and returns what that server received.
func forwardAttempt(t *testing.T, request websocket.AttemptRequest) receivedRequest {
	t.Helper()

	received := make(chan receivedRequest, 1)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- receivedRequest{body: body, contentType: r.Header.Get("Content-Type"), contentLength: r.ContentLength}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(local.Close)

	target, err := url.Parse(local.URL)
	require.NoError(t, err)
	p := New(&Config{URL: target, NoHealthcheck: true}, nil, newRecordingRenderer())

	p.processAttempt(websocket.IncomingMessage{Attempt: &websocket.Attempt{
		Body: websocket.AttemptBody{Path: "/webhooks", EventID: "evt_test", AttemptId: "evt_test", Request: request},
	}})

	select {
	case r := <-received:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("local server never received the request")
		return receivedRequest{}
	}
}

func headersJSON(t *testing.T, headers map[string]string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(headers)
	require.NoError(t, err)
	return raw
}

// Every byte value, so any UTF-8 round trip along the way would show.
func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func TestProcessAttemptForwardsBinaryBodyByteExact(t *testing.T) {
	body := allBytes()

	got := forwardAttempt(t, websocket.AttemptRequest{
		Method:     http.MethodPost,
		BodyFormat: websocket.BodyFormatBinary,
		DataBase64: base64.StdEncoding.EncodeToString(body),
		// A stale Content-Length must not win over the decoded body's length.
		Headers: headersJSON(t, map[string]string{"content-type": "application/octet-stream", "content-length": "1"}),
	})

	assert.Equal(t, body, got.body)
	assert.Equal(t, int64(len(body)), got.contentLength)
	assert.Equal(t, "application/octet-stream", got.contentType)
}

func TestProcessAttemptForwardsBinaryMultipartUnparsed(t *testing.T) {
	contentType := "multipart/form-data; boundary=hookdeck-boundary"
	var body bytes.Buffer
	body.WriteString("--hookdeck-boundary\r\nContent-Disposition: form-data; name=\"field\"\r\n\r\nvalue\r\n")
	body.WriteString("--hookdeck-boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"f.bin\"\r\nContent-Type: application/octet-stream\r\n\r\n")
	body.Write(allBytes())
	body.WriteString("\r\n--hookdeck-boundary--\r\n")

	got := forwardAttempt(t, websocket.AttemptRequest{
		Method:     http.MethodPost,
		BodyFormat: websocket.BodyFormatBinary,
		DataBase64: base64.StdEncoding.EncodeToString(body.Bytes()),
		Headers:    headersJSON(t, map[string]string{"content-type": contentType}),
	})

	assert.Equal(t, body.Bytes(), got.body, "boundary and file part bytes must match the original")
	assert.Equal(t, contentType, got.contentType, "the boundary travels in the original Content-Type")
}

func TestProcessAttemptForwardsTextBodyFromDataString(t *testing.T) {
	// The shape every server sends today, and the only one older servers send.
	got := forwardAttempt(t, websocket.AttemptRequest{
		Method:     http.MethodPost,
		DataString: `{"hello":"wörld"}`,
		Headers:    headersJSON(t, map[string]string{"content-type": "application/json"}),
	})

	assert.Equal(t, `{"hello":"wörld"}`, string(got.body))
	assert.Equal(t, int64(len(`{"hello":"wörld"}`)), got.contentLength)
}

func TestAttemptRequestDecodesFromTheServerWireFormat(t *testing.T) {
	var msg websocket.IncomingMessage
	require.NoError(t, json.Unmarshal([]byte(`{"event":"attempt","body":{"cli_path":"/","request":{"method":"POST","headers":{},"body_format":"binary","data_base64":"AP+A"}}}`), &msg))
	require.NotNil(t, msg.Attempt)

	body, err := msg.Attempt.Body.Request.Body()
	require.NoError(t, err)
	assert.Equal(t, []byte{0x00, 0xff, 0x80}, body)
	assert.True(t, msg.Attempt.Body.Request.IsBinary())
}

func TestAttemptRequestRejectsInvalidBase64(t *testing.T) {
	_, err := websocket.AttemptRequest{BodyFormat: websocket.BodyFormatBinary, DataBase64: "not base64!"}.Body()
	assert.Error(t, err)
}
