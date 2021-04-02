package websocket

import (
	"encoding/json"
)

type AttemptRequest struct {
	Method  string          `json:"method"`
	Data    json.RawMessage `json:"data"`
	Headers json.RawMessage `json:"headers"`
}

type AttemptBody struct {
	Path         string         `json:"cli_path"`
	AttemptId    string         `json:"attempt_id"`
	ConnectionId string         `json:"webhook_id"`
	Request      AttemptRequest `json:"request"`
}

type Attempt struct {
	Event string      `json:"type"`
	Body  AttemptBody `json:"body"`
}

type AttemptResponseBody struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

type AttemptResponse struct {
	Event string              `json:"event"`
	Body  AttemptResponseBody `json:"body"`
}
