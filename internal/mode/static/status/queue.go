package status

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// UpdateType is the type of status update to perform.
type UpdateType int

const (
	// UpdateAll means to update statuses of all Gateway API resources.
	UpdateAll = iota
	// UpdateGateway means to just update the status of the Gateway resource.
	UpdateGateway
)

// QueueObject is the object to be passed to the queue for status updates.
type QueueObject struct {
	// GatewayService is the Gateway Service that was updated. When set, UpdateType should be UpdateGateway.
	// Set by the provisioner
	GatewayService *corev1.Service
	Error          error
	Deployment     types.NamespacedName
	UpdateType     UpdateType
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
