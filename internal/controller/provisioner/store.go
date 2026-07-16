package provisioner

import (
	"reflect"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
)

// NginxResources are all of the NGINX resources deployed in relation to a Gateway.
type NginxResources struct {
	Gateway             *graph.Gateway
	Deployment          metav1.ObjectMeta
	HPA                 metav1.ObjectMeta
	PDB                 metav1.ObjectMeta
	DaemonSet           metav1.ObjectMeta
	Service             metav1.ObjectMeta
	ServiceLBClass      *string
	ServiceAccount      metav1.ObjectMeta
	Role                metav1.ObjectMeta
	RoleBinding         metav1.ObjectMeta
	BootstrapConfigMap  metav1.ObjectMeta
	AgentConfigMap      metav1.ObjectMeta
	AgentTLSSecret      metav1.ObjectMeta
	PlusJWTSecret       metav1.ObjectMeta
	PlusClientSSLSecret metav1.ObjectMeta
	PlusCASecret        metav1.ObjectMeta
	DataplaneKeySecret  metav1.ObjectMeta
	DockerSecrets       []metav1.ObjectMeta
}

// store stores the cluster state needed by the provisioner and allows to update it from the events.
type store struct {
	// gateways is a map of all Gateway resources in the cluster. Used on startup to determine
	// which nginx resources aren't tied to any Gateways and need to be cleaned up.
	gateways map[types.NamespacedName]*gatewayv1.Gateway
	// nginxResources is a map of Gateway NamespacedNames and their associated nginx resources.
	nginxResources map[types.NamespacedName]*NginxResources

	// deletingGateways is a set of Gateways that are currently being deleted.
	deletingGateways sync.Map

	dockerSecretNames  map[string]struct{}
	agentTLSSecretName string

	// NGINX Plus secrets
	jwtSecretName       string
	caSecretName        string
	clientSSLSecretName string

	// NGINX One Dataplane key secret
	dataplaneKeySecretName string

	lock sync.RWMutex
}

func newStore(
	dockerSecretNames []string,
	agentTLSSecretName,
	jwtSecretName,
	caSecretName,
	clientSSLSecretName,
	dataplaneKeySecretName string,
) *store {
	dockerSecretNamesMap := make(map[string]struct{})
	for _, name := range dockerSecretNames {
		dockerSecretNamesMap[name] = struct{}{}
	}

	return &store{
		gateways:               make(map[types.NamespacedName]*gatewayv1.Gateway),
		nginxResources:         make(map[types.NamespacedName]*NginxResources),
		deletingGateways:       sync.Map{},
		dockerSecretNames:      dockerSecretNamesMap,
		agentTLSSecretName:     agentTLSSecretName,
		jwtSecretName:          jwtSecretName,
		caSecretName:           caSecretName,
		clientSSLSecretName:    clientSSLSecretName,
		dataplaneKeySecretName: dataplaneKeySecretName,
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
//
// The Gateway case returns whether anything we care about changed; all other resource types always
// report a change.
func (s *store) registerResourceInGatewayConfig(gatewayNSName types.NamespacedName, object any) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	switch obj := object.(type) {
	case *graph.Gateway:
		if cfg, ok := s.nginxResources[gatewayNSName]; ok {
			changed := gatewayChanged(cfg.Gateway, obj)
			cfg.Gateway = obj
			return changed
		}
		s.nginxResources[gatewayNSName] = &NginxResources{Gateway: obj}
	case *appsv1.Deployment:
		s.getOrCreateNginxResources(gatewayNSName).Deployment = obj.ObjectMeta
	case *autoscalingv2.HorizontalPodAutoscaler:
		s.getOrCreateNginxResources(gatewayNSName).HPA = obj.ObjectMeta
	case *policyv1.PodDisruptionBudget:
		s.getOrCreateNginxResources(gatewayNSName).PDB = obj.ObjectMeta
	case *appsv1.DaemonSet:
		s.getOrCreateNginxResources(gatewayNSName).DaemonSet = obj.ObjectMeta
	case *corev1.Service:
		res := s.getOrCreateNginxResources(gatewayNSName)
		res.Service = obj.ObjectMeta
		res.ServiceLBClass = obj.Spec.LoadBalancerClass
	case *corev1.ServiceAccount:
		s.getOrCreateNginxResources(gatewayNSName).ServiceAccount = obj.ObjectMeta
	case *rbacv1.Role:
		s.getOrCreateNginxResources(gatewayNSName).Role = obj.ObjectMeta
	case *rbacv1.RoleBinding:
		s.getOrCreateNginxResources(gatewayNSName).RoleBinding = obj.ObjectMeta
	case *corev1.ConfigMap:
		s.registerConfigMapInGatewayConfig(obj, gatewayNSName)
	case *corev1.Secret:
		s.registerSecretInGatewayConfig(obj, gatewayNSName)
	}

	return true
}

// getOrCreateNginxResources returns the NginxResources tracked for the given Gateway, creating and
// storing an empty entry first if none exists yet. Callers must hold s.lock.
func (s *store) getOrCreateNginxResources(gatewayNSName types.NamespacedName) *NginxResources {
	cfg, ok := s.nginxResources[gatewayNSName]
	if !ok {
		cfg = &NginxResources{}
		s.nginxResources[gatewayNSName] = cfg
	}

	return cfg
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

// hasSuffix reports whether str ends with a non-empty suffix.
func hasSuffix(str, suffix string) bool {
	return suffix != "" && strings.HasSuffix(str, suffix)
}

// assignNamedSecret sets the matching named-secret field on cfg from obj and
// reports whether obj matched one of the configured named secrets.
// Callers must hold s.lock.
func (s *store) assignNamedSecret(cfg *NginxResources, obj *corev1.Secret) bool {
	switch name := obj.GetName(); {
	case hasSuffix(name, s.agentTLSSecretName):
		cfg.AgentTLSSecret = obj.ObjectMeta
	case hasSuffix(name, s.jwtSecretName):
		cfg.PlusJWTSecret = obj.ObjectMeta
	case hasSuffix(name, s.caSecretName):
		cfg.PlusCASecret = obj.ObjectMeta
	case hasSuffix(name, s.clientSSLSecretName):
		cfg.PlusClientSSLSecret = obj.ObjectMeta
	case hasSuffix(name, s.dataplaneKeySecretName):
		cfg.DataplaneKeySecret = obj.ObjectMeta
	default:
		return false
	}

	return true
}

func (s *store) registerSecretInGatewayConfig(obj *corev1.Secret, gatewayNSName types.NamespacedName) {
	if cfg, ok := s.nginxResources[gatewayNSName]; ok {
		s.assignNamedSecret(cfg, obj)

		for secret := range s.dockerSecretNames {
			if hasSuffix(obj.GetName(), secret) {
				if len(cfg.DockerSecrets) == 0 {
					cfg.DockerSecrets = []metav1.ObjectMeta{obj.ObjectMeta}
				} else {
					cfg.DockerSecrets = append(cfg.DockerSecrets, obj.ObjectMeta)
				}
			}
		}

		return
	}

	// No config exists yet for this Gateway: create a fresh entry only when the
	// Secret matches a configured name. A matching docker secret overwrites the
	// entry, preserving the original first-match-wins behavior.
	cfg := &NginxResources{}
	if s.assignNamedSecret(cfg, obj) {
		s.nginxResources[gatewayNSName] = cfg
	}

	for secret := range s.dockerSecretNames {
		if hasSuffix(obj.GetName(), secret) {
			s.nginxResources[gatewayNSName] = &NginxResources{
				DockerSecrets: []metav1.ObjectMeta{obj.ObjectMeta},
			}
			break
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

	if !reflect.DeepEqual(original.EffectiveNginxProxy, updated.EffectiveNginxProxy) {
		return true
	}

	// Check if the effective set of listeners changed (e.g., due to ListenerSet additions/removals).
	// We compare port/protocol pairs since those determine the Service and container ports.
	return listenersChanged(original.Listeners, updated.Listeners)
}

// listenersChanged returns true if the set of listener names, ports, protocols, or hostnames
// has changed. Order is intentionally ignored: the provisioner only cares about which ports
// need to be exposed, not their order. Listener conflict resolution is handled upstream in
// the graph builder and does not affect provisioned resources.
func listenersChanged(original, updated []*graph.Listener) bool {
	if len(original) != len(updated) {
		return true
	}

	type listenerKey struct {
		Name     string
		Protocol gatewayv1.ProtocolType
		Hostname string
		Port     gatewayv1.PortNumber
	}

	buildSet := func(listeners []*graph.Listener) map[listenerKey]struct{} {
		set := make(map[listenerKey]struct{}, len(listeners))
		for _, l := range listeners {
			hostname := ""
			if l.Source.Hostname != nil {
				hostname = string(*l.Source.Hostname)
			}
			key := listenerKey{
				Name:     l.Name,
				Port:     l.Source.Port,
				Protocol: l.Source.Protocol,
				Hostname: hostname,
			}
			set[key] = struct{}{}
		}
		return set
	}

	return !reflect.DeepEqual(buildSet(original), buildSet(updated))
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

// clearServiceForGateway removes the Service entry from the NginxResources tracked for the
// given Gateway. This prevents the delete-event handler from treating a subsequent intentional
// Service deletion as an unexpected removal that needs reprovisioning.
func (s *store) clearServiceForGateway(gatewayNSName types.NamespacedName) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if cfg, ok := s.nginxResources[gatewayNSName]; ok {
		cfg.Service = metav1.ObjectMeta{}
		cfg.ServiceLBClass = nil
	}
}

func (s *store) gatewayExistsForResource(object client.Object, nsName types.NamespacedName) *graph.Gateway {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for _, resources := range s.nginxResources {
		if resources.matchesObject(object, nsName) {
			return resources.Gateway
		}
	}

	return nil
}

// matchesObject reports whether nsName identifies one of the nginx resources tracked for this
// Gateway, dispatching on the concrete type of object.
func (r *NginxResources) matchesObject(object client.Object, nsName types.NamespacedName) bool {
	switch object.(type) {
	case *appsv1.Deployment:
		return resourceMatches(r.Deployment, nsName)
	case *autoscalingv2.HorizontalPodAutoscaler:
		return resourceMatches(r.HPA, nsName)
	case *policyv1.PodDisruptionBudget:
		return resourceMatches(r.PDB, nsName)
	case *appsv1.DaemonSet:
		return resourceMatches(r.DaemonSet, nsName)
	case *corev1.Service:
		return resourceMatches(r.Service, nsName)
	case *corev1.ServiceAccount:
		return resourceMatches(r.ServiceAccount, nsName)
	case *rbacv1.Role:
		return resourceMatches(r.Role, nsName)
	case *rbacv1.RoleBinding:
		return resourceMatches(r.RoleBinding, nsName)
	case *corev1.ConfigMap:
		return resourceMatches(r.BootstrapConfigMap, nsName) || resourceMatches(r.AgentConfigMap, nsName)
	case *corev1.Secret:
		return secretResourceMatches(r, nsName)
	}

	return false
}

func secretResourceMatches(resources *NginxResources, nsName types.NamespacedName) bool {
	if resourceMatches(resources.AgentTLSSecret, nsName) {
		return true
	}

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

	if resourceMatches(resources.DataplaneKeySecret, nsName) {
		return true
	}

	return resourceMatches(resources.PlusCASecret, nsName)
}

func resourceMatches(objMeta metav1.ObjectMeta, nsName types.NamespacedName) bool {
	return objMeta.GetName() == nsName.Name && objMeta.GetNamespace() == nsName.Namespace
}

func (s *store) getResourceVersionForObject(gatewayNSName types.NamespacedName, object client.Object) string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	resources, exists := s.nginxResources[gatewayNSName]
	if !exists {
		return ""
	}

	switch obj := object.(type) {
	case *appsv1.Deployment:
		return resourceVersionIfNameMatches(resources.Deployment, obj.GetName())
	case *autoscalingv2.HorizontalPodAutoscaler:
		return resourceVersionIfNameMatches(resources.HPA, obj.GetName())
	case *policyv1.PodDisruptionBudget:
		return resourceVersionIfNameMatches(resources.PDB, obj.GetName())
	case *appsv1.DaemonSet:
		return resourceVersionIfNameMatches(resources.DaemonSet, obj.GetName())
	case *corev1.Service:
		return resourceVersionIfNameMatches(resources.Service, obj.GetName())
	case *corev1.ServiceAccount:
		return resourceVersionIfNameMatches(resources.ServiceAccount, obj.GetName())
	case *rbacv1.Role:
		return resourceVersionIfNameMatches(resources.Role, obj.GetName())
	case *rbacv1.RoleBinding:
		return resourceVersionIfNameMatches(resources.RoleBinding, obj.GetName())
	case *corev1.ConfigMap:
		return getResourceVersionForConfigMap(resources, obj)
	case *corev1.Secret:
		return getResourceVersionForSecret(resources, obj)
	}

	return ""
}

// resourceVersionIfNameMatches returns the tracked resource's ResourceVersion when its name matches
// the provided name, and an empty string otherwise.
func resourceVersionIfNameMatches(meta metav1.ObjectMeta, name string) string {
	if meta.GetName() == name {
		return meta.GetResourceVersion()
	}

	return ""
}

func getResourceVersionForConfigMap(resources *NginxResources, configmap *corev1.ConfigMap) string {
	if resources.BootstrapConfigMap.GetName() == configmap.GetName() {
		return resources.BootstrapConfigMap.GetResourceVersion()
	}
	if resources.AgentConfigMap.GetName() == configmap.GetName() {
		return resources.AgentConfigMap.GetResourceVersion()
	}

	return ""
}

func getResourceVersionForSecret(resources *NginxResources, secret *corev1.Secret) string {
	if resources.AgentTLSSecret.GetName() == secret.GetName() {
		return resources.AgentTLSSecret.GetResourceVersion()
	}
	for _, dockerSecret := range resources.DockerSecrets {
		if dockerSecret.GetName() == secret.GetName() {
			return dockerSecret.GetResourceVersion()
		}
	}
	if resources.PlusJWTSecret.GetName() == secret.GetName() {
		return resources.PlusJWTSecret.GetResourceVersion()
	}
	if resources.PlusClientSSLSecret.GetName() == secret.GetName() {
		return resources.PlusClientSSLSecret.GetResourceVersion()
	}
	if resources.PlusCASecret.GetName() == secret.GetName() {
		return resources.PlusCASecret.GetResourceVersion()
	}
	if resources.DataplaneKeySecret.GetName() == secret.GetName() {
		return resources.DataplaneKeySecret.GetResourceVersion()
	}

	return ""
}

// markGatewayDeleting marks a Gateway as being deleted.
func (s *store) markGatewayDeleting(nsName types.NamespacedName) {
	s.deletingGateways.Store(nsName, struct{}{})
}

// clearGatewayDeleting removes the deleting mark for a Gateway.
// This must be called when a Gateway with the same name is re-created
// so that reprovisionResources is not blocked for the new Gateway's managed resources.
func (s *store) clearGatewayDeleting(nsName types.NamespacedName) {
	s.deletingGateways.Delete(nsName)
}

// isGatewayDeleting checks if a Gateway is marked as being deleted.
func (s *store) isGatewayDeleting(nsName types.NamespacedName) bool {
	_, exists := s.deletingGateways.Load(nsName)
	return exists
}
