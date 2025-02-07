package provisioner

import (
	"reflect"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
)

// NginxResources are all of the NGINX resources deployed in relation to a Gateway.
type NginxResources struct {
	Gateway            *graph.Gateway
	Deployment         *appsv1.Deployment
	Service            *corev1.Service
	ServiceAccount     *corev1.ServiceAccount
	BootstrapConfigMap *corev1.ConfigMap
	AgentConfigMap     *corev1.ConfigMap
}

// store stores the cluster state needed by the provisioner and allows to update it from the events.
type store struct {
	// gateways is a map of all Gateway resources in the cluster. Used on startup to determine
	// which nginx resources aren't tied to any Gateways and need to be cleaned up.
	gateways map[types.NamespacedName]*gatewayv1.Gateway
	// nginxResources is a map of Gateway NamespacedNames and their associated nginx resources.
	nginxResources map[types.NamespacedName]*NginxResources

	lock sync.RWMutex
}

func newStore() *store {
	return &store{
		gateways:       make(map[types.NamespacedName]*gatewayv1.Gateway),
		nginxResources: make(map[types.NamespacedName]*NginxResources),
	}
}

func (s *store) updateGateway(obj *gatewayv1.Gateway) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.gateways[client.ObjectKeyFromObject(obj)] = obj
}

func (s *store) deleteGateway(nsName types.NamespacedName) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.gateways, nsName)
}

func (s *store) getGateway(nsName types.NamespacedName) *gatewayv1.Gateway {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.gateways[nsName]
}

// registerResourceInGatewayConfig adds or updates the provided resource in the tracking map.
// If the object being updated is the Gateway, check if anything that we care about changed. This ensures that
// we don't attempt to update nginx resources when the main event handler triggers this call with an unrelated event
// (like a Route update) that shouldn't result in nginx resource changes.
func (s *store) registerResourceInGatewayConfig(gatewayNSName types.NamespacedName, object interface{}) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	switch obj := object.(type) {
	case *graph.Gateway:
		if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
			s.nginxResources[gatewayNSName] = &NginxResources{
				Gateway: obj,
			}
		} else {
			changed := gatewayChanged(cfg.Gateway, obj)
			cfg.Gateway = obj
			return changed
		}
	case *appsv1.Deployment:
		if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
			s.nginxResources[gatewayNSName] = &NginxResources{
				Deployment: obj,
			}
		} else {
			cfg.Deployment = obj
		}
	case *corev1.Service:
		if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
			s.nginxResources[gatewayNSName] = &NginxResources{
				Service: obj,
			}
		} else {
			cfg.Service = obj
		}
	case *corev1.ServiceAccount:
		if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
			s.nginxResources[gatewayNSName] = &NginxResources{
				ServiceAccount: obj,
			}
		} else {
			cfg.ServiceAccount = obj
		}
	case *corev1.ConfigMap:
		if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
			if strings.HasSuffix(obj.GetName(), nginxIncludesConfigMapNameSuffix) {
				s.nginxResources[gatewayNSName] = &NginxResources{
					BootstrapConfigMap: obj,
				}
			} else if strings.HasSuffix(obj.GetName(), nginxAgentConfigMapNameSuffix) {
				s.nginxResources[gatewayNSName] = &NginxResources{
					AgentConfigMap: obj,
				}
			}
		} else {
			if strings.HasSuffix(obj.GetName(), nginxIncludesConfigMapNameSuffix) {
				cfg.BootstrapConfigMap = obj
			} else if strings.HasSuffix(obj.GetName(), nginxAgentConfigMapNameSuffix) {
				cfg.AgentConfigMap = obj
			}
		}
	}

	return true
}

func gatewayChanged(original, updated *graph.Gateway) bool {
	if original == nil {
		return true
	}

	if original.Valid != updated.Valid {
		return true
	}

	if !reflect.DeepEqual(original.Source, updated.Source) {
		return true
	}

	return !reflect.DeepEqual(original.EffectiveNginxProxy, updated.EffectiveNginxProxy)
}

func (s *store) getNginxResourcesForGateway(nsName types.NamespacedName) *NginxResources {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.nginxResources[nsName]
}

func (s *store) deleteResourcesForGateway(nsName types.NamespacedName) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.nginxResources, nsName)
}

//nolint:gocyclo // will refactor at some point
func (s *store) gatewayExistsForResource(object client.Object, nsName types.NamespacedName) *graph.Gateway {
	s.lock.RLock()
	defer s.lock.RUnlock()

	resourceMatches := func(obj client.Object) bool {
		return obj.GetName() == nsName.Name && obj.GetNamespace() == nsName.Namespace
	}

	for _, resources := range s.nginxResources {
		switch object.(type) {
		case *appsv1.Deployment:
			if resources.Deployment != nil && resourceMatches(resources.Deployment) {
				return resources.Gateway
			}
		case *corev1.Service:
			if resources.Service != nil && resourceMatches(resources.Service) {
				return resources.Gateway
			}
		case *corev1.ServiceAccount:
			if resources.ServiceAccount != nil && resourceMatches(resources.ServiceAccount) {
				return resources.Gateway
			}
		case *corev1.ConfigMap:
			if resources.BootstrapConfigMap != nil && resourceMatches(resources.BootstrapConfigMap) {
				return resources.Gateway
			}
			if resources.AgentConfigMap != nil && resourceMatches(resources.AgentConfigMap) {
				return resources.Gateway
			}
		}
	}

	return nil
}
