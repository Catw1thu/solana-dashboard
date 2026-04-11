package projector

import (
	"context"
	"fmt"
	"log"
	"time"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/observability"
	"solana-dashboard-go/internal/store"
)

const (
	defaultReplayBatchSize = 256
	defaultPollInterval    = 250 * time.Millisecond
)

type logReader interface {
	ListServiceEventsAfterLogID(ctx context.Context, afterLogID int64, limit int) ([]store.ServiceEventLogEntry, error)
	LoadProjectionCheckpoint(ctx context.Context, projectorName string) (int64, error)
	SaveProjectionCheckpoint(ctx context.Context, projectorName string, lastLogID int64) error
	LoadLatestServiceEventLogID(ctx context.Context) (int64, error)
	RunInTransaction(ctx context.Context, fn func(context.Context) error) error
}

type Runner struct {
	name         string
	reader       logReader
	projector    *Projector
	batchSize    int
	pollInterval time.Duration
}

func NewRunner(name string, reader logReader, projector *Projector) *Runner {
	return &Runner{
		name:         name,
		reader:       reader,
		projector:    projector,
		batchSize:    defaultReplayBatchSize,
		pollInterval: defaultPollInterval,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	lastLogID, err := r.reader.LoadProjectionCheckpoint(ctx, r.name)
	if err != nil {
		return err
	}

	log.Printf("projector %s starting from log_id=%d", r.name, lastLogID)

	for {
		nextLogID, processed, err := r.ReplayBatch(ctx, lastLogID)
		if err != nil {
			return err
		}
		lastLogID = nextLogID
		if processed > 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(r.pollInterval):
		}
	}
}

func (r *Runner) ReplayFromStart(ctx context.Context) (int64, error) {
	lastLogID := int64(0)

	for {
		nextLogID, processed, err := r.ReplayBatch(ctx, lastLogID)
		if err != nil {
			return lastLogID, err
		}
		lastLogID = nextLogID
		if processed == 0 {
			return lastLogID, nil
		}
	}
}

func (r *Runner) ReplayBatch(
	ctx context.Context,
	afterLogID int64,
) (nextLogID int64, processed int, err error) {
	if r.projector == nil {
		return afterLogID, 0, nil
	}

	start := time.Now()
	entries, err := r.reader.ListServiceEventsAfterLogID(ctx, afterLogID, r.batchSize)
	if err != nil {
		observability.Default().IncCounter("projector_replay_errors_total", 1)
		return afterLogID, 0, err
	}
	if len(entries) == 0 {
		return afterLogID, 0, nil
	}

	lastLogID := afterLogID
	writer := r.projector.store
	if batchStore, ok := writer.(batchCapableWriter); ok {
		writer = newWriteBatch(batchStore)
	}

	if err := r.reader.RunInTransaction(ctx, func(txCtx context.Context) error {
		batchProjector := r.projector.WithStore(writer)
		for _, entry := range entries {
			payload, err := events.DecodePayload(entry.Event)
			if err != nil {
				return fmt.Errorf(
					"decode payload for log_id=%d event_id=%s: %w",
					entry.LogID,
					entry.Event.EventID,
					err,
				)
			}

			if err := batchProjector.Project(txCtx, &entry.Event, payload); err != nil {
				return fmt.Errorf(
					"project log_id=%d event_id=%s: %w",
					entry.LogID,
					entry.Event.EventID,
					err,
				)
			}

			lastLogID = entry.LogID
			processed++
		}

		if batch, ok := writer.(*writeBatch); ok {
			if err := batch.Flush(txCtx); err != nil {
				return err
			}
		}

		if err := r.reader.SaveProjectionCheckpoint(txCtx, r.name, lastLogID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		observability.Default().IncCounter("projector_replay_errors_total", 1)
		return lastLogID, processed, err
	}

	observability.Default().IncCounter("projector_batches_total", 1)
	observability.Default().IncCounter("projector_events_total", int64(processed))
	observability.Default().ObserveDuration("projector_batch_latency_ms", time.Since(start))
	observability.Default().SetGauge("projector_last_checkpoint_log_id", lastLogID)
	observability.Default().SetGauge("projector_last_projected_event_unix", entries[len(entries)-1].Event.EventUnixTS)
	if latestServiceLogID, err := r.reader.LoadLatestServiceEventLogID(ctx); err == nil {
		lagLogID := latestServiceLogID - lastLogID
		if lagLogID < 0 {
			lagLogID = 0
		}
		observability.Default().SetGauge("projector_log_id_lag", lagLogID)
	}
	lagSeconds := time.Now().Unix() - entries[len(entries)-1].Event.EventUnixTS
	if lagSeconds < 0 {
		lagSeconds = 0
	}
	observability.Default().SetGauge("projector_event_time_lag_seconds", lagSeconds)

	return lastLogID, processed, nil
}
