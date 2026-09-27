package websocket

import (
	"encoding/base64"
	"encoding/json"
)

// CapabilitiesHeader advertises, on every websocket connect, which attempt
// formats this CLI understands. The server only sends a binary body
// (data_base64) to a session that advertised CapabilityBinaryBody; without it,
// binary events fail on the server instead of reaching an older CLI that would
// forward an empty body.
const CapabilitiesHeader = "X-Hookdeck-CLI-Capabilities"

// CapabilityBinaryBody means the CLI forwards request.data_base64 as raw bytes.
const CapabilityBinaryBody = "binary"

// BodyFormatBinary marks an attempt whose body is carried in DataBase64.
const BodyFormatBinary = "binary"

type AttemptRequest struct {
	Method     string          `json:"method"`
	Timeout    int64           `json:"timeout"`
	DataString string          `json:"data_string"`
	BodyFormat string          `json:"body_format,omitempty"`
	DataBase64 string          `json:"data_base64,omitempty"`
	Headers    json.RawMessage `json:"headers"`
}

// IsBinary reports whether the body travels as base64 rather than as DataString.
func (r AttemptRequest) IsBinary() bool {
	return r.BodyFormat == BodyFormatBinary || r.DataBase64 != ""
}

// Body returns the exact bytes to forward to the local server. Binary bodies
// are decoded from DataBase64; text bodies are DataString as sent, which keeps
// working against servers that predate data_base64.
func (r AttemptRequest) Body() ([]byte, error) {
	if r.IsBinary() {
		return base64.StdEncoding.DecodeString(r.DataBase64)
	}
	return []byte(r.DataString), nil
}

type AttemptBody struct {
	Path         string         `json:"cli_path"`
	EventID      string         `json:"event_id"`
	AttemptId    string         `json:"attempt_id"`
	ConnectionId string         `json:"webhook_id"`
	Request      AttemptRequest `json:"request"`
}

type Attempt struct {
	Event string      `json:"type"`
	Body  AttemptBody `json:"body"`
}

type AttemptResponseBody struct {
	AttemptId string `json:"attempt_id"`
	CLIPath   string `json:"cli_path"`
	Status    int    `json:"status"`
	Data      string `json:"data"`
}

type AttemptResponse struct {
	Event string              `json:"event"`
	Body  AttemptResponseBody `json:"body"`
}

type ErrorAttemptBody struct {
	AttemptId string `json:"attempt_id"`
	Error     bool   `json:"error"`
}

type ErrorAttemptResponse struct {
	Event string           `json:"event"`
	Body  ErrorAttemptBody `json:"body"`
}
