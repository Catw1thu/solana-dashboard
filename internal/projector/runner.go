package projector

import (
	"context"
	"fmt"
	"log"
	"time"

	"solana-dashboard-go/internal/events"
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

	entries, err := r.reader.ListServiceEventsAfterLogID(ctx, afterLogID, r.batchSize)
	if err != nil {
		return afterLogID, 0, err
	}
	if len(entries) == 0 {
		return afterLogID, 0, nil
	}

	lastLogID := afterLogID
	for _, entry := range entries {
		payload, err := events.DecodePayload(entry.Event)
		if err != nil {
			return lastLogID, processed, fmt.Errorf(
				"decode payload for log_id=%d event_id=%s: %w",
				entry.LogID,
				entry.Event.EventID,
				err,
			)
		}

		if err := r.projector.Project(ctx, &entry.Event, payload); err != nil {
			return lastLogID, processed, fmt.Errorf(
				"project log_id=%d event_id=%s: %w",
				entry.LogID,
				entry.Event.EventID,
				err,
			)
		}

		lastLogID = entry.LogID
		processed++
	}

	if err := r.reader.SaveProjectionCheckpoint(ctx, r.name, lastLogID); err != nil {
		return afterLogID, 0, err
	}

	return lastLogID, processed, nil
}
