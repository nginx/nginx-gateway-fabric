package controller

import (
	"context"
	"fmt"
	"time"

	ctlr "sigs.k8s.io/controller-runtime"
	ctlrBuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/index"
	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
)

const (
	// addIndexFieldTimeout is the timeout used for adding an Index Field to a cache.
	addIndexFieldTimeout = 2 * time.Minute
)

type config struct {
	namespacedNameFilter NamespacedNameFilterFunc
	k8sPredicate         predicate.Predicate
	fieldIndices         index.FieldIndices
	newReconciler        NewReconcilerFunc
	onlyMetadata         bool
}

// NewReconcilerFunc defines a function that creates a new Reconciler. Used for unit-testing.
type NewReconcilerFunc func(cfg ReconcilerConfig) *Reconciler

// Option defines configuration options for registering a controller.
type Option func(*config)

// WithNamespacedNameFilter enables filtering of objects by NamespacedName by the controller.
func WithNamespacedNameFilter(filter NamespacedNameFilterFunc) Option {
	return func(cfg *config) {
		cfg.namespacedNameFilter = filter
	}
}

// WithK8sPredicate enables filtering of events before they are sent to the controller.
func WithK8sPredicate(p predicate.Predicate) Option {
	return func(cfg *config) {
		cfg.k8sPredicate = p
	}
}

// WithFieldIndices adds indices to the FieldIndexer of the manager.
func WithFieldIndices(fieldIndices index.FieldIndices) Option {
	return func(cfg *config) {
		cfg.fieldIndices = fieldIndices
	}
}

// WithNewReconciler allows us to mock reconciler creation in the unit tests.
func WithNewReconciler(newReconciler NewReconcilerFunc) Option {
	return func(cfg *config) {
		cfg.newReconciler = newReconciler
	}
}

// WithOnlyMetadata tells the controller to only cache metadata, and to watch the API server in metadata-only form.
// If using this option, you must set the GroupVersionKind on the ObjectType you pass into the Register function.
// If watching a resource with OnlyMetadata, for example the v1.Pod, you must not Get and List using the v1.Pod type.
// Instead, you must use the special metav1.PartialObjectMetadata type.
func WithOnlyMetadata() Option {
	return func(cfg *config) {
		cfg.onlyMetadata = true
	}
}

func defaultConfig() config {
	return config{
		newReconciler: NewReconciler,
	}
}

// Register registers a new controller for the object type in the manager and configure it with the provided options.
// If the options include WithFieldIndices, it will add the specified indices to FieldIndexer of the manager.
// The registered controller will send events to the provided channel.
func Register(
	ctx context.Context,
	objectType ngftypes.ObjectType,
	name string,
	mgr manager.Manager,
	eventCh chan<- interface{},
	options ...Option,
) error {
	cfg := defaultConfig()

	for _, opt := range options {
		opt(&cfg)
	}

	for field, indexerFunc := range cfg.fieldIndices {
		if err := addIndex(
			ctx,
			mgr.GetFieldIndexer(),
			objectType,
			field,
			indexerFunc,
		); err != nil {
			return err
		}
	}

	var forOpts []ctlrBuilder.ForOption
	if cfg.onlyMetadata {
		if objectType.GetObjectKind().GroupVersionKind().Empty() {
			panic("the object must have its GVK set")
		}
		forOpts = append(forOpts, ctlrBuilder.OnlyMetadata)
	}

	builder := ctlr.NewControllerManagedBy(mgr).Named(name).For(objectType, forOpts...)

	if cfg.k8sPredicate != nil {
		builder = builder.WithEventFilter(cfg.k8sPredicate)
	}

	recCfg := ReconcilerConfig{
		Getter:               mgr.GetClient(),
		ObjectType:           objectType,
		EventCh:              eventCh,
		NamespacedNameFilter: cfg.namespacedNameFilter,
		OnlyMetadata:         cfg.onlyMetadata,
	}

	if err := builder.Complete(cfg.newReconciler(recCfg)); err != nil {
		return fmt.Errorf("cannot build a controller for %T: %w", objectType, err)
	}

	return nil
}

func addIndex(
	ctx context.Context,
	indexer client.FieldIndexer,
	objectType ngftypes.ObjectType,
	field string,
	indexerFunc client.IndexerFunc,
) error {
	c, cancel := context.WithTimeout(ctx, addIndexFieldTimeout)
	defer cancel()

	if err := indexer.IndexField(c, objectType, field, indexerFunc); err != nil {
		return fmt.Errorf("failed to add index for %T for field %s: %w", objectType, field, err)
	}

	return nil
}
