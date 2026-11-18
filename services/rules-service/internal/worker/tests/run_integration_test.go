package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/rules-service/internal/domain"
	"github.com/Damirka228/PerkBack/services/rules-service/internal/worker"
	"github.com/sirupsen/logrus"
)

// Observe completed batches without reading the publisher concurrently.
type workerLogHook struct {
	entries chan *logrus.Entry
}

func (h *workerLogHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.InfoLevel, logrus.ErrorLevel}
}

func (h *workerLogHook) Fire(entry *logrus.Entry) error {
	copy := entry.Dup()
	copy.Level = entry.Level
	copy.Message = entry.Message
	select {
	case h.entries <- copy:
	default:
	}
	return nil
}

func startTestWorker(
	t *testing.T,
	outboxWorker *worker.OutboxWorker,
	pollInterval, retryInterval time.Duration,
) (context.CancelFunc, <-chan error, <-chan *logrus.Entry) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	log := newWorkerTestLogger()
	hook := &workerLogHook{entries: make(chan *logrus.Entry, 16)}
	log.AddHook(hook)
	done := make(chan error, 1)
	go func() {
		done <- outboxWorker.Run(ctx, pollInterval, retryInterval, log)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		waitWorkerStopped(t, done)
	})
	return cancel, done, hook.entries
}

func waitWorkerStopped(t *testing.T, done <-chan error) {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned an error: %v", err)
		}
	case <-timer.C:
		t.Fatal("Run() did not stop after context cancellation")
	}
}

func waitWorkerLog(t *testing.T, ctx context.Context, entries <-chan *logrus.Entry, wantLevel logrus.Level) {
	t.Helper()
	select {
	case entry := <-entries:
		if entry.Level != wantLevel {
			t.Fatalf("worker log level = %v, want %v (message: %s)", entry.Level, wantLevel, entry.Message)
		}
	case <-ctx.Done():
		t.Fatalf("waiting for worker log: %v", ctx.Err())
	}
}

func TestRunDrainsBatchesWithoutPollingDelayIntegration(t *testing.T) {
	repo, pool := newWorkerTestStorage(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	events := insertPendingEvents(t, ctx, pool)
	publisher := &fakePublisher{}
	// One event per batch means draining the queue requires two iterations.
	outboxWorker := worker.NewOutboxWorker(repo, publisher, 1)
	stop, done, entries := startTestWorker(t, outboxWorker, time.Hour, time.Hour)

	waitWorkerLog(t, ctx, entries, logrus.InfoLevel)
	waitWorkerLog(t, ctx, entries, logrus.InfoLevel)
	stop()
	waitWorkerStopped(t, done)

	assertPublishCalls(t, publisher.calls, events)
	assertEventStatuses(t, ctx, pool, events, "done")
}

func TestRunRetriesPublishFailureIntegration(t *testing.T) {
	repo, pool := newWorkerTestStorage(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	events := insertPendingEvents(t, ctx, pool)
	publisher := &fakePublisher{failOn: 2, err: errors.New("temporary publisher failure")}
	outboxWorker := worker.NewOutboxWorker(repo, publisher, 10)
	stop, done, entries := startTestWorker(t, outboxWorker, time.Hour, 10*time.Millisecond)

	waitWorkerLog(t, ctx, entries, logrus.ErrorLevel)
	waitWorkerLog(t, ctx, entries, logrus.InfoLevel)
	stop()
	waitWorkerStopped(t, done)

	wantCalls := []domain.OutboxEvent{events[0], events[1], events[0], events[1]}
	assertPublishCalls(t, publisher.calls, wantCalls)
	assertEventStatuses(t, ctx, pool, events, "done")
}

func TestRunCancellationInterruptsRetryWaitIntegration(t *testing.T) {
	repo, pool := newWorkerTestStorage(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	events := insertPendingEvents(t, ctx, pool)
	publisher := &fakePublisher{failOn: 1, err: errors.New("publisher unavailable")}
	outboxWorker := worker.NewOutboxWorker(repo, publisher, 10)
	stop, done, entries := startTestWorker(t, outboxWorker, time.Hour, time.Hour)

	waitWorkerLog(t, ctx, entries, logrus.ErrorLevel)
	assertEventStatuses(t, ctx, pool, events, "new")
	stop()
	waitWorkerStopped(t, done)

	// An hour-long retry delay must not prevent prompt shutdown or cause a retry.
	assertPublishCalls(t, publisher.calls, events[:1])
}
