package kafka

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
)

var (
	ErrEventIdEmpty     = errors.New("event ID is empty")
	ErrEventType        = errors.New("unknown event type")
	ErrPromotionIdEmpty = errors.New("promotion ID is empty")
)

type RuleUpdater interface {
	Upsert(rule domain.RulePromotion) error
}

type PromotionEventProcessor struct {
	updater RuleUpdater
}

type promotionEvent struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	CreatedAt time.Time       `json:"created_at"`
	Payload   json.RawMessage `json:"payload"`
}

func NewPromotionEventProcessor(ruleUpdater RuleUpdater) *PromotionEventProcessor {
	return &PromotionEventProcessor{
		updater: ruleUpdater,
	}
}

func (p *PromotionEventProcessor) Process(value []byte) error {
	var event promotionEvent

	if err := json.Unmarshal(value, &event); err != nil {
		return fmt.Errorf("err unmarshal promotion event:%w", err)
	}
	if event.EventID == "" {
		return ErrEventIdEmpty
	}
	if event.EventType != "promotion.created" {
		return fmt.Errorf("unsupported event type %s:%w", event.EventType, ErrEventType)
	}
	var rule domain.RulePromotion
	if err := json.Unmarshal(event.Payload, &rule); err != nil {
		return fmt.Errorf("err unmarshal rule promotion: %w", err)
	}
	if rule.ID == "" {
		return ErrPromotionIdEmpty
	}
	if err := p.updater.Upsert(rule); err != nil {
		return fmt.Errorf("err updater upsert: %w", err)
	}
	return nil
}
