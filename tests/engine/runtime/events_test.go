//go:build ignore

package runtime

import (
	"context"
	"testing"
	"time"
)

func TestEventQueueOrderingAndCancellation(t *testing.T) {
	q := NewEventQueue()
	now := time.Unix(100, 0)
	seen := make([]uint64, 0, 2)
	q.AddAfter(now, 2*time.Second, func(context.Context) { seen = append(seen, 2) })
	cancel := q.AddAfter(now, time.Second, func(context.Context) { seen = append(seen, 1) })
	if !q.Cancel(cancel) {
		t.Fatal("event was not cancelled")
	}
	q.AddAfter(now, 2*time.Second, func(context.Context) { seen = append(seen, 3) })
	if count := q.RunDue(context.Background(), now.Add(time.Second)); count != 0 {
		t.Fatalf("due count=%d", count)
	}
	if count := q.RunDue(context.Background(), now.Add(2*time.Second)); count != 2 {
		t.Fatalf("due count=%d", count)
	}
	if len(seen) != 2 || seen[0] != 2 || seen[1] != 3 {
		t.Fatalf("events=%v", seen)
	}
}

