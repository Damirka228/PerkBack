package kafka_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	consumer "github.com/Damirka228/PerkBack/services/benefit-service/internal/consumer/kafka"
	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
)

type fakeRuleUpdater struct {
	calls []domain.RulePromotion
	err   error
}

func (f *fakeRuleUpdater) Upsert(rule domain.RulePromotion) error {
	f.calls = append(f.calls, rule)
	return f.err
}

// Outer fields use the event envelope's JSON names; payload fields match the
// promotion JSON currently emitted by rules-service. Event and promotion IDs
// deliberately differ so the test catches confusion between the two.
const validPromotionEvent = `{
	"event_id": "00000000-0000-0000-0000-000000000001",
	"event_type": "promotion.created",
	"created_at": "2026-01-01T12:00:00Z",
	"payload": {
		"ID": "00000000-0000-0000-0000-000000000002",
		"ShopID": "магазин-42",
		"CategoryID": "groceries",
		"MinPurchaseAmountKop": 100000,
		"DiscountKop": 7000,
		"RateBPS": 300
	}
}`

func expectedPromotionRule() domain.RulePromotion {
	return domain.RulePromotion{
		ID:                   "00000000-0000-0000-0000-000000000002",
		ShopID:               "магазин-42",
		CategoryID:           "groceries",
		MinPurchaseAmountKop: 100_000,
		DiscountKop:          7_000,
		RateBPS:              300,
	}
}

func TestProcessValidPromotionEvent(t *testing.T) {
	updater := &fakeRuleUpdater{}
	processor := consumer.NewPromotionEventProcessor(updater)

	if err := processor.Process([]byte(validPromotionEvent)); err != nil {
		t.Fatalf("Process() error = %v, want nil", err)
	}
	want := []domain.RulePromotion{expectedPromotionRule()}
	if !slices.Equal(updater.calls, want) {
		t.Fatalf("Upsert calls = %+v, want %+v", updater.calls, want)
	}
}

func TestProcessRepeatedEventCallsUpsertAgain(t *testing.T) {
	updater := &fakeRuleUpdater{}
	processor := consumer.NewPromotionEventProcessor(updater)

	for i := 0; i < 2; i++ {
		if err := processor.Process([]byte(validPromotionEvent)); err != nil {
			t.Fatalf("Process() attempt %d: %v", i+1, err)
		}
	}
	// The processor delegates both deliveries. Idempotent storage of the rule
	// is the real OptionsProvider.Upsert's responsibility, not this fake's.
	rule := expectedPromotionRule()
	want := []domain.RulePromotion{rule, rule}
	if !slices.Equal(updater.calls, want) {
		t.Fatalf("Upsert calls = %+v, want two identical rules %+v", updater.calls, want)
	}
}

func TestProcessInvalidEventDoesNotCallUpsert(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr error
	}{
		{name: "empty_message", value: ""},
		{name: "malformed_json", value: `{"event_id":`},
		{name: "array_instead_of_envelope", value: `[]`},
		{name: "null_envelope", value: `null`, wantErr: consumer.ErrEventIdEmpty},
		{
			name:    "missing_event_id",
			value:   `{"event_type":"promotion.created","payload":{"ID":"promotion-1"}}`,
			wantErr: consumer.ErrEventIdEmpty,
		},
		{
			name:    "empty_event_id",
			value:   `{"event_id":"","event_type":"promotion.created","payload":{"ID":"promotion-1"}}`,
			wantErr: consumer.ErrEventIdEmpty,
		},
		{
			name:  "wrong_event_id_type",
			value: `{"event_id":123,"event_type":"promotion.created","payload":{"ID":"promotion-1"}}`,
		},
		{
			name:    "missing_event_type",
			value:   `{"event_id":"event-1","payload":{"ID":"promotion-1"}}`,
			wantErr: consumer.ErrEventType,
		},
		{
			name:    "unsupported_event_type",
			value:   `{"event_id":"event-1","event_type":"promotion.deleted","payload":{"ID":"promotion-1"}}`,
			wantErr: consumer.ErrEventType,
		},
		{
			name:  "invalid_created_at",
			value: `{"event_id":"event-1","event_type":"promotion.created","created_at":"invalid","payload":{"ID":"promotion-1"}}`,
		},
		{
			name:  "missing_payload",
			value: `{"event_id":"event-1","event_type":"promotion.created"}`,
		},
		{
			name:    "null_payload",
			value:   `{"event_id":"event-1","event_type":"promotion.created","payload":null}`,
			wantErr: consumer.ErrPromotionIdEmpty,
		},
		{
			name:    "missing_promotion_id",
			value:   `{"event_id":"event-1","event_type":"promotion.created","payload":{"ShopID":"shop-42"}}`,
			wantErr: consumer.ErrPromotionIdEmpty,
		},
		{
			name:    "empty_promotion_id",
			value:   `{"event_id":"event-1","event_type":"promotion.created","payload":{"ID":""}}`,
			wantErr: consumer.ErrPromotionIdEmpty,
		},
		{
			name:  "string_instead_of_payload_object",
			value: `{"event_id":"event-1","event_type":"promotion.created","payload":"not an object"}`,
		},
		{
			name:  "array_instead_of_payload_object",
			value: `{"event_id":"event-1","event_type":"promotion.created","payload":[]}`,
		},
		{
			name:  "wrong_promotion_id_type",
			value: `{"event_id":"event-1","event_type":"promotion.created","payload":{"ID":123}}`,
		},
		{
			name:  "wrong_amount_type",
			value: `{"event_id":"event-1","event_type":"promotion.created","payload":{"ID":"promotion-1","DiscountKop":"invalid"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := &fakeRuleUpdater{}
			processor := consumer.NewPromotionEventProcessor(updater)

			err := processor.Process([]byte(tt.value))
			if err == nil {
				t.Fatal("Process() must reject the invalid event")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Process() error = %v, want %v", err, tt.wantErr)
			}
			if len(updater.calls) != 0 {
				t.Fatalf("invalid event called Upsert %d times", len(updater.calls))
			}
		})
	}
}

func TestProcessPreservesJSONError(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "envelope_syntax", value: `{"event_id":`},
		{
			name:  "payload_field_type",
			value: `{"event_id":"event-1","event_type":"promotion.created","payload":{"ID":123}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := &fakeRuleUpdater{}
			err := consumer.NewPromotionEventProcessor(updater).Process([]byte(tt.value))
			if tt.name == "envelope_syntax" {
				var syntaxErr *json.SyntaxError
				if !errors.As(err, &syntaxErr) {
					t.Fatalf("Process() error = %v, want wrapped json.SyntaxError", err)
				}
			} else {
				var typeErr *json.UnmarshalTypeError
				if !errors.As(err, &typeErr) {
					t.Fatalf("Process() error = %v, want wrapped json.UnmarshalTypeError", err)
				}
			}
			if len(updater.calls) != 0 {
				t.Fatal("invalid JSON must not reach Upsert")
			}
		})
	}
}

func TestProcessPreservesUpsertError(t *testing.T) {
	upsertErr := errors.New("cannot update rules")
	updater := &fakeRuleUpdater{err: upsertErr}
	processor := consumer.NewPromotionEventProcessor(updater)

	err := processor.Process([]byte(validPromotionEvent))
	if !errors.Is(err, upsertErr) {
		t.Fatalf("Process() error = %v, want wrapped Upsert error %v", err, upsertErr)
	}
	if err == upsertErr {
		t.Fatal("Process() should add context to the Upsert error")
	}
	if want := []domain.RulePromotion{expectedPromotionRule()}; !slices.Equal(updater.calls, want) {
		t.Fatalf("Upsert calls = %+v, want %+v", updater.calls, want)
	}
}
