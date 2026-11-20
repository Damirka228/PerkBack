package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

var (
	ErrProcessor    = errors.New("processor is empty")
	ErrGroupIDEmpty = errors.New("group id is empty")
)

type Consumer struct {
	reader    *kafka.Reader
	processor *PromotionEventProcessor
}

func NewConsumer(brokers []string, topic string, groupID string, processor *PromotionEventProcessor) (*Consumer, error) {
	if processor == nil {
		return nil, ErrProcessor
	}
	if strings.TrimSpace(groupID) == "" {
		return nil, ErrGroupIDEmpty
	}
	readerConfig := kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10_000_000,
		MaxWait:        500 * time.Millisecond,
		CommitInterval: 0,
		StartOffset:    kafka.FirstOffset,
	}
	if err := readerConfig.Validate(); err != nil {
		return nil, fmt.Errorf("validate kafka reader: %w", err)
	}
	reader := kafka.NewReader(readerConfig)
	return &Consumer{
		reader:    reader,
		processor: processor,
	}, nil
}

func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("close consumer: %w", err)
	}
	return nil
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch kafka message: %w", err)
		}
		if err := c.processor.Process(msg.Value); err != nil {
			return fmt.Errorf(
				"process kafka message partition=%d offset=%d: %w",
				msg.Partition,
				msg.Offset,
				err,
			)
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf(
				"commit kafka message partition=%d offset=%d: %w",
				msg.Partition,
				msg.Offset,
				err,
			)
		}
	}
}
