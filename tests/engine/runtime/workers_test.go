//go:build ignore

package runtime

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestWorkerPoolRunsTasksAndCloses(t *testing.T) {
	p := NewWorkerPool(context.Background(), 2, 2)
	var count atomic.Int32
	for i := 0; i < 4; i++ {
		if err := p.Submit(func(context.Context) error { count.Add(1); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	p.Close()
	if count.Load() != 4 {
		t.Fatalf("tasks=%d", count.Load())
	}
	if err := p.Submit(func(context.Context) error { return nil }); err != ErrWorkerPoolClosed {
		t.Fatalf("submit after close: %v", err)
	}
}

