package controller

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
)

// NamespacedNameFilterFunc is a function that returns true if the resource should be processed by the reconciler.
// If the function returns false, the reconciler will log the returned string.
type NamespacedNameFilterFunc func(nsname types.NamespacedName) (shouldProcess bool, msg string)

// ReconcilerConfig is the configuration for the reconciler.
type ReconcilerConfig struct {
	// Getter gets a resource from the k8s API.
	Getter Getter
	// ObjectType is the type of the resource that the reconciler will reconcile.
	ObjectType ngftypes.ObjectType
	// EventCh is the channel where the reconciler will send events.
	EventCh chan<- interface{}
	// NamespacedNameFilter filters resources the controller will process. Can be nil.
	NamespacedNameFilter NamespacedNameFilterFunc
	// OnlyMetadata indicates that this controller for this resource is only caching metadata for the resource.
	OnlyMetadata bool
}

// Reconciler reconciles Kubernetes resources of a specific type.
// It implements the reconcile.Reconciler interface.
// A successful reconciliation of a resource has the two possible outcomes:
// (1) If the resource is deleted, the Implementation will send a DeleteEvent to the event channel.
// (2) If the resource is upserted (created or updated), the Implementation will send an UpsertEvent
// to the event channel.
type Reconciler struct {
	cfg ReconcilerConfig
}

var _ reconcile.Reconciler = &Reconciler{}

// NewReconciler creates a new reconciler.
func NewReconciler(cfg ReconcilerConfig) *Reconciler {
	return &Reconciler{
		cfg: cfg,
	}
}

func (r *Reconciler) mustCreateNewObject(objectType ngftypes.ObjectType) ngftypes.ObjectType {
	if r.cfg.OnlyMetadata {
		partialObj := &metav1.PartialObjectMetadata{}
		partialObj.SetGroupVersionKind(objectType.GetObjectKind().GroupVersionKind())

		return partialObj
	}

	// without Elem(), t will be a pointer to the type. For example, *v1.Gateway, not v1.Gateway
	t := reflect.TypeOf(objectType).Elem()

	// We could've used objectType.DeepCopyObject() here, but it's a bit slower confirmed by benchmarks.
	obj, ok := reflect.New(t).Interface().(client.Object)
	if !ok {
		panic("failed to create a new object")
	}
	return obj
}

// Reconcile implements the reconcile.Reconciler Reconcile method.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	// The controller runtime has set the logger with the group, kind, namespace and name of the resource,
	// and a few other key/value pairs. So we don't need to set them here.

	logger.Info("Reconciling the resource")

	if r.cfg.NamespacedNameFilter != nil {
		if shouldProcess, msg := r.cfg.NamespacedNameFilter(req.NamespacedName); !shouldProcess {
			logger.Info(msg)
			return reconcile.Result{}, nil
		}
	}

	obj := r.mustCreateNewObject(r.cfg.ObjectType)

	if err := r.cfg.Getter.Get(ctx, req.NamespacedName, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to get the resource")
			return reconcile.Result{}, err
		}
		// The resource does not exist (was deleted).
		obj = nil
	}

	var e interface{}
	var op string

	if obj == nil {
		e = &events.DeleteEvent{
			Type:           r.cfg.ObjectType,
			NamespacedName: req.NamespacedName,
		}
		op = "Deleted"
	} else {
		e = &events.UpsertEvent{
			Resource: obj,
		}
		op = "Upserted"
	}

	select {
	case <-ctx.Done():
		logger.Info("Did not process the resource because the context was canceled")
		return reconcile.Result{}, nil
	case r.cfg.EventCh <- e:
	}

	logger.Info(fmt.Sprintf("%s the resource", op))

	return reconcile.Result{}, nil
}
