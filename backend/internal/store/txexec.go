package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type dbExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type txContextKey struct{}

func contextWithExecutor(ctx context.Context, executor dbExecutor) context.Context {
	return context.WithValue(ctx, txContextKey{}, executor)
}

func executorFromContext(ctx context.Context, fallback dbExecutor) dbExecutor {
	if executor, ok := ctx.Value(txContextKey{}).(dbExecutor); ok && executor != nil {
		return executor
	}
	return fallback
}
