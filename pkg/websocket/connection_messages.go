package websocket

type ConnectionMessageBody struct {
	SourceId      string   `json:"source_id"`
	ConnectionIds []string `json:"webhook_ids"`
}

type ConnectionMessage struct {
	Event string                `json:"event"`
	Body  ConnectionMessageBody `json:"body"`
}
