package domain

import (
	"encoding/json"
	"time"
)

type OutboxEvent struct {
	ID        string
	EventType string
	Payload   json.RawMessage
	CreatedAt time.Time
}
