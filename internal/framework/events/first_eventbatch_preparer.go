package events

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/kubernetes"
)

//counterfeiter:generate . FirstEventBatchPreparer

// FirstEventBatchPreparer prepares the first batch of events to be processed by the EventHandler.
// The first batch includes the UpsertEvents for all relevant resources in the cluster.
type FirstEventBatchPreparer interface {
	// Prepare prepares the first event batch.
	Prepare(ctx context.Context) (EventBatch, error)
}

// EachListItemFunc lists each item of a client.ObjectList.
// It is from k8s.io/apimachinery/pkg/api/meta.
type EachListItemFunc func(obj runtime.Object, fn func(runtime.Object) error) error

// FirstEventBatchPreparerImpl is an implementation of FirstEventBatchPreparer.
type FirstEventBatchPreparerImpl struct {
	reader       kubernetes.Reader
	eachListItem EachListItemFunc
	objects      []client.Object
	objectLists  []client.ObjectList
}

// NewFirstEventBatchPreparerImpl creates a new FirstEventBatchPreparerImpl.
// objects and objectList specify which resources will be included in the first batch.
// For each object from objects, FirstEventBatchPreparerImpl will get the corresponding resource from the reader.
// The object must specify its namespace (if any) and name.
// For each list from objectLists, FirstEventBatchPreparerImpl will list the resources of the corresponding type from
// the reader.
func NewFirstEventBatchPreparerImpl(
	reader kubernetes.Reader,
	objects []client.Object,
	objectLists []client.ObjectList,
) *FirstEventBatchPreparerImpl {
	return &FirstEventBatchPreparerImpl{
		reader:       reader,
		objects:      objects,
		objectLists:  objectLists,
		eachListItem: meta.EachListItem,
	}
}

// SetEachListItem sets the EachListItemFunc function.
// Used for unit testing.
func (p *FirstEventBatchPreparerImpl) SetEachListItem(eachListItem EachListItemFunc) {
	p.eachListItem = eachListItem
}

func (p *FirstEventBatchPreparerImpl) Prepare(ctx context.Context) (EventBatch, error) {
	total := 0

	for _, list := range p.objectLists {
		if err := p.reader.List(ctx, list); err != nil {
			return nil, err
		}

		total += meta.LenList(list)
	}

	// If some of p.objects don't exist, they will not be added to the batch. In that case, the capacity will be greater
	// than the length, but it is OK, because len(p.objects) is small.
	batch := make([]interface{}, 0, total+len(p.objects))

	for _, obj := range p.objects {
		key := types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}

		if err := p.reader.Get(ctx, key, obj); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
		} else {
			batch = append(batch, &UpsertEvent{Resource: obj})
		}
	}

	// Note: the order of the events doesn't matter.

	for _, list := range p.objectLists {
		err := p.eachListItem(list, func(object runtime.Object) error {
			clientObj, ok := object.(client.Object)
			if !ok {
				return fmt.Errorf("cannot cast %T to client.Object", object)
			}
			batch = append(batch, &UpsertEvent{Resource: clientObj})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return batch, nil
}
