package provisioner

import (
	"reflect"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
)

// NginxResources are all of the NGINX resources deployed in relation to a Gateway.
type NginxResources struct {
	Gateway             *graph.Gateway
	Deployment          metav1.ObjectMeta
	Service             metav1.ObjectMeta
	ServiceAccount      metav1.ObjectMeta
	BootstrapConfigMap  metav1.ObjectMeta
	AgentConfigMap      metav1.ObjectMeta
	PlusJWTSecret       metav1.ObjectMeta
	PlusClientSSLSecret metav1.ObjectMeta
	PlusCASecret        metav1.ObjectMeta
	DockerSecrets       []metav1.ObjectMeta
}

// store stores the cluster state needed by the provisioner and allows to update it from the events.
type store struct {
	// gateways is a map of all Gateway resources in the cluster. Used on startup to determine
	// which nginx resources aren't tied to any Gateways and need to be cleaned up.
	gateways map[types.NamespacedName]*gatewayv1.Gateway
	// nginxResources is a map of Gateway NamespacedNames and their associated nginx resources.
	nginxResources map[types.NamespacedName]*NginxResources

	dockerSecretNames map[string]struct{}
	// NGINX Plus secrets
	jwtSecretName       string
	caSecretName        string
	clientSSLSecretName string

	lock sync.RWMutex
}

func newStore(
	dockerSecretNames []string,
	jwtSecretName,
	caSecretName,
	clientSSLSecretName string,
) *store {
	dockerSecretNamesMap := make(map[string]struct{})
	for _, name := range dockerSecretNames {
		dockerSecretNamesMap[name] = struct{}{}
	}

	return &store{
		gateways:            make(map[types.NamespacedName]*gatewayv1.Gateway),
		nginxResources:      make(map[types.NamespacedName]*NginxResources),
		dockerSecretNames:   dockerSecretNamesMap,
		jwtSecretName:       jwtSecretName,
		caSecretName:        caSecretName,
		clientSSLSecretName: clientSSLSecretName,
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

func (s *store) getGateways() map[types.NamespacedName]*gatewayv1.Gateway {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.gateways
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
				Deployment: obj.ObjectMeta,
			}
		} else {
			cfg.Deployment = obj.ObjectMeta
		}
	case *corev1.Service:
		if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
			s.nginxResources[gatewayNSName] = &NginxResources{
				Service: obj.ObjectMeta,
			}
		} else {
			cfg.Service = obj.ObjectMeta
		}
	case *corev1.ServiceAccount:
		if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
			s.nginxResources[gatewayNSName] = &NginxResources{
				ServiceAccount: obj.ObjectMeta,
			}
		} else {
			cfg.ServiceAccount = obj.ObjectMeta
		}
	case *corev1.ConfigMap:
		s.registerConfigMapInGatewayConfig(obj, gatewayNSName)
	case *corev1.Secret:
		s.registerSecretInGatewayConfig(obj, gatewayNSName)
	}

	return true
}

func (s *store) registerConfigMapInGatewayConfig(obj *corev1.ConfigMap, gatewayNSName types.NamespacedName) {
	if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
		if strings.HasSuffix(obj.GetName(), nginxIncludesConfigMapNameSuffix) {
			s.nginxResources[gatewayNSName] = &NginxResources{
				BootstrapConfigMap: obj.ObjectMeta,
			}
		} else if strings.HasSuffix(obj.GetName(), nginxAgentConfigMapNameSuffix) {
			s.nginxResources[gatewayNSName] = &NginxResources{
				AgentConfigMap: obj.ObjectMeta,
			}
		}
	} else {
		if strings.HasSuffix(obj.GetName(), nginxIncludesConfigMapNameSuffix) {
			cfg.BootstrapConfigMap = obj.ObjectMeta
		} else if strings.HasSuffix(obj.GetName(), nginxAgentConfigMapNameSuffix) {
			cfg.AgentConfigMap = obj.ObjectMeta
		}
	}
}

func (s *store) registerSecretInGatewayConfig(obj *corev1.Secret, gatewayNSName types.NamespacedName) {
	hasSuffix := func(str, suffix string) bool {
		return suffix != "" && strings.HasSuffix(str, suffix)
	}

	if cfg, ok := s.nginxResources[gatewayNSName]; !ok {
		switch {
		case hasSuffix(obj.GetName(), s.jwtSecretName):
			s.nginxResources[gatewayNSName] = &NginxResources{
				PlusJWTSecret: obj.ObjectMeta,
			}
		case hasSuffix(obj.GetName(), s.caSecretName):
			s.nginxResources[gatewayNSName] = &NginxResources{
				PlusCASecret: obj.ObjectMeta,
			}
		case hasSuffix(obj.GetName(), s.clientSSLSecretName):
			s.nginxResources[gatewayNSName] = &NginxResources{
				PlusClientSSLSecret: obj.ObjectMeta,
			}
		}

		for secret := range s.dockerSecretNames {
			if hasSuffix(obj.GetName(), secret) {
				s.nginxResources[gatewayNSName] = &NginxResources{
					DockerSecrets: []metav1.ObjectMeta{obj.ObjectMeta},
				}
				break
			}
		}
	} else {
		switch {
		case hasSuffix(obj.GetName(), s.jwtSecretName):
			cfg.PlusJWTSecret = obj.ObjectMeta
		case hasSuffix(obj.GetName(), s.caSecretName):
			cfg.PlusCASecret = obj.ObjectMeta
		case hasSuffix(obj.GetName(), s.clientSSLSecretName):
			cfg.PlusClientSSLSecret = obj.ObjectMeta
		}

		for secret := range s.dockerSecretNames {
			if hasSuffix(obj.GetName(), secret) {
				if len(cfg.DockerSecrets) == 0 {
					cfg.DockerSecrets = []metav1.ObjectMeta{obj.ObjectMeta}
				} else {
					cfg.DockerSecrets = append(cfg.DockerSecrets, obj.ObjectMeta)
				}
			}
		}
	}
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

func (s *store) gatewayExistsForResource(object client.Object, nsName types.NamespacedName) *graph.Gateway {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for _, resources := range s.nginxResources {
		switch object.(type) {
		case *appsv1.Deployment:
			if resourceMatches(resources.Deployment, nsName) {
				return resources.Gateway
			}
		case *corev1.Service:
			if resourceMatches(resources.Service, nsName) {
				return resources.Gateway
			}
		case *corev1.ServiceAccount:
			if resourceMatches(resources.ServiceAccount, nsName) {
				return resources.Gateway
			}
		case *corev1.ConfigMap:
			if resourceMatches(resources.BootstrapConfigMap, nsName) {
				return resources.Gateway
			}
			if resourceMatches(resources.AgentConfigMap, nsName) {
				return resources.Gateway
			}
		case *corev1.Secret:
			if secretResourceMatches(resources, nsName) {
				return resources.Gateway
			}
		}
	}

	return nil
}

func secretResourceMatches(resources *NginxResources, nsName types.NamespacedName) bool {
	for _, secret := range resources.DockerSecrets {
		if resourceMatches(secret, nsName) {
			return true
		}
	}

	if resourceMatches(resources.PlusJWTSecret, nsName) {
		return true
	}

	if resourceMatches(resources.PlusClientSSLSecret, nsName) {
		return true
	}

	return resourceMatches(resources.PlusCASecret, nsName)
}

func resourceMatches(objMeta metav1.ObjectMeta, nsName types.NamespacedName) bool {
	return objMeta.GetName() == nsName.Name && objMeta.GetNamespace() == nsName.Namespace
}
