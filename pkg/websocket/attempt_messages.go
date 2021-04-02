package websocket

import (
	"encoding/json"
)

type AttemptRequest struct {
	Method  string          `json:"method"`
	Timeout int64           `json:"timeout"`
	Data    json.RawMessage `json:"data"`
	Headers json.RawMessage `json:"headers"`
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
	Error     error  `json:"error"`
}

type ErrorAttemptResponse struct {
	Event string           `json:"event"`
	Body  ErrorAttemptBody `json:"body"`
}
