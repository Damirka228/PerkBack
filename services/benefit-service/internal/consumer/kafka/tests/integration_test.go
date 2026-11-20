package kafka_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	consumer "github.com/Damirka228/PerkBack/services/benefit-service/internal/consumer/kafka"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
)

// TEST_KAFKA_BROKERS=localhost:9092 go test ./services/benefit-service/internal/consumer/kafka/tests -count=1
//
// Every integration test owns a unique topic and consumer group. Cleanup only
// deletes these generated resources, never the application's topic or groups.
type consumerKafkaFixture struct {
	brokers []string
	topic   string
	groupID string
	admin   *kafkago.Client
}

func newConsumerKafkaFixture(t *testing.T) *consumerKafkaFixture {
	t.Helper()
	addresses := strings.TrimSpace(os.Getenv("TEST_KAFKA_BROKERS"))
	if addresses == "" {
		t.Skip("set TEST_KAFKA_BROKERS to run Kafka integration tests")
	}
	brokers := strings.Split(addresses, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
		if brokers[i] == "" {
			t.Fatal("TEST_KAFKA_BROKERS contains an empty address")
		}
	}
	transport := &kafkago.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	fixture := &consumerKafkaFixture{
		brokers: brokers,
		topic:   "perkback-consumer-test-" + uuid.NewString(),
		groupID: "perkback-consumer-test-" + uuid.NewString(),
		admin: &kafkago.Client{
			Addr:      kafkago.TCP(brokers...),
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	response, err := fixture.admin.CreateTopics(ctx, &kafkago.CreateTopicsRequest{
		Topics: []kafkago.TopicConfig{{
			Topic:             fixture.topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		}},
	})
	if err != nil {
		t.Fatalf("create test topic: %v", err)
	}
	if err := response.Errors[fixture.topic]; err != nil {
		t.Fatalf("create test topic: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		response, err := fixture.admin.DeleteTopics(ctx, &kafkago.DeleteTopicsRequest{
			Topics: []string{fixture.topic},
		})
		if err != nil {
			t.Errorf("delete test topic: %v", err)
		} else if err := response.Errors[fixture.topic]; err != nil {
			t.Errorf("delete test topic: %v", err)
		}
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		response, err := fixture.admin.DeleteGroups(ctx, &kafkago.DeleteGroupsRequest{
			GroupIDs: []string{fixture.groupID},
		})
		if err != nil {
			t.Errorf("delete test consumer group: %v", err)
		} else if err := response.Errors[fixture.groupID]; err != nil && !errors.Is(err, kafkago.GroupIdNotFound) {
			t.Errorf("delete test consumer group: %v", err)
		}
	})
	fixture.waitTopicReady(t, ctx)
	return fixture
}

func (f *consumerKafkaFixture) waitTopicReady(t *testing.T, ctx context.Context) {
	t.Helper()
	// CreateTopics may return before broker metadata exposes the new leader.
	// Wait for readiness rather than racing the first publication.
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		response, err := f.admin.Metadata(ctx, &kafkago.MetadataRequest{Topics: []string{f.topic}})
		if err == nil {
			for _, topic := range response.Topics {
				if topic.Name != f.topic || topic.Error != nil {
					continue
				}
				for _, partition := range topic.Partitions {
					if partition.ID == 0 && partition.Error == nil &&
						partition.Leader.Host != "" && partition.Leader.Port > 0 &&
						len(partition.Isr) > 0 {
						return
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for test topic leader: %v (last request error: %v)", ctx.Err(), err)
		case <-ticker.C:
		}
	}
}

func (f *consumerKafkaFixture) newConsumer(t *testing.T, updater consumer.RuleUpdater) *consumer.Consumer {
	t.Helper()
	client, err := consumer.NewConsumer(
		f.brokers, f.topic, f.groupID,
		consumer.NewPromotionEventProcessor(updater),
	)
	if err != nil {
		t.Fatalf("NewConsumer(): %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close(): %v", err)
		}
	})
	return client
}

func (f *consumerKafkaFixture) publish(t *testing.T, values ...[]byte) {
	t.Helper()
	// Do not reuse DefaultTransport's metadata cache across disposable topics.
	transport := &kafkago.Transport{}
	defer transport.CloseIdleConnections()
	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(f.brokers...),
		Topic:        f.topic,
		RequiredAcks: kafkago.RequireAll,
		BatchSize:    1,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Transport:    transport,
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Errorf("close test writer: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	// One partition and sequential writes give deterministic offsets 0, 1, ...
	for _, value := range values {
		if err := writer.WriteMessages(ctx, kafkago.Message{Value: value}); err != nil {
			t.Fatalf("publish test event: %v", err)
		}
	}
}

func (f *consumerKafkaFixture) committedOffset(t *testing.T, ctx context.Context) int64 {
	t.Helper()
	response, err := f.admin.OffsetFetch(ctx, &kafkago.OffsetFetchRequest{
		GroupID: f.groupID,
		Topics:  map[string][]int{f.topic: {0}},
	})
	if err != nil {
		t.Fatalf("fetch group offset: %v", err)
	}
	if errors.Is(response.Error, kafkago.GroupIdNotFound) {
		return -1
	}
	if response.Error != nil {
		t.Fatalf("fetch group offset: %v", response.Error)
	}
	for _, partition := range response.Topics[f.topic] {
		if partition.Partition == 0 {
			if partition.Error != nil {
				t.Fatalf("fetch partition offset: %v", partition.Error)
			}
			return partition.CommittedOffset
		}
	}
	t.Fatal("offset response is missing the test partition")
	return -1
}

func consumerEventValue(t *testing.T, rule domain.RulePromotion) []byte {
	t.Helper()
	value, err := json.Marshal(struct {
		EventID   string               `json:"event_id"`
		EventType string               `json:"event_type"`
		CreatedAt time.Time            `json:"created_at"`
		Payload   domain.RulePromotion `json:"payload"`
	}{
		EventID:   uuid.NewString(),
		EventType: "promotion.created",
		CreatedAt: time.Now().UTC(),
		Payload:   rule,
	})
	if err != nil {
		t.Fatalf("encode test event: %v", err)
	}
	return value
}

type ruleUpdaterFunc func(domain.RulePromotion) error

func (f ruleUpdaterFunc) Upsert(rule domain.RulePromotion) error {
	return f(rule)
}
