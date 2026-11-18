package kafka_test

import (
	"testing"
	"time"

	producerpkg "github.com/Damirka228/PerkBack/services/rules-service/internal/client/kafka"
)

func TestNewProducerAndClose(t *testing.T) {
	producer, err := producerpkg.NewProducer([]string{"localhost:9092"}, "promotions.events", time.Second)
	if err != nil {
		t.Fatalf("NewProducer(): %v", err)
	}
	t.Cleanup(func() { _ = producer.Close() })
	if err := producer.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
}
