package runtime

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

type Clock interface{ Now() time.Time }

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type event struct {
	id        uint64
	due       time.Time
	fn        func(context.Context)
	cancelled bool
	index     int
}

type eventHeap []*event

func (h eventHeap) Len() int { return len(h) }
func (h eventHeap) Less(i, j int) bool {
	if h[i].due.Equal(h[j].due) {
		return h[i].id < h[j].id
	}
	return h[i].due.Before(h[j].due)
}
func (h eventHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *eventHeap) Push(value any) {
	item := value.(*event)
	item.index = len(*h)
	*h = append(*h, item)
}
func (h *eventHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[:n-1]
	return item
}

type EventQueue struct {
	mu     sync.Mutex
	next   uint64
	events eventHeap
}

func NewEventQueue() *EventQueue { return &EventQueue{events: make(eventHeap, 0)} }

func (q *EventQueue) Add(due time.Time, fn func(context.Context)) uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.next++
	item := &event{id: q.next, due: due, fn: fn}
	heap.Push(&q.events, item)
	return item.id
}

func (q *EventQueue) AddAfter(now time.Time, delay time.Duration, fn func(context.Context)) uint64 {
	return q.Add(now.Add(delay), fn)
}

func (q *EventQueue) Cancel(id uint64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, item := range q.events {
		if item.id == id && !item.cancelled {
			item.cancelled = true
			return true
		}
	}
	return false
}

func (q *EventQueue) RunDue(ctx context.Context, now time.Time) int {
	ready := make([]*event, 0)
	q.mu.Lock()
	for q.events.Len() > 0 {
		item := q.events[0]
		if item.due.After(now) {
			break
		}
		heap.Pop(&q.events)
		if !item.cancelled && item.fn != nil {
			ready = append(ready, item)
		}
	}
	q.mu.Unlock()
	for _, item := range ready {
		item.fn(ctx)
	}
	return len(ready)
}

func (q *EventQueue) NextDue() (time.Time, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.events.Len() > 0 && q.events[0].cancelled {
		heap.Pop(&q.events)
	}
	if q.events.Len() == 0 {
		return time.Time{}, false
	}
	return q.events[0].due, true
}
