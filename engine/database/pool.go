package database

import (
	"context"
	"database/sql"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/runtime"
)

type QueryResult struct {
	Rows *sql.Rows
	Err  error
}

type AsyncPool struct {
	store   *Store
	workers *runtime.WorkerPool
}

func NewAsyncPool(parent context.Context, store *Store, workers, queue int) *AsyncPool {
	return &AsyncPool{store: store, workers: runtime.NewWorkerPool(parent, workers, queue)}
}

func (p *AsyncPool) Query(ctx context.Context, id StatementID, args ...any) <-chan QueryResult {
	result := make(chan QueryResult, 1)
	if err := p.workers.Submit(func(taskCtx context.Context) error {
		rows, err := p.store.QueryStatement(taskCtx, id, args...)
		result <- QueryResult{Rows: rows, Err: err}
		return nil
	}); err != nil {
		result <- QueryResult{Err: err}
	}
	return result
}

func (p *AsyncPool) Exec(ctx context.Context, id StatementID, args ...any) <-chan error {
	result := make(chan error, 1)
	if err := p.workers.Submit(func(taskCtx context.Context) error {
		_, err := p.store.ExecStatement(taskCtx, id, args...)
		result <- err
		return nil
	}); err != nil {
		result <- err
	}
	return result
}

func (p *AsyncPool) Close() { p.workers.Close() }
