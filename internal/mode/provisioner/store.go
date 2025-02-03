package provisioner

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
)

// store stores the cluster state needed by the provisioner and allows to update it from the events.
type store struct {
	gatewayClasses map[types.NamespacedName]*v1.GatewayClass
	gateways       map[types.NamespacedName]*v1.Gateway
	crdMetadata    map[types.NamespacedName]*metav1.PartialObjectMetadata
}

func newStore() *store {
	return &store{
		gatewayClasses: make(map[types.NamespacedName]*v1.GatewayClass),
		gateways:       make(map[types.NamespacedName]*v1.Gateway),
		crdMetadata:    make(map[types.NamespacedName]*metav1.PartialObjectMetadata),
	}
}

func (s *store) update(batch events.EventBatch) {
	for _, event := range batch {
		switch e := event.(type) {
		case *events.UpsertEvent:
			switch obj := e.Resource.(type) {
			case *v1.GatewayClass:
				s.gatewayClasses[client.ObjectKeyFromObject(obj)] = obj
			case *v1.Gateway:
				s.gateways[client.ObjectKeyFromObject(obj)] = obj
			case *metav1.PartialObjectMetadata:
				s.crdMetadata[client.ObjectKeyFromObject(obj)] = obj
			default:
				panic(fmt.Errorf("unknown resource type %T", e.Resource))
			}
		case *events.DeleteEvent:
			switch e.Type.(type) {
			case *v1.GatewayClass:
				delete(s.gatewayClasses, e.NamespacedName)
			case *v1.Gateway:
				delete(s.gateways, e.NamespacedName)
			case *metav1.PartialObjectMetadata:
				delete(s.crdMetadata, e.NamespacedName)
			default:
				panic(fmt.Errorf("unknown resource type %T", e.Type))
			}
		default:
			panic(fmt.Errorf("unknown event type %T", e))
		}
	}
}
