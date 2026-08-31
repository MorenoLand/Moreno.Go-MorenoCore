package runtime

import (
	"context"
	"errors"
	"sync"
)

var ErrWorkerPoolClosed = errors.New("worker pool is closed")

type Task func(context.Context) error

type WorkerPool struct {
	ctx    context.Context
	cancel context.CancelFunc
	tasks  chan Task
	wg     sync.WaitGroup
	mu     sync.RWMutex
	closed bool
}

func NewWorkerPool(parent context.Context, workers, queue int) *WorkerPool {
	if workers < 1 {
		workers = 1
	}
	if queue < workers {
		queue = workers
	}
	ctx, cancel := context.WithCancel(parent)
	p := &WorkerPool{ctx: ctx, cancel: cancel, tasks: make(chan Task, queue)}
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker()
	}
	return p
}

func (p *WorkerPool) Submit(task Task) error {
	if task == nil {
		return errors.New("worker task is nil")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrWorkerPoolClosed
	}
	select {
	case p.tasks <- task:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

func (p *WorkerPool) Close() {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		p.cancel()
	}
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task := <-p.tasks:
			if task != nil {
				_ = task(p.ctx)
			}
		case <-p.ctx.Done():
			return
		}
	}
}
