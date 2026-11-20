package kafka_test

import (
	"context"
	"errors"
	"testing"

	consumer "github.com/Damirka228/PerkBack/services/benefit-service/internal/consumer/kafka"
)

func TestNewConsumerValidation(t *testing.T) {
	processor := consumer.NewPromotionEventProcessor(&fakeRuleUpdater{})
	tests := []struct {
		name      string
		brokers   []string
		topic     string
		groupID   string
		processor *consumer.PromotionEventProcessor
		wantErr   error
	}{
		{
			name: "nil_processor", brokers: []string{"localhost:9092"}, topic: "test-topic", groupID: "test-group",
			wantErr: consumer.ErrProcessor,
		},
		{
			name: "empty_group", brokers: []string{"localhost:9092"}, topic: "test-topic", processor: processor,
			wantErr: consumer.ErrGroupIDEmpty,
		},
		{
			name: "whitespace_group", brokers: []string{"localhost:9092"}, topic: "test-topic", groupID: " \t\n", processor: processor,
			wantErr: consumer.ErrGroupIDEmpty,
		},
		{
			name: "empty_brokers", topic: "test-topic", groupID: "test-group", processor: processor,
		},
		{
			name: "empty_topic", brokers: []string{"localhost:9092"}, groupID: "test-group", processor: processor,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := consumer.NewConsumer(tt.brokers, tt.topic, tt.groupID, tt.processor)
			if client != nil {
				_ = client.Close()
				t.Fatal("NewConsumer() returned a consumer for invalid arguments")
			}
			if err == nil {
				t.Fatal("NewConsumer() must reject invalid arguments")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestConsumerRunAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// Cancellation must be checked before touching the reader or processor.
	client := &consumer.Consumer{}
	if err := client.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v, want nil on cancellation", err)
	}
}
