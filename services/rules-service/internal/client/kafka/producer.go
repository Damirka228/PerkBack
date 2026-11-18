package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	"github.com/segmentio/kafka-go"
)

var (
	ErrInvalidBrokers     = errors.New("brokers is empty")
	ErrInvalidTopic       = errors.New("topic is empty")
	ErrInvalidTimeout     = errors.New("timeout <= 0")
	ErrBrokerIsEmpty      = errors.New("broker is empty")
	ErrPromotionIDIsEmpty = errors.New("promotion id is empty")
)

type Producer struct {
	writer         *kafka.Writer
	publishTimeout time.Duration
}

func NewProducer(brokers []string, topic string, publishTimeout time.Duration) (*Producer, error) {
	if err := validateProducer(brokers, topic, publishTimeout); err != nil {
		return nil, fmt.Errorf("validate kafka producer: %w", err)
	}
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			BatchSize:    1,
			ReadTimeout:  publishTimeout,
			WriteTimeout: publishTimeout,
		},
		publishTimeout: publishTimeout,
	}, nil
}

func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close kafka producer: %w", err)
	}
	return nil
}

type eventMessage struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	CreatedAt time.Time       `json:"created_at"`
	Payload   json.RawMessage `json:"payload"`
}

func (p *Producer) Publish(ctx context.Context, event domain.OutboxEvent) error {
	var promotion domain.Promotion
	if err := json.Unmarshal(event.Payload, &promotion); err != nil {
		return fmt.Errorf("decode promotion payload:%w", err)
	}
	if promotion.ID == "" {
		return ErrPromotionIDIsEmpty
	}

	message := eventMessage{
		EventID:   event.ID,
		EventType: event.EventType,
		CreatedAt: event.CreatedAt,
		Payload:   event.Payload,
	}

	value, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode kafka event:%w", err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()

	err = p.writer.WriteMessages(publishCtx, kafka.Message{
		Key:   []byte(promotion.ID),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("publish event %s:%w", event.ID, err)
	}
	return nil
}

func validateProducer(brokers []string, topic string, publishTimeout time.Duration) error {
	if len(brokers) == 0 {
		return ErrInvalidBrokers
	}
	for _, broker := range brokers {
		if broker == "" {
			return ErrBrokerIsEmpty
		}
	}
	if topic == "" {
		return ErrInvalidTopic
	}
	if publishTimeout <= 0 {
		return ErrInvalidTimeout
	}
	return nil
}
