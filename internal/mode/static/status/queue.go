package status

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

// QueueObject is the object to be passed to the queue for status updates.
type QueueObject struct {
	Error      error
	Deployment types.NamespacedName
}

// Queue represents a queue with unlimited size.
type Queue struct {
	notifyCh chan struct{}
	items    []*QueueObject

	lock sync.Mutex
}

// NewQueue returns a new Queue object.
func NewQueue() *Queue {
	return &Queue{
		items:    []*QueueObject{},
		notifyCh: make(chan struct{}, 1),
	}
}

// Enqueue adds an item to the queue and notifies any blocked readers.
func (q *Queue) Enqueue(item *QueueObject) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.items = append(q.items, item)

	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

// Dequeue removes and returns the front item from the queue.
// It blocks if the queue is empty or when the context is canceled.
func (q *Queue) Dequeue(ctx context.Context) *QueueObject {
	q.lock.Lock()
	defer q.lock.Unlock()

	for len(q.items) == 0 {
		q.lock.Unlock()
		select {
		case <-ctx.Done():
			q.lock.Lock()
			return nil
		case <-q.notifyCh:
			q.lock.Lock()
		}
	}

	front := q.items[0]
	q.items = q.items[1:]

	return front
}
