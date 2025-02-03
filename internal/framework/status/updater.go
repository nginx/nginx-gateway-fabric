package status

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
)

// UpdateRequest is a request to update the status of a resource.
type UpdateRequest struct {
	ResourceType ngftypes.ObjectType
	Setter       Setter
	NsName       types.NamespacedName
}

// Setter is a function that sets the status of the passed resource.
// It returns true if the status was set, false otherwise.
// The status is not set when the status is already up-to-date.
type Setter func(client.Object) (wasSet bool)

// Updater updates the status of resources.
//
// It has the following limitations:
//
// (1) It is synchronous, which means the status reporter can slow down the event loop.
// Consider the following cases:
// (a) Sometimes the Gateway will need to update statuses of all resources it handles, which could be ~1000. Making 1000
// status API calls sequentially will take time.
// (b) k8s API can become slow or even timeout. This will increase every update status API call.
// Making Updater asynchronous will prevent it from adding variable delays to the event loop.
// FIXME(pleshakov): https://github.com/nginx/nginx-gateway-fabric/issues/1014
//
// (2) It doesn't clear the statuses of a resources that are no longer handled by the Gateway. For example, if
// an HTTPRoute resource no longer has the parentRef to the Gateway resources, the Gateway must update the status
// of the resource to remove the status about the removed parentRef.
// FIXME(pleshakov): https://github.com/nginx/nginx-gateway-fabric/issues/1015
//
// (3) If another controllers changes the status of the Gateway/HTTPRoute resource so that the information set by our
// Gateway is removed, our Gateway will not restore the status until the EventLoop invokes the StatusUpdater as a
// result of processing some other new change to a resource(s).
// FIXME(pleshakov): https://github.com/nginx/nginx-gateway-fabric/issues/1813
type Updater struct {
	client client.Client
	logger logr.Logger
}

var ErrFailedAssert = errors.New("type assertion failed")

// NewUpdater creates a new Updater.
func NewUpdater(c client.Client, logger logr.Logger) *Updater {
	return &Updater{
		client: c,
		logger: logger,
	}
}

// Update updates the status of the resources from the requests.
func (u *Updater) Update(ctx context.Context, reqs ...UpdateRequest) {
	for _, r := range reqs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		u.logger.V(1).Info(
			"Updating status for resource",
			"namespace", r.NsName.Namespace,
			"name", r.NsName.Name,
			"kind", r.ResourceType.GetObjectKind().GroupVersionKind().Kind,
		)

		u.writeStatuses(ctx, r.NsName, r.ResourceType, r.Setter)
	}
}

func (u *Updater) writeStatuses(
	ctx context.Context,
	nsname types.NamespacedName,
	resourceType ngftypes.ObjectType,
	statusSetter Setter,
) {
	copiedObject := resourceType.DeepCopyObject()
	obj, ok := copiedObject.(client.Object)
	if !ok {
		panic(fmt.Errorf("object is not a client.Object: %w", ErrFailedAssert))
	}

	err := wait.ExponentialBackoffWithContext(
		ctx,
		wait.Backoff{
			Duration: time.Millisecond * 200,
			Factor:   2,
			Jitter:   0.5,
			Steps:    4,
			Cap:      time.Millisecond * 3000,
		},
		// Function returns true if the condition is satisfied, or an error if the loop should be aborted.
		NewRetryUpdateFunc(u.client, u.client.Status(), nsname, obj, u.logger, statusSetter),
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		u.logger.Error(
			err,
			"Failed to update status",
			"namespace", nsname.Namespace,
			"name", nsname.Name,
			"kind", resourceType.GetObjectKind().GroupVersionKind().Kind)
	}
}

// NewRetryUpdateFunc returns a function which will be used in wait.ExponentialBackoffWithContext.
// The function will attempt to Update a kubernetes resource and will be retried in
// wait.ExponentialBackoffWithContext if an error occurs. Exported for testing purposes.
//
// wait.ExponentialBackoffWithContext will retry if this function returns nil as its error,
// which is what we want if we encounter an error from the functions we call. However,
// the linter will complain if we return nil if an error was found.
//
// Note: this function is public because fake dependencies require us to test this function from the test package
// to avoid import cycles.
func NewRetryUpdateFunc(
	getter controller.Getter,
	updater K8sUpdater,
	nsname types.NamespacedName,
	obj client.Object,
	logger logr.Logger,
	statusSetter Setter,
) func(ctx context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		// The function handles errors by reporting them in the logs.
		// We need to get the latest version of the resource.
		// Otherwise, the Update status API call can fail.
		// Note: the default client uses a cache for reads, so we're not making an unnecessary API call here.
		// the default is configurable in the Manager options.
		if err := getter.Get(ctx, nsname, obj); err != nil {
			// apierrors.IsNotFound(err) can happen when the resource is deleted,
			// so no need to retry or return an error.
			if apierrors.IsNotFound(err) {
				return true, nil
			}

			logger.V(1).Info(
				"Encountered error when getting resource to update status",
				"error", err,
				"namespace", nsname.Namespace,
				"name", nsname.Name,
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			)

			return false, nil
		}

		if !statusSetter(obj) {
			logger.V(1).Info(
				"Skipping status update because there's no change",
				"namespace", nsname.Namespace,
				"name", nsname.Name,
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			)

			return true, nil
		}

		if err := updater.Update(ctx, obj); err != nil {
			logger.V(1).Info(
				"Encountered error updating status",
				"error", err,
				"namespace", nsname.Namespace,
				"name", nsname.Name,
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			)

			return false, nil
		}

		return true, nil
	}
}
