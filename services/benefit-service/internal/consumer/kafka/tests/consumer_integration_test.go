package kafka_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/domain"
)

func TestConsumerRunCommitsAndResumesIntegration(t *testing.T) {
	fixture := newConsumerKafkaFixture(t)
	first := expectedPromotionRule()
	second := first
	second.ID = "00000000-0000-0000-0000-000000000003"
	fixture.publish(t, consumerEventValue(t, first), consumerEventValue(t, second))
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()

	// Process offset 0 successfully, then stop with an error at offset 1.
	stopErr := errors.New("stop on second event")
	var calls []domain.RulePromotion
	client := fixture.newConsumer(t, ruleUpdaterFunc(func(rule domain.RulePromotion) error {
		calls = append(calls, rule)
		if rule.ID == second.ID {
			return stopErr
		}
		return nil
	}))
	err := client.Run(ctx)
	if !errors.Is(err, stopErr) {
		t.Fatalf("Run() error = %v, want %v", err, stopErr)
	}
	if !strings.Contains(err.Error(), "partition=0 offset=1") {
		t.Fatalf("error must identify the failed partition and offset: %v", err)
	}
	if want := []domain.RulePromotion{first, second}; !slices.Equal(calls, want) {
		t.Fatalf("Upsert calls = %+v, want %+v", calls, want)
	}
	// Kafka commits the NEXT offset. Committed offset 1 means offset 0 is done.
	if got := fixture.committedOffset(t, ctx); got != 1 {
		t.Fatalf("committed offset = %d, want 1", got)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	// A fresh reader in the same group must start at the failed second event,
	// not replay the successfully committed first event.
	probeErr := errors.New("stop replay probe")
	updater := &fakeRuleUpdater{err: probeErr}
	restarted := fixture.newConsumer(t, updater)
	if err := restarted.Run(ctx); !errors.Is(err, probeErr) {
		t.Fatalf("restarted Run() error = %v, want %v", err, probeErr)
	}
	if want := []domain.RulePromotion{second}; !slices.Equal(updater.calls, want) {
		t.Fatalf("restarted Upsert calls = %+v, want only %+v", updater.calls, second)
	}
	if got := fixture.committedOffset(t, ctx); got != 1 {
		t.Fatalf("failed replay changed committed offset to %d, want 1", got)
	}
}

func TestConsumerRunFailedMessageIsRedeliveredIntegration(t *testing.T) {
	fixture := newConsumerKafkaFixture(t)
	first := expectedPromotionRule()
	second := first
	second.ID = "00000000-0000-0000-0000-000000000003"
	fixture.publish(t, consumerEventValue(t, first), consumerEventValue(t, second))
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()

	upsertErr := errors.New("rules update failed")
	updater := &fakeRuleUpdater{err: upsertErr}
	client := fixture.newConsumer(t, updater)
	if err := client.Run(ctx); !errors.Is(err, upsertErr) {
		t.Fatalf("Run() error = %v, want %v", err, upsertErr)
	}
	if want := []domain.RulePromotion{first}; !slices.Equal(updater.calls, want) {
		t.Fatalf("consumer must stop on the first failure, calls = %+v", updater.calls)
	}
	if got := fixture.committedOffset(t, ctx); got != -1 {
		t.Fatalf("failed processing committed offset %d, want no committed offset (-1)", got)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	// Recover processing of the first event; fail at the second to stop the
	// loop deterministically after the first event has been committed.
	stopErr := errors.New("stop after recovery")
	var retried []domain.RulePromotion
	restarted := fixture.newConsumer(t, ruleUpdaterFunc(func(rule domain.RulePromotion) error {
		retried = append(retried, rule)
		if rule.ID == second.ID {
			return stopErr
		}
		return nil
	}))
	if err := restarted.Run(ctx); !errors.Is(err, stopErr) {
		t.Fatalf("restarted Run() error = %v, want %v", err, stopErr)
	}
	if want := []domain.RulePromotion{first, second}; !slices.Equal(retried, want) {
		t.Fatalf("retry must start with the original failed rule: got %+v, want %+v", retried, want)
	}
	if got := fixture.committedOffset(t, ctx); got != 1 {
		t.Fatalf("offset after successful retry = %d, want 1", got)
	}
}

func TestConsumerRunInvalidJSONIsNotCommittedIntegration(t *testing.T) {
	fixture := newConsumerKafkaFixture(t)
	fixture.publish(t, []byte(`{"event_id":`), []byte(validPromotionEvent))
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	updater := &fakeRuleUpdater{}
	client := fixture.newConsumer(t, updater)

	err := client.Run(ctx)
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("Run() error = %v, want wrapped json.SyntaxError", err)
	}
	if len(updater.calls) != 0 {
		t.Fatalf("malformed event must stop processing before Upsert; calls = %+v", updater.calls)
	}
	if got := fixture.committedOffset(t, ctx); got != -1 {
		t.Fatalf("malformed event committed offset %d, want -1", got)
	}
}

func TestConsumerRunCancellationWhileFetchingIntegration(t *testing.T) {
	fixture := newConsumerKafkaFixture(t)
	rule := expectedPromotionRule()
	fixture.publish(t, consumerEventValue(t, rule))
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	runCtx, stop := context.WithCancel(ctx)
	updater := &fakeRuleUpdater{}
	client := fixture.newConsumer(t, updater)
	done := make(chan error, 1)
	go func() {
		done <- client.Run(runCtx)
		close(done)
	}()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("consumer did not stop during cleanup")
		}
	})

	// Wait for actual progress, not an arbitrary sleep. Once offset 1 is
	// committed, there are no more messages and Run waits for new input.
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for fixture.committedOffset(t, ctx) != 1 {
		select {
		case err := <-done:
			t.Fatalf("Run() stopped before committing the event: %v", err)
		case <-ctx.Done():
			t.Fatalf("waiting for committed offset: %v", ctx.Err())
		case <-ticker.C:
		}
	}

	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error on cancellation = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("context cancellation did not interrupt FetchMessage")
	}
	// Read the fake's state only after Run exits, avoiding a test data race.
	if want := []domain.RulePromotion{rule}; !slices.Equal(updater.calls, want) {
		t.Fatalf("Upsert calls = %+v, want %+v", updater.calls, want)
	}
}
