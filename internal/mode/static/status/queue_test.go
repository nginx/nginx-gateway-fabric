package status

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
)

func TestNewQueue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	q := NewQueue()

	g.Expect(q).ToNot(BeNil())
	g.Expect(q.items).To(BeEmpty())
}

func TestEnqueue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	q := NewQueue()
	item := &QueueObject{
		Error:      nil,
		Deployment: types.NamespacedName{Namespace: "default", Name: "test-object"},
	}
	q.Enqueue(item)

	g.Expect(q.items).To(HaveLen(1))
	g.Expect(q.items[0]).To(Equal(item))
}

func TestDequeue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	q := NewQueue()
	item := &QueueObject{
		Error:      nil,
		Deployment: types.NamespacedName{Namespace: "default", Name: "test-object"},
	}
	q.Enqueue(item)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dequeuedItem := q.Dequeue(ctx)
	g.Expect(dequeuedItem).To(Equal(item))
	g.Expect(q.items).To(BeEmpty())
}

func TestDequeueEmptyQueue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	q := NewQueue()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dequeuedItem := q.Dequeue(ctx)
	g.Expect(dequeuedItem).To(BeNil())
}

func TestDequeueWithMultipleItems(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	q := NewQueue()
	item1 := &QueueObject{
		Error:      nil,
		Deployment: types.NamespacedName{Namespace: "default", Name: "test-object-1"},
	}
	item2 := &QueueObject{
		Error:      nil,
		Deployment: types.NamespacedName{Namespace: "default", Name: "test-object-2"},
	}
	q.Enqueue(item1)
	q.Enqueue(item2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dequeuedItem1 := q.Dequeue(ctx)
	g.Expect(dequeuedItem1).To(Equal(item1))

	dequeuedItem2 := q.Dequeue(ctx)

	g.Expect(dequeuedItem2).To(Equal(item2))
	g.Expect(q.items).To(BeEmpty())
}
