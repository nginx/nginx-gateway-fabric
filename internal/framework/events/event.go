package events

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngftypes "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
)

// EventBatch is a batch of events to be handled at once.
type EventBatch []any

// UpsertEvent represents upserting a resource.
type UpsertEvent struct {
	// Resource is the resource that is being upserted.
	Resource client.Object
}

// DeleteEvent representing deleting a resource.
type DeleteEvent struct {
	// Type is the resource type. For example, if the event is for *v1.HTTPRoute, pass &v1.HTTPRoute{} as Type.
	Type ngftypes.ObjectType
	// NamespacedName is the namespace & name of the deleted resource.
	NamespacedName types.NamespacedName
}

// WAFBundleReconcileEvent is injected by the WAF poller manager when a bundle that was previously
// unavailable has been successfully fetched for the first time.
// It signals the event handler to re-reconcile the affected policy so the Gateway config push proceeds.
type WAFBundleReconcileEvent struct {
	// PolicyNsName is the namespace/name of the WAFPolicy whose bundle is now available.
	PolicyNsName types.NamespacedName
}
