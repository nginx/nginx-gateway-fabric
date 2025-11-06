package provisioner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strconv"
	"time"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	nginxTypes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/configmaps"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const (
	defaultNginxErrorLogLevel        = "info"
	nginxIncludesConfigMapNameSuffix = "includes-bootstrap"
	nginxAgentConfigMapNameSuffix    = "agent-config"

	defaultServiceType   = corev1.ServiceTypeLoadBalancer
	defaultServicePolicy = corev1.ServiceExternalTrafficPolicyLocal

	defaultNginxImagePath      = "ghcr.io/nginx/nginx-gateway-fabric/nginx"
	defaultNginxPlusImagePath  = "private-registry.nginx.com/nginx-gateway-fabric/nginx-plus"
	defaultImagePullPolicy     = corev1.PullIfNotPresent
	defaultInitialDelaySeconds = int32(3)

	// WAF container defaults.
	defaultWAFEnforcerImagePath  = "private-registry.nginx.com/nap/waf-enforcer"
	defaultWAFConfigMgrImagePath = "private-registry.nginx.com/nap/waf-config-mgr"
	// FIXME(ciarams87): Figure out best way to handle WAF image tags.
	defaultWAFImageTag = "5.7.0"

	// WAF shared volume names.
	appProtectBundlesVolumeName  = "app-protect-bundles"
	appProtectConfigVolumeName   = "app-protect-config"
	appProtectBdConfigVolumeName = "app-protect-bd-config"
)

var emptyDirVolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}

// setOwnerReference is a helper to set owner reference on an object.
func (p *NginxProvisioner) setOwnerReference(obj client.Object, gateway *gatewayv1.Gateway) error {
	return controllerutil.SetControllerReference(gateway, obj, p.k8sClient.Scheme())
}

// resourceNames holds all the generated resource names.
type resourceNames struct {
	// new name -> original name
	dockerSecrets          map[string]string
	nginxIncludesConfigMap string
	nginxAgentConfigMap    string
	agentTLS               string
	jwt                    string
	ca                     string
	clientSSL              string
	dataplaneKey           string
}

// buildNginxResourceObjects builds all the NGINX resource objects for a given Gateway and EffectiveNginxProxy.
func (p *NginxProvisioner) buildNginxResourceObjects(
	resourceName string,
	gateway *gatewayv1.Gateway,
	nProxyCfg *graph.EffectiveNginxProxy,
) ([]client.Object, error) {
	var errs []error

	// Need to ensure nginx resource objects are generated deterministically. Specifically when generating
	// an object's field by ranging over a map, since ranging over a map is done in random order, we need to
	// do some processing to ensure the generated results are the same each time.

	// build resource names
	resourceNames := p.buildResourceNames(resourceName)

	// build labels, annotations, and objectMeta
	selectorLabels, labels, annotations := p.buildLabelsAndAnnotations(resourceName, gateway)
	objectMeta := metav1.ObjectMeta{
		Name:        resourceName,
		Namespace:   gateway.GetNamespace(),
		Labels:      labels,
		Annotations: annotations,
	}

	// build secrets
	secretsList, secretsErr := p.buildNginxSecrets(
		objectMeta,
		resourceNames,
		gateway,
	)
	if secretsErr != nil {
		errs = append(errs, secretsErr)
	}

	configmapsList, configMapErrs := p.buildNginxConfigMaps(
		objectMeta,
		nProxyCfg,
		resourceNames,
		gateway,
	)
	if configMapErrs != nil {
		errs = append(errs, configMapErrs...)
	}

	// build service account
	serviceAccount, err := p.buildServiceAccount(objectMeta, gateway)
	if err != nil {
		errs = append(errs, err)
	}

	var openshiftObjs []client.Object
	if p.isOpenshift {
		var openShiftErrs []error
		openshiftObjs, openShiftErrs = p.buildOpenshiftObjects(objectMeta, gateway)
		if openShiftErrs != nil {
			errs = append(errs, openShiftErrs...)
		}
	}

	// build ports from gateway listeners
	ports := p.buildPortsFromListeners(gateway.Spec.Listeners)

	// Add healthcheck port to service if expose is enabled
	var healthcheckPort int32
	if isNginxReadinessProbeExposed(nProxyCfg) {
		healthcheckPort = dataplane.GetNginxReadinessProbePort(nProxyCfg)
		ports[healthcheckPort] = corev1.ProtocolTCP
	}

	// build service
	service, err := buildNginxService(
		cloneObjectMeta(objectMeta),
		nProxyCfg,
		ports,
		healthcheckPort,
		selectorLabels,
		gateway.Spec.Addresses,
	)
	if err != nil {
		errs = append(errs, err)
	}
	if err := p.setOwnerReference(service, gateway); err != nil {
		errs = append(errs, fmt.Errorf("failed to set owner reference on Service %s: %w", service.GetName(), err))
	}

	// build deployment/daemonset
	deployment, err := p.buildNginxDeployment(
		cloneObjectMeta(objectMeta),
		nProxyCfg,
		ports,
		selectorLabels,
		resourceNames,
	)
	if err != nil {
		errs = append(errs, err)
	}
	if err := p.setOwnerReference(deployment, gateway); err != nil {
		errs = append(errs, fmt.Errorf(
			"failed to set owner reference on %s %s: %w",
			deployment.GetObjectKind().GroupVersionKind().Kind,
			deployment.GetName(),
			err,
		))
	}

	// order to install resources:
	// secrets
	// configmaps
	// serviceaccount
	// role/binding (if openshift)
	// service
	// deployment/daemonset
	// hpa

	objects := make([]client.Object, 0, len(configmapsList)+len(secretsList)+len(openshiftObjs)+3)
	objects = append(objects, secretsList...)
	objects = append(objects, configmapsList...)
	objects = append(objects, serviceAccount)
	if p.isOpenshift {
		objects = append(objects, openshiftObjs...)
	}

	objects = append(objects, service, deployment)

	if hpa := p.buildHPA(objectMeta, nProxyCfg); hpa != nil {
		if err := p.setOwnerReference(hpa, gateway); err != nil {
			errs = append(errs, fmt.Errorf("failed to set owner reference on HPA %s: %w", hpa.GetName(), err))
		}
		objects = append(objects, hpa)
	}

	return objects, errors.Join(errs...)
}

// buildResourceNames builds all the resource names for a given gateway resource name.
func (p *NginxProvisioner) buildResourceNames(resourceName string) resourceNames {
	names := resourceNames{
		nginxIncludesConfigMap: controller.CreateNginxResourceName(resourceName, nginxIncludesConfigMapNameSuffix),
		nginxAgentConfigMap:    controller.CreateNginxResourceName(resourceName, nginxAgentConfigMapNameSuffix),
		agentTLS:               controller.CreateNginxResourceName(resourceName, p.cfg.AgentTLSSecretName),
		dockerSecrets:          make(map[string]string),
	}

	if p.cfg.Plus {
		names.jwt = controller.CreateNginxResourceName(resourceName, p.cfg.PlusUsageConfig.SecretName)
		if p.cfg.PlusUsageConfig.CASecretName != "" {
			names.ca = controller.CreateNginxResourceName(resourceName, p.cfg.PlusUsageConfig.CASecretName)
		}
		if p.cfg.PlusUsageConfig.ClientSSLSecretName != "" {
			names.clientSSL = controller.CreateNginxResourceName(resourceName, p.cfg.PlusUsageConfig.ClientSSLSecretName)
		}
	}

	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		names.dataplaneKey = controller.CreateNginxResourceName(
			resourceName,
			p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName,
		)
	}

	for _, name := range p.cfg.NginxDockerSecretNames {
		newName := controller.CreateNginxResourceName(resourceName, name)
		names.dockerSecrets[newName] = name
	}

	return names
}

// buildLabelsAndAnnotations builds labels and annotations for NGINX resources.
func (p *NginxProvisioner) buildLabelsAndAnnotations(
	resourceName string,
	gateway *gatewayv1.Gateway,
) (selectorLabels, labels, annotations map[string]string) {
	selectorLabels = make(map[string]string)
	maps.Copy(selectorLabels, p.baseLabelSelector.MatchLabels)
	selectorLabels[controller.AppNameLabel] = resourceName

	annotations = make(map[string]string)
	if len(gateway.GetName()) > controller.MaxServiceNameLen {
		annotations[controller.GatewayLabel] = gateway.GetName()
	} else {
		selectorLabels[controller.GatewayLabel] = gateway.GetName()
	}

	labels = make(map[string]string)
	maps.Copy(labels, selectorLabels)

	if gateway.Spec.Infrastructure != nil {
		for key, value := range gateway.Spec.Infrastructure.Labels {
			labels[string(key)] = string(value)
		}
		for key, value := range gateway.Spec.Infrastructure.Annotations {
			annotations[string(key)] = string(value)
		}
	}

	return selectorLabels, labels, annotations
}

// buildServiceAccount builds the ServiceAccount for NGINX deployment/daemonset.
func (p *NginxProvisioner) buildServiceAccount(
	objectMeta metav1.ObjectMeta,
	gateway *gatewayv1.Gateway,
) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta:                   objectMeta,
		AutomountServiceAccountToken: helpers.GetPointer(false),
	}
	if err := p.setOwnerReference(serviceAccount, gateway); err != nil {
		return serviceAccount, fmt.Errorf(
			"failed to set owner reference on ServiceAccount %s: %w", serviceAccount.GetName(),
			err,
		)
	}
	return serviceAccount, nil
}

// buildPortsFromListeners builds a map of ports to protocols from the Gateway listeners.
func (p *NginxProvisioner) buildPortsFromListeners(listeners []gatewayv1.Listener) map[int32]corev1.Protocol {
	ports := make(map[int32]corev1.Protocol, len(listeners))
	for _, listener := range listeners {
		switch listener.Protocol {
		case gatewayv1.UDPProtocolType:
			ports[listener.Port] = corev1.ProtocolUDP
		default:
			ports[listener.Port] = corev1.ProtocolTCP
		}
	}
	return ports
}

// cloneObjectMeta clones the given ObjectMeta.
func cloneObjectMeta(meta metav1.ObjectMeta) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        meta.Name,
		Namespace:   meta.Namespace,
		Labels:      maps.Clone(meta.Labels),
		Annotations: maps.Clone(meta.Annotations),
	}
}

func isAutoscalingEnabled(dep *ngfAPIv1alpha2.DeploymentSpec) bool {
	return dep != nil && dep.Autoscaling != nil && dep.Autoscaling.Enable
}

func (p *NginxProvisioner) buildHPA(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
) client.Object {
	if nProxyCfg == nil || nProxyCfg.Kubernetes == nil || !isAutoscalingEnabled(nProxyCfg.Kubernetes.Deployment) {
		return nil
	}

	return buildNginxDeploymentHPA(objectMeta, nProxyCfg.Kubernetes.Deployment.Autoscaling)
}

func (p *NginxProvisioner) buildNginxSecrets(
	objectMeta metav1.ObjectMeta,
	names resourceNames,
	gateway *gatewayv1.Gateway,
) ([]client.Object, error) {
	var secretsList []client.Object
	var dockerSecrets []client.Object
	var errs []error

	addSecret := func(
		sourceSecretName string,
		targetSecretName string,
		secretType corev1.SecretType,
	) *corev1.Secret {
		if targetSecretName == "" {
			return nil
		}
		newSecret, err := p.getAndUpdateSecret(
			sourceSecretName,
			metav1.ObjectMeta{
				Name:        targetSecretName,
				Namespace:   objectMeta.Namespace,
				Labels:      objectMeta.Labels,
				Annotations: objectMeta.Annotations,
			},
			secretType,
		)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if err := p.setOwnerReference(newSecret, gateway); err != nil {
			errs = append(errs, fmt.Errorf("failed to set owner reference on Secret %s: %w", newSecret.GetName(), err))
			return nil
		}
		return newSecret
	}

	// agent TLS Secret
	if secret := addSecret(p.cfg.AgentTLSSecretName, names.agentTLS, corev1.SecretTypeTLS); secret != nil {
		secretsList = append(secretsList, secret)
	}

	// docker Secrets
	for targetSecretName, sourceSecretName := range names.dockerSecrets {
		if secret := addSecret(sourceSecretName, targetSecretName, corev1.SecretTypeDockerConfigJson); secret != nil {
			dockerSecrets = append(dockerSecrets, secret)
		}
	}

	// need to sort secrets so everytime buildNginxSecrets is called it will generate the exact same
	// array of secrets. This is needed to satisfy deterministic results of the method.
	sort.Slice(dockerSecrets, func(i, j int) bool {
		return dockerSecrets[i].GetName() < dockerSecrets[j].GetName()
	})
	secretsList = append(secretsList, dockerSecrets...)

	// plus Secrets (in order: jwt, ca, clientSSL)
	if p.cfg.PlusUsageConfig != nil {
		if secret := addSecret(p.cfg.PlusUsageConfig.SecretName, names.jwt, corev1.SecretTypeOpaque); secret != nil {
			secretsList = append(secretsList, secret)
		}
		if secret := addSecret(p.cfg.PlusUsageConfig.CASecretName, names.ca, corev1.SecretTypeOpaque); secret != nil {
			secretsList = append(secretsList, secret)
		}
		if secret := addSecret(
			p.cfg.PlusUsageConfig.ClientSSLSecretName,
			names.clientSSL,
			corev1.SecretTypeTLS,
		); secret != nil {
			secretsList = append(secretsList, secret)
		}
	}

	// Dataplane Key Secret
	if secret := addSecret(
		p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName,
		names.dataplaneKey,
		corev1.SecretTypeOpaque,
	); secret != nil {
		secretsList = append(secretsList, secret)
	}

	return secretsList, errors.Join(errs...)
}

func (p *NginxProvisioner) getAndUpdateSecret(
	name string,
	newObjectMeta metav1.ObjectMeta,
	secretType corev1.SecretType,
) (*corev1.Secret, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := types.NamespacedName{Namespace: p.cfg.GatewayPodConfig.Namespace, Name: name}
	secret := &corev1.Secret{}
	if err := p.k8sClient.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("error getting secret: %w", err)
	}

	newSecret := &corev1.Secret{
		ObjectMeta: newObjectMeta,
		Data:       secret.Data,
		Type:       secretType,
	}

	return newSecret, nil
}

func (p *NginxProvisioner) buildNginxConfigMaps(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	names resourceNames,
	gateway *gatewayv1.Gateway,
) ([]client.Object, []error) {
	var errs []error

	bootstrapCM := p.buildBootstrapConfigMap(
		objectMeta,
		nProxyCfg,
		names.nginxIncludesConfigMap,
		names.ca != "",
		names.clientSSL != "",
	)
	if err := p.setOwnerReference(bootstrapCM, gateway); err != nil {
		errs = append(errs, fmt.Errorf("failed to set owner reference on ConfigMap %s: %w", bootstrapCM.GetName(), err))
	}

	agentCM := p.buildAgentConfigMap(objectMeta, nProxyCfg, names.nginxAgentConfigMap)
	if err := p.setOwnerReference(agentCM, gateway); err != nil {
		errs = append(errs, fmt.Errorf("failed to set owner reference on ConfigMap %s: %w", agentCM.GetName(), err))
	}

	return []client.Object{bootstrapCM, agentCM}, errs
}

func (p *NginxProvisioner) buildBootstrapConfigMap(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	name string,
	caSecret bool,
	clientSSLSecret bool,
) *corev1.ConfigMap {
	logLevel := defaultNginxErrorLogLevel
	if nProxyCfg != nil && nProxyCfg.Logging != nil && nProxyCfg.Logging.ErrorLevel != nil {
		logLevel = string(*nProxyCfg.Logging.ErrorLevel)
	}

	workerConnections := dataplane.DefaultWorkerConnections
	if nProxyCfg != nil && nProxyCfg.WorkerConnections != nil {
		workerConnections = *nProxyCfg.WorkerConnections
	}

	mainFields := map[string]any{
		"ErrorLevel":        logLevel,
		"WorkerConnections": workerConnections,
	}

	eventsFields := map[string]any{
		"WorkerConnections": workerConnections,
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
		},
		Data: map[string]string{
			configmaps.MainConfKey:   string(helpers.MustExecuteTemplate(mainTemplate, mainFields)),
			configmaps.EventsConfKey: string(helpers.MustExecuteTemplate(eventsTemplate, eventsFields)),
		},
	}

	if p.cfg.Plus {
		mgmtFields := map[string]any{
			"UsageEndpoint":        p.cfg.PlusUsageConfig.Endpoint,
			"SkipVerify":           p.cfg.PlusUsageConfig.SkipVerify,
			"UsageCASecret":        caSecret,
			"UsageClientSSLSecret": clientSSLSecret,
		}
		cm.Data[configmaps.MgmtConfKey] = string(helpers.MustExecuteTemplate(mgmtTemplate, mgmtFields))
	}

	return cm
}

func (p *NginxProvisioner) buildAgentConfigMap(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	name string,
) *corev1.ConfigMap {
	metricsPort := config.DefaultNginxMetricsPort
	port, enableMetrics := graph.MetricsEnabledForNginxProxy(nProxyCfg)
	if port != nil {
		metricsPort = *port
	}

	depType := nginxTypes.DeploymentType
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil && nProxyCfg.Kubernetes.DaemonSet != nil {
		depType = nginxTypes.DaemonSetType
	}

	agentLabels := make(map[string]string)
	maps.Copy(agentLabels, p.cfg.AgentLabels)
	agentLabels[nginxTypes.AgentOwnerNameLabel] = fmt.Sprintf("%s_%s", objectMeta.Namespace, objectMeta.Name)
	agentLabels[nginxTypes.AgentOwnerTypeLabel] = depType

	agentFields := map[string]any{
		"Plus":          p.cfg.Plus,
		"ServiceName":   p.cfg.GatewayPodConfig.ServiceName,
		"Namespace":     p.cfg.GatewayPodConfig.Namespace,
		"EnableMetrics": enableMetrics,
		"MetricsPort":   metricsPort,
		"AgentLabels":   agentLabels,
	}

	if nProxyCfg != nil && nProxyCfg.Logging != nil && nProxyCfg.Logging.AgentLevel != nil {
		agentFields["LogLevel"] = *nProxyCfg.Logging.AgentLevel
	}

	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		agentFields["NginxOneReporting"] = true
		agentFields["EndpointHost"] = p.cfg.NginxOneConsoleTelemetryConfig.EndpointHost
		agentFields["EndpointPort"] = strconv.Itoa(p.cfg.NginxOneConsoleTelemetryConfig.EndpointPort)
		agentFields["EndpointTLSSkipVerify"] = p.cfg.NginxOneConsoleTelemetryConfig.EndpointTLSSkipVerify
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
		},
		Data: map[string]string{
			configmaps.AgentConfKey: string(helpers.MustExecuteTemplate(agentTemplate, agentFields)),
		},
	}
}

func (p *NginxProvisioner) buildOpenshiftObjects(
	objectMeta metav1.ObjectMeta,
	gateway *gatewayv1.Gateway,
) ([]client.Object, []error) {
	var errs []error
	role := &rbacv1.Role{
		ObjectMeta: objectMeta,
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"security.openshift.io"},
				ResourceNames: []string{p.cfg.NGINXSCCName},
				Resources:     []string{"securitycontextconstraints"},
				Verbs:         []string{"use"},
			},
		},
	}

	if err := p.setOwnerReference(role, gateway); err != nil {
		errs = append(errs, fmt.Errorf("failed to set owner reference on Role %s: %w", role.GetName(), err))
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: objectMeta,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     objectMeta.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      objectMeta.Name,
				Namespace: objectMeta.Namespace,
			},
		},
	}

	if err := p.setOwnerReference(roleBinding, gateway); err != nil {
		errs = append(errs, fmt.Errorf("failed to set owner reference on RoleBinding %s: %w", roleBinding.GetName(), err))
	}

	return []client.Object{role, roleBinding}, errs
}

func buildNginxService(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ports map[int32]corev1.Protocol,
	healthcheckPort int32,
	selectorLabels map[string]string,
	addresses []gatewayv1.GatewaySpecAddress,
) (*corev1.Service, error) {
	var serviceCfg ngfAPIv1alpha2.ServiceSpec
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil && nProxyCfg.Kubernetes.Service != nil {
		serviceCfg = *nProxyCfg.Kubernetes.Service
	}

	serviceType := defaultServiceType
	if serviceCfg.ServiceType != nil {
		serviceType = corev1.ServiceType(*serviceCfg.ServiceType)
	}

	var servicePolicy corev1.ServiceExternalTrafficPolicy
	if serviceType != corev1.ServiceTypeClusterIP {
		servicePolicy = defaultServicePolicy
		if serviceCfg.ExternalTrafficPolicy != nil {
			servicePolicy = corev1.ServiceExternalTrafficPolicy(*serviceCfg.ExternalTrafficPolicy)
		}
	}

	servicePorts := buildServicePorts(ports, healthcheckPort, serviceType, serviceCfg.NodePorts)

	svc := &corev1.Service{
		ObjectMeta: objectMeta,
		Spec: corev1.ServiceSpec{
			Type:                  serviceType,
			Ports:                 servicePorts,
			ExternalTrafficPolicy: servicePolicy,
			Selector:              selectorLabels,
			IPFamilyPolicy:        helpers.GetPointer(corev1.IPFamilyPolicyPreferDualStack),
		},
	}

	setSvcExternalIPs(svc, addresses)

	setIPFamily(nProxyCfg, svc)

	setSvcLoadBalancerSettings(serviceCfg, &svc.Spec)

	// Apply service patches
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil && nProxyCfg.Kubernetes.Service != nil {
		if err := applyPatches(svc, nProxyCfg.Kubernetes.Service.Patches); err != nil {
			return svc, fmt.Errorf("failed to apply service patches: %w", err)
		}
	}

	return svc, nil
}

func buildServicePorts(
	ports map[int32]corev1.Protocol,
	healthcheckPort int32,
	serviceType corev1.ServiceType,
	nodePorts []ngfAPIv1alpha2.NodePort,
) []corev1.ServicePort {
	servicePorts := make([]corev1.ServicePort, 0, len(ports))
	for port, protocol := range ports {
		name := fmt.Sprintf("port-%d", port)
		if healthcheckPort > 0 && port == healthcheckPort {
			name = "health"
		}
		servicePort := corev1.ServicePort{
			Name:       name,
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   protocol,
		}

		if serviceType != corev1.ServiceTypeClusterIP {
			for _, nodePort := range nodePorts {
				if nodePort.ListenerPort == port {
					servicePort.NodePort = nodePort.Port
				}
			}
		}

		servicePorts = append(servicePorts, servicePort)
	}

	// need to sort ports so everytime buildNginxService is called it will generate the exact same
	// array of ports. This is needed to satisfy deterministic results of the method.
	sort.Slice(servicePorts, func(i, j int) bool {
		return servicePorts[i].Port < servicePorts[j].Port
	})

	return servicePorts
}

func setSvcExternalIPs(svc *corev1.Service, addresses []gatewayv1.GatewaySpecAddress) {
	for _, address := range addresses {
		if address.Type != nil && *address.Type == gatewayv1.IPAddressType {
			svc.Spec.ExternalIPs = append(svc.Spec.ExternalIPs, address.Value)
		}
	}
}

func setIPFamily(nProxyCfg *graph.EffectiveNginxProxy, svc *corev1.Service) {
	if nProxyCfg != nil && nProxyCfg.IPFamily != nil && *nProxyCfg.IPFamily != ngfAPIv1alpha2.Dual {
		svc.Spec.IPFamilyPolicy = helpers.GetPointer(corev1.IPFamilyPolicySingleStack)
		if *nProxyCfg.IPFamily == ngfAPIv1alpha2.IPv4 {
			svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
		} else {
			svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol}
		}
	}
}

func setSvcLoadBalancerSettings(svcCfg ngfAPIv1alpha2.ServiceSpec, svcSpec *corev1.ServiceSpec) {
	if svcCfg.LoadBalancerIP != nil {
		svcSpec.LoadBalancerIP = *svcCfg.LoadBalancerIP
	}
	if svcCfg.LoadBalancerClass != nil {
		svcSpec.LoadBalancerClass = svcCfg.LoadBalancerClass
	}
	if svcCfg.LoadBalancerSourceRanges != nil {
		svcSpec.LoadBalancerSourceRanges = svcCfg.LoadBalancerSourceRanges
	}
}

func (p *NginxProvisioner) buildNginxDeployment(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ports map[int32]corev1.Protocol,
	selectorLabels map[string]string,
	names resourceNames,
) (client.Object, error) {
	podTemplateSpec := p.buildNginxPodTemplateSpec(
		objectMeta,
		nProxyCfg,
		ports,
		names,
	)

	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil && nProxyCfg.Kubernetes.DaemonSet != nil {
		daemonSet := &appsv1.DaemonSet{
			ObjectMeta: objectMeta,
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: selectorLabels,
				},
				Template: podTemplateSpec,
			},
		}

		// Apply DaemonSet patches
		if err := applyPatches(daemonSet, nProxyCfg.Kubernetes.DaemonSet.Patches); err != nil {
			return daemonSet, fmt.Errorf("failed to apply daemonset patches: %w", err)
		}

		return daemonSet, nil
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: objectMeta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: podTemplateSpec,
		},
	}

	var deploymentCfg ngfAPIv1alpha2.DeploymentSpec
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil && nProxyCfg.Kubernetes.Deployment != nil {
		deploymentCfg = *nProxyCfg.Kubernetes.Deployment
		// Apply Deployment patches
		if err := applyPatches(deployment, nProxyCfg.Kubernetes.Deployment.Patches); err != nil {
			return deployment, fmt.Errorf("failed to apply deployment patches: %w", err)
		}
	}

	// Determine replica count based on HPA status
	replicas := p.determineReplicas(objectMeta, deploymentCfg)
	if replicas != nil {
		deployment.Spec.Replicas = replicas
	}

	return deployment, nil
}

// determineReplicas determines the appropriate replica count for a deployment based on HPA status.
//
// HPA Replicas Management Strategy:
//
// When an HPA is managing a deployment, we must read the current deployment's replicas
// from the cluster and use that value, rather than trying to set our own value or read
// from HPA.Status.DesiredReplicas (which is eventually consistent and stale).
//
// Why we can't use HPA.Status.DesiredReplicas:
// - HPA.Status updates lag behind Deployment.Spec.Replicas changes
// - When HPA scales down: HPA writes Deployment.Spec → then updates its own Status
// - If we read Status during this window, we get the OLD value and overwrite HPA's new value
// - This creates a race condition causing pod churn
//
// Our approach:
// - When HPA exists: Read current deployment replicas from cluster and use that
// - When HPA doesn't exist yet: Set replicas for initial deployment creation
// - When HPA exists but Deployment doesn't exist yet: Set replicas for initial deployment creation
// - When HPA is disabled: Set replicas normally.
func (p *NginxProvisioner) determineReplicas(
	objectMeta metav1.ObjectMeta,
	deploymentCfg ngfAPIv1alpha2.DeploymentSpec,
) *int32 {
	replicas := deploymentCfg.Replicas

	if !isAutoscalingEnabled(&deploymentCfg) {
		return replicas
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err := p.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: objectMeta.Namespace,
		Name:      objectMeta.Name,
	}, hpa)
	if err != nil {
		return replicas
	}

	existingDeployment := &appsv1.Deployment{}
	err = p.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: objectMeta.Namespace,
		Name:      objectMeta.Name,
	}, existingDeployment)

	if err == nil && existingDeployment.Spec.Replicas != nil {
		replicas = existingDeployment.Spec.Replicas
	}

	return replicas
}

// applyPatches applies the provided patches to the given object.
func applyPatches(obj client.Object, patches []ngfAPIv1alpha2.Patch) error {
	if len(patches) == 0 {
		return nil
	}

	// Convert object to JSON
	objData, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	// Apply each patch in sequence
	for i, patch := range patches {
		if patch.Value == nil || len(patch.Value.Raw) == 0 {
			continue
		}
		patchType := ngfAPIv1alpha2.PatchTypeStrategicMerge
		if patch.Type != nil {
			patchType = *patch.Type
		}

		patchData := patch.Value.Raw
		var patchedData []byte

		switch patchType {
		case ngfAPIv1alpha2.PatchTypeStrategicMerge:
			patchedData, err = strategicpatch.StrategicMergePatch(objData, patchData, obj)
			if err != nil {
				return fmt.Errorf("failed to apply %s patch %d: %w", patchType, i, err)
			}
		case ngfAPIv1alpha2.PatchTypeMerge:
			patchedData, err = jsonpatch.MergePatch(objData, patchData)
			if err != nil {
				return fmt.Errorf("failed to apply %s patch %d: %w", patchType, i, err)
			}
		case ngfAPIv1alpha2.PatchTypeJSONPatch:
			jsonPatch, err := jsonpatch.DecodePatch(patchData)
			if err != nil {
				return fmt.Errorf("failed to decode json patch %d: %w", i, err)
			}
			patchedData, err = jsonPatch.Apply(objData)
			if err != nil {
				return fmt.Errorf("failed to apply %s patch %d: %w", patchType, i, err)
			}
		default:
			return fmt.Errorf("unsupported patch type: %s", patchType)
		}

		objData = patchedData
	}

	// Unmarshal back to object
	if err := json.Unmarshal(objData, obj); err != nil {
		return fmt.Errorf("failed to unmarshal patched object: %w", err)
	}

	return nil
}

// buildNginxPodTemplateSpec builds the complete pod template spec.
func (p *NginxProvisioner) buildNginxPodTemplateSpec(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ports map[int32]corev1.Protocol,
	names resourceNames,
) corev1.PodTemplateSpec {
	// Build container ports and pod annotations
	containerPorts, podAnnotations := p.buildContainerPortsAndAnnotations(ports, nProxyCfg, objectMeta.Annotations)

	// Build NGINX container
	nginxContainer := p.buildNginxContainer(containerPorts, nProxyCfg)

	// Build base volumes
	volumes := p.buildBaseVolumes(names)

	// Build containers list
	containers := []corev1.Container{nginxContainer}

	// Configure WAF if enabled
	if graph.WAFEnabledForNginxProxy(nProxyCfg) {
		containers, volumes = p.configureWAF(containers, volumes, nProxyCfg)
	}

	// Build init containers
	initContainers := p.buildInitContainers(nProxyCfg)

	// Create base pod template spec
	spec := p.buildBasePodTemplateSpec(objectMeta, podAnnotations, containers, initContainers, volumes)

	// Apply user configuration overrides
	p.applyUserConfiguration(&spec, nProxyCfg)

	// Configure image pull secrets
	p.configureImagePullSecrets(&spec, names)

	// Configure NGINX Plus if enabled
	if p.cfg.Plus {
		p.configureNginxPlus(&spec, names)
	}

	// Configure dataplane key secret for NGINX One Console telemetry
	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		p.configureDataplaneKeySecret(&spec, names)
	}

	// Configure inference extension if enabled
	if p.cfg.InferenceExtension {
		var containerResources corev1.ResourceRequirements
		if nProxyCfg != nil && nProxyCfg.Kubernetes != nil {
			var containerSpec *ngfAPIv1alpha2.ContainerSpec
			if nProxyCfg.Kubernetes.Deployment != nil {
				containerSpec = &nProxyCfg.Kubernetes.Deployment.Container
			} else if nProxyCfg.Kubernetes.DaemonSet != nil {
				containerSpec = &nProxyCfg.Kubernetes.DaemonSet.Container
			}
			if containerSpec != nil && containerSpec.Resources != nil {
				containerResources = *containerSpec.Resources
			}
		}
		p.configureInferenceExtension(&spec, containerResources)
	}

	return spec
}

// buildContainerPortsAndAnnotations builds container ports and pod annotations.
func (p *NginxProvisioner) buildContainerPortsAndAnnotations(
	ports map[int32]corev1.Protocol,
	nProxyCfg *graph.EffectiveNginxProxy,
	baseAnnotations map[string]string,
) ([]corev1.ContainerPort, map[string]string) {
	containerPorts := make([]corev1.ContainerPort, 0, len(ports))
	for port, protocol := range ports {
		containerPort := corev1.ContainerPort{
			Name:          fmt.Sprintf("port-%d", port),
			ContainerPort: port,
			Protocol:      protocol,
		}
		containerPorts = append(containerPorts, containerPort)
	}

	podAnnotations := make(map[string]string)
	maps.Copy(podAnnotations, baseAnnotations)

	// Add metrics port if enabled
	if port, enabled := graph.MetricsEnabledForNginxProxy(nProxyCfg); enabled {
		metricsPort := config.DefaultNginxMetricsPort
		if port != nil {
			metricsPort = *port
		}

		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          "metrics",
			ContainerPort: metricsPort,
		})

		podAnnotations["prometheus.io/scrape"] = "true"
		podAnnotations["prometheus.io/port"] = strconv.Itoa(int(metricsPort))
	}

	// need to sort ports so everytime buildNginxPodTemplateSpec is called it will generate the exact same
	// array of ports. This is needed to satisfy deterministic results of the method.
	sort.Slice(containerPorts, func(i, j int) bool {
		return containerPorts[i].ContainerPort < containerPorts[j].ContainerPort
	})

	return containerPorts, podAnnotations
}

// buildNginxContainer builds the base NGINX container.
func (p *NginxProvisioner) buildNginxContainer(
	containerPorts []corev1.ContainerPort,
	nProxyCfg *graph.EffectiveNginxProxy,
) corev1.Container {
	image, pullPolicy := p.buildImage(nProxyCfg)

	return corev1.Container{
		Name:            "nginx",
		Image:           image,
		ImagePullPolicy: pullPolicy,
		Ports:           containerPorts,
		ReadinessProbe:  p.buildReadinessProbe(nProxyCfg),
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
				Drop: []corev1.Capability{"ALL"},
			},
			ReadOnlyRootFilesystem: helpers.GetPointer(true),
			RunAsGroup:             helpers.GetPointer[int64](1001),
			RunAsUser:              helpers.GetPointer[int64](101),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{MountPath: "/etc/nginx-agent", Name: "nginx-agent"},
			{MountPath: "/var/run/secrets/ngf", Name: "nginx-agent-tls"},
			{MountPath: "/var/run/secrets/ngf/serviceaccount", Name: "token"},
			{MountPath: "/var/log/nginx-agent", Name: "nginx-agent-log"},
			{MountPath: "/var/lib/nginx-agent", Name: "nginx-agent-lib"},
			{MountPath: "/etc/nginx/conf.d", Name: "nginx-conf"},
			{MountPath: "/etc/nginx/stream-conf.d", Name: "nginx-stream-conf"},
			{MountPath: "/etc/nginx/main-includes", Name: "nginx-main-includes"},
			{MountPath: "/etc/nginx/events-includes", Name: "nginx-events-includes"},
			{MountPath: "/etc/nginx/secrets", Name: "nginx-secrets"},
			{MountPath: "/var/run/nginx", Name: "nginx-run"},
			{MountPath: "/var/cache/nginx", Name: "nginx-cache"},
			{MountPath: "/etc/nginx/includes", Name: "nginx-includes"},
		},
	}
}

// buildBaseVolumes builds the base volumes needed for NGINX.
func (p *NginxProvisioner) buildBaseVolumes(names resourceNames,) []corev1.Volume {
	tokenAudience := fmt.Sprintf("%s.%s.svc", p.cfg.GatewayPodConfig.ServiceName, p.cfg.GatewayPodConfig.Namespace)

	return []corev1.Volume{
		{
			Name: "token",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Path:     "token",
								Audience: tokenAudience,
							},
						},
					},
				},
			},
		},
		{Name: "nginx-agent", VolumeSource: emptyDirVolumeSource},
		{
			Name: "nginx-agent-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: names.nginxAgentConfigMap,
					},
				},
			},
		},
		{
			Name: "nginx-agent-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: names.agentTLS,
				},
			},
		},
		{Name: "nginx-agent-log", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-agent-lib", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-conf", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-stream-conf", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-main-includes", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-events-includes", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-secrets", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-run", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-cache", VolumeSource: emptyDirVolumeSource},
		{Name: "nginx-includes", VolumeSource: emptyDirVolumeSource},
		{
			Name: "nginx-includes-bootstrap",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: names.nginxIncludesConfigMap,
					},
				},
			},
		},
	}
}

// buildInitContainers builds the init containers.
func (p *NginxProvisioner) buildInitContainers(nProxyCfg *graph.EffectiveNginxProxy) []corev1.Container {
	_, pullPolicy := p.buildImage(nProxyCfg)

	clusterID := "unknown"
	if val, ok := p.cfg.AgentLabels["cluster-id"]; ok {
		clusterID = val
	}

	return []corev1.Container{
		{
			Name:            "init",
			Image:           p.cfg.GatewayPodConfig.Image,
			ImagePullPolicy: pullPolicy,
			Command: []string{
				"/usr/bin/gateway",
				"initialize",
				"--source", "/agent/nginx-agent.conf",
				"--destination", "/etc/nginx-agent",
				"--source", "/includes/main.conf",
				"--destination", "/etc/nginx/main-includes",
				"--source", "/includes/events.conf",
				"--destination", "/etc/nginx/events-includes",
			},
			Env: []corev1.EnvVar{
				{
					Name: "POD_UID",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.uid",
						},
					},
				},
				{
					Name:  "CLUSTER_UID",
					Value: clusterID,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{MountPath: "/agent", Name: "nginx-agent-config"},
				{MountPath: "/etc/nginx-agent", Name: "nginx-agent"},
				{MountPath: "/includes", Name: "nginx-includes-bootstrap"},
				{MountPath: "/etc/nginx/main-includes", Name: "nginx-main-includes"},
				{MountPath: "/etc/nginx/events-includes", Name: "nginx-events-includes"},
			},
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				ReadOnlyRootFilesystem: helpers.GetPointer(true),
				RunAsGroup:             helpers.GetPointer[int64](1001),
				RunAsUser:              helpers.GetPointer[int64](101),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
		},
	}
}

// buildBasePodTemplateSpec builds the base pod template spec.
func (p *NginxProvisioner) buildBasePodTemplateSpec(
	objectMeta metav1.ObjectMeta,
	podAnnotations map[string]string,
	containers []corev1.Container,
	initContainers []corev1.Container,
	volumes []corev1.Volume,
) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      objectMeta.Labels,
			Annotations: podAnnotations,
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: helpers.GetPointer(true),
			Containers:                   containers,
			InitContainers:               initContainers,
			ImagePullSecrets:             []corev1.LocalObjectReference{},
			ServiceAccountName:           objectMeta.Name,
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup:      helpers.GetPointer[int64](1001),
				RunAsNonRoot: helpers.GetPointer(true),
				Sysctls: []corev1.Sysctl{
					{
						Name:  "net.ipv4.ip_unprivileged_port_start",
						Value: "0",
					},
				},
			},
			Volumes: volumes,
		},
	}
}

// applyUserConfiguration applies user-defined configuration overrides.
func (p *NginxProvisioner) applyUserConfiguration(
	spec *corev1.PodTemplateSpec,
	nProxyCfg *graph.EffectiveNginxProxy,
) {
	if nProxyCfg == nil || nProxyCfg.Kubernetes == nil {
		return
	}

	var podSpec *ngfAPIv1alpha2.PodSpec
	var containerSpec *ngfAPIv1alpha2.ContainerSpec

	if nProxyCfg.Kubernetes.Deployment != nil {
		podSpec = &nProxyCfg.Kubernetes.Deployment.Pod
		containerSpec = &nProxyCfg.Kubernetes.Deployment.Container
	} else if nProxyCfg.Kubernetes.DaemonSet != nil {
		podSpec = &nProxyCfg.Kubernetes.DaemonSet.Pod
		containerSpec = &nProxyCfg.Kubernetes.DaemonSet.Container
	}

	// Apply pod-level configuration
	if podSpec != nil {
		spec.Spec.TerminationGracePeriodSeconds = podSpec.TerminationGracePeriodSeconds
		spec.Spec.Affinity = podSpec.Affinity
		spec.Spec.NodeSelector = podSpec.NodeSelector
		spec.Spec.Tolerations = podSpec.Tolerations
		spec.Spec.Volumes = append(spec.Spec.Volumes, podSpec.Volumes...)
		spec.Spec.TopologySpreadConstraints = podSpec.TopologySpreadConstraints
	}

	// Apply container-level configuration (NGINX container only)
	if containerSpec != nil {
		container := spec.Spec.Containers[0]
		if containerSpec.Resources != nil {
			container.Resources = *containerSpec.Resources
		}
		container.Lifecycle = containerSpec.Lifecycle
		container.VolumeMounts = append(container.VolumeMounts, containerSpec.VolumeMounts...)

		if containerSpec.Debug != nil && *containerSpec.Debug {
			container.Command = append(container.Command, "/agent/entrypoint.sh")
			container.Args = append(container.Args, "debug")
		}

		for _, hostPort := range containerSpec.HostPorts {
			for i, port := range container.Ports {
				if hostPort.ContainerPort == port.ContainerPort {
					container.Ports[i].HostPort = hostPort.Port
				}
			}
		}

		spec.Spec.Containers[0] = container
	}
}

// configureImagePullSecrets configures image pull secrets.
func (p *NginxProvisioner) configureImagePullSecrets(
	spec *corev1.PodTemplateSpec,
	names resourceNames,
) {
	for name := range names.dockerSecrets {
		ref := corev1.LocalObjectReference{Name: name}
		spec.Spec.ImagePullSecrets = append(spec.Spec.ImagePullSecrets, ref)
	}

	// need to sort secret names so everytime buildNginxPodTemplateSpec is called it will generate the exact same
	// array of secrets. This is needed to satisfy deterministic results of the method.
	sort.Slice(spec.Spec.ImagePullSecrets, func(i, j int) bool {
		return spec.Spec.ImagePullSecrets[i].Name < spec.Spec.ImagePullSecrets[j].Name
	})
}

// configureNginxPlus configures NGINX Plus specific settings.
func (p *NginxProvisioner) configureNginxPlus(
	spec *corev1.PodTemplateSpec,
	names resourceNames,
) {
	// Update init container command
	initCmd := spec.Spec.InitContainers[0].Command
	initCmd = append(initCmd,
		"--source", "/includes/mgmt.conf",
		"--destination", "/etc/nginx/main-includes",
		"--nginx-plus",
	)
	spec.Spec.InitContainers[0].Command = initCmd

	// Add NGINX Plus volumes and volume mounts
	volumeMounts := spec.Spec.Containers[0].VolumeMounts

	// Add nginx-lib volume
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "nginx-lib",
		MountPath: "/var/lib/nginx/state",
	})
	spec.Spec.Volumes = append(spec.Spec.Volumes, corev1.Volume{
		Name:         "nginx-lib",
		VolumeSource: emptyDirVolumeSource,
	})

	// Add JWT license if configured
	if names.jwt != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "nginx-plus-license",
			MountPath: "/etc/nginx/license.jwt",
			SubPath:   "license.jwt",
		})
		spec.Spec.Volumes = append(spec.Spec.Volumes, corev1.Volume{
			Name:         "nginx-plus-license",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: names.jwt}},
		})
	}

	// Add usage certs if configured
	if names.ca != "" || names.clientSSL != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "nginx-plus-usage-certs",
			MountPath: "/etc/nginx/certs-bootstrap/",
		})

		sources := []corev1.VolumeProjection{}
		if names.ca != "" {
			sources = append(sources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: names.ca},
				},
			})
		}
		if names.clientSSL != "" {
			sources = append(sources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: names.clientSSL},
				},
			})
		}

		spec.Spec.Volumes = append(spec.Spec.Volumes, corev1.Volume{
			Name: "nginx-plus-usage-certs",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: sources,
				},
			},
		})
	}

	spec.Spec.Containers[0].VolumeMounts = volumeMounts
}

// configureDataplaneKeySecret configures the dataplane key secret for NGINX One Console telemetry.
func (p *NginxProvisioner) configureDataplaneKeySecret(
	spec *corev1.PodTemplateSpec,
	names resourceNames,
) {
	volumeMounts := spec.Spec.Containers[0].VolumeMounts

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "agent-dataplane-key",
		MountPath: "/etc/nginx-agent/secrets/dataplane.key",
		SubPath:   "dataplane.key",
	})
	spec.Spec.Volumes = append(spec.Spec.Volumes, corev1.Volume{
		Name:         "agent-dataplane-key",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: names.dataplaneKey}},
	})

	spec.Spec.Containers[0].VolumeMounts = volumeMounts
}

// configureInferenceExtension configures the inference extension endpoint-picker sidecar.
func (p *NginxProvisioner) configureInferenceExtension(
	spec *corev1.PodTemplateSpec,
	containerResources corev1.ResourceRequirements,
) {
	command := []string{
		"/usr/bin/gateway",
		"endpoint-picker",
	}

	if p.cfg.EndpointPickerDisableTLS {
		command = append(command, "--endpoint-picker-disable-tls")
	}
	if p.cfg.EndpointPickerTLSSkipVerify {
		command = append(command, "--endpoint-picker-tls-skip-verify")
	}

	spec.Spec.Containers = append(spec.Spec.Containers, corev1.Container{
		Name:            "endpoint-picker-shim",
		Image:           p.cfg.GatewayPodConfig.Image,
		ImagePullPolicy: defaultImagePullPolicy,
		Command:         command,
		Resources:       containerResources,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: helpers.GetPointer(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			ReadOnlyRootFilesystem: helpers.GetPointer(true),
			RunAsGroup:             helpers.GetPointer[int64](1001),
			RunAsUser:              helpers.GetPointer[int64](101),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
	})
}

func (p *NginxProvisioner) buildImage(nProxyCfg *graph.EffectiveNginxProxy) (string, corev1.PullPolicy) {
	return DetermineNginxImageName(nProxyCfg, p.cfg.Plus, p.cfg.GatewayPodConfig.Version)
}

func buildNginxDeploymentHPA(
	objectMeta metav1.ObjectMeta,
	autoScaling *ngfAPIv1alpha2.AutoscalingSpec,
) *autoscalingv2.HorizontalPodAutoscaler {
	cpuUtil := autoScaling.TargetCPUUtilizationPercentage
	memUtil := autoScaling.TargetMemoryUtilizationPercentage

	metricsLen := len(autoScaling.Metrics)
	if cpuUtil != nil {
		metricsLen++
	}
	if memUtil != nil {
		metricsLen++
	}

	metrics := make([]autoscalingv2.MetricSpec, 0, metricsLen)

	if cpuUtil != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: "cpu",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: cpuUtil,
				},
			},
		})
	}

	if memUtil != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: "memory",
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: memUtil,
				},
			},
		})
	}

	metrics = append(metrics, autoScaling.Metrics...)

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: objectMeta,
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       objectMeta.Name,
			},
			MinReplicas: autoScaling.MinReplicas,
			MaxReplicas: autoScaling.MaxReplicas,
			Metrics:     metrics,
			Behavior:    autoScaling.Behavior,
		},
	}
}

// configureWAF configures WAF containers, volume mounts, and volumes.
func (p *NginxProvisioner) configureWAF(
	containers []corev1.Container,
	volumes []corev1.Volume,
	nProxyCfg *graph.EffectiveNginxProxy,
) ([]corev1.Container, []corev1.Volume) {
	// Add WAF containers
	wafContainers := p.buildWAFContainers(nProxyCfg)
	containers = append(containers, wafContainers...)

	// Add WAF volume mounts to NGINX container
	nginxContainer := containers[0]
	nginxContainer.VolumeMounts = append(nginxContainer.VolumeMounts, buildNginxWAFVolumeMounts()...)
	containers[0] = nginxContainer

	// Add WAF volumes
	wafVolumes := buildWAFSharedVolumes()
	volumes = append(volumes, wafVolumes...)

	return containers, volumes
}

// buildWAFSharedVolumes creates the required shared volumes for WAF containers.
func buildWAFSharedVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name:         appProtectBundlesVolumeName,
			VolumeSource: emptyDirVolumeSource,
		},
		{
			Name:         appProtectConfigVolumeName,
			VolumeSource: emptyDirVolumeSource,
		},
		{
			Name:         appProtectBdConfigVolumeName,
			VolumeSource: emptyDirVolumeSource,
		},
	}
}

// buildNginxWAFVolumeMounts creates the required volume mounts for NGINX container when WAF is enabled.
func buildNginxWAFVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      appProtectBundlesVolumeName,
			MountPath: "/etc/app_protect/bundles",
		},
		{
			Name:      appProtectConfigVolumeName,
			MountPath: "/opt/app_protect/config",
		},
		{
			Name:      appProtectBdConfigVolumeName,
			MountPath: "/opt/app_protect/bd_config",
		},
	}
}

// buildWAFContainers creates the WAF enforcer and config manager containers.
func (p *NginxProvisioner) buildWAFContainers(nProxyCfg *graph.EffectiveNginxProxy) []corev1.Container {
	var containers []corev1.Container
	var wafContainersCfg *ngfAPIv1alpha2.WAFContainerSpec

	// Get WAF container configuration
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil {
		if nProxyCfg.Kubernetes.Deployment != nil {
			wafContainersCfg = nProxyCfg.Kubernetes.Deployment.WAFContainers
		} else if nProxyCfg.Kubernetes.DaemonSet != nil {
			wafContainersCfg = nProxyCfg.Kubernetes.DaemonSet.WAFContainers
		}
	}

	// Build WAF Enforcer container
	enforcerContainer := p.buildWAFEnforcerContainer(wafContainersCfg)
	containers = append(containers, enforcerContainer)

	// Build WAF Config Manager container
	configMgrContainer := p.buildWAFConfigManagerContainer(wafContainersCfg)
	containers = append(containers, configMgrContainer)

	return containers
}

// buildWAFEnforcerContainer creates the WAF enforcer container.
func (p *NginxProvisioner) buildWAFEnforcerContainer(
	wafContainersCfg *ngfAPIv1alpha2.WAFContainerSpec,
) corev1.Container {
	image := p.buildWAFImage(
		defaultWAFEnforcerImagePath,
		defaultWAFImageTag,
		wafContainersCfg,
		"enforcer",
	)

	container := corev1.Container{
		Name:            "waf-enforcer",
		Image:           image,
		ImagePullPolicy: defaultImagePullPolicy,
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                helpers.GetPointer[int64](101),
			AllowPrivilegeEscalation: helpers.GetPointer(false),
			RunAsNonRoot:             helpers.GetPointer(true),
			ReadOnlyRootFilesystem:   helpers.GetPointer(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"all"},
			},
		},
		Env: []corev1.EnvVar{
			{Name: "ENFORCER_PORT", Value: "50000"},
			{Name: "ENFORCER_CONFIG_TIMEOUT", Value: "0"},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      appProtectBdConfigVolumeName,
				MountPath: "/opt/app_protect/bd_config",
			},
		},
	}

	// Apply user-configured settings
	if wafContainersCfg != nil && wafContainersCfg.Enforcer != nil {
		if wafContainersCfg.Enforcer.Resources != nil {
			container.Resources = *wafContainersCfg.Enforcer.Resources
		}
		if len(wafContainersCfg.Enforcer.VolumeMounts) > 0 {
			container.VolumeMounts = append(container.VolumeMounts, wafContainersCfg.Enforcer.VolumeMounts...)
		}
	}

	return container
}

// buildWAFConfigManagerContainer creates the WAF config manager container.
func (p *NginxProvisioner) buildWAFConfigManagerContainer(
	wafContainersCfg *ngfAPIv1alpha2.WAFContainerSpec,
) corev1.Container {
	image := p.buildWAFImage(
		defaultWAFConfigMgrImagePath,
		defaultWAFImageTag,
		wafContainersCfg,
		"configManager",
	)

	container := corev1.Container{
		Name:            "waf-config-mgr",
		Image:           image,
		ImagePullPolicy: defaultImagePullPolicy,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: helpers.GetPointer(false),
			RunAsNonRoot:             helpers.GetPointer(false),
			RunAsUser:                helpers.GetPointer[int64](101),
			ReadOnlyRootFilesystem:   helpers.GetPointer(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"all"},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      appProtectBdConfigVolumeName,
				MountPath: "/opt/app_protect/bd_config",
			},
			{
				Name:      appProtectConfigVolumeName,
				MountPath: "/opt/app_protect/config",
			},
			{
				Name:      appProtectBundlesVolumeName,
				MountPath: "/etc/app_protect/bundles",
			},
		},
	}

	// Apply user-configured settings
	if wafContainersCfg != nil && wafContainersCfg.ConfigManager != nil {
		if wafContainersCfg.ConfigManager.Resources != nil {
			container.Resources = *wafContainersCfg.ConfigManager.Resources
		}
		if len(wafContainersCfg.ConfigManager.VolumeMounts) > 0 {
			container.VolumeMounts = append(container.VolumeMounts, wafContainersCfg.ConfigManager.VolumeMounts...)
		}
	}

	return container
}

// buildWAFImage builds the WAF container image string.
func (p *NginxProvisioner) buildWAFImage(
	defaultImagePath,
	defaultTag string,
	wafContainersCfg *ngfAPIv1alpha2.WAFContainerSpec,
	containerType string,
) string {
	image := defaultImagePath
	tag := defaultTag

	if wafContainersCfg != nil {
		var containerCfg *ngfAPIv1alpha2.WAFContainerConfig
		switch containerType {
		case "enforcer":
			containerCfg = wafContainersCfg.Enforcer
		case "configManager":
			containerCfg = wafContainersCfg.ConfigManager
		}

		if containerCfg != nil && containerCfg.Image != nil {
			if containerCfg.Image.Repository != nil {
				image = *containerCfg.Image.Repository
			}
			if containerCfg.Image.Tag != nil {
				tag = *containerCfg.Image.Tag
			}
		}
	}

	return fmt.Sprintf("%s:%s", image, tag)
}

// TODO(sberman): see about how this can be made more elegant. Maybe create some sort of Object factory
// that can better store/build all the objects we need, to reduce the amount of duplicate object lists that we
// have everywhere.
func (p *NginxProvisioner) buildResourcesForInvalidGatewayCleanup(
	deploymentNSName types.NamespacedName,
) []client.Object {
	// Order to delete:
	// 1. deployment/daemonset
	// 2. service
	// 3. hpa
	// 4. role/binding (if openshift)
	// 5. serviceaccount
	// 6. configmaps
	// 7. secrets

	var objects []client.Object

	meta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      name,
			Namespace: deploymentNSName.Namespace,
		}
	}

	resourceName := func(suffix string) string {
		return controller.CreateNginxResourceName(deploymentNSName.Name, suffix)
	}

	baseMeta := meta(deploymentNSName.Name)

	// 1. Deployment/DaemonSet
	objects = append(objects,
		&appsv1.Deployment{ObjectMeta: baseMeta},
		&appsv1.DaemonSet{ObjectMeta: baseMeta},
	)

	// 2. Service
	objects = append(objects, &corev1.Service{ObjectMeta: baseMeta})

	// 3. HPA
	objects = append(objects, &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: baseMeta})

	// 4. OpenShift Role/RoleBinding
	if p.isOpenshift {
		objects = append(objects,
			&rbacv1.Role{ObjectMeta: baseMeta},
			&rbacv1.RoleBinding{ObjectMeta: baseMeta},
		)
	}

	// 5. ServiceAccount
	objects = append(objects, &corev1.ServiceAccount{ObjectMeta: baseMeta})

	// 6. ConfigMaps
	objects = append(objects,
		&corev1.ConfigMap{ObjectMeta: meta(resourceName(nginxIncludesConfigMapNameSuffix))},
		&corev1.ConfigMap{ObjectMeta: meta(resourceName(nginxAgentConfigMapNameSuffix))},
	)

	// 7. Secrets
	objects = append(objects, &corev1.Secret{ObjectMeta: meta(resourceName(p.cfg.AgentTLSSecretName))})

	for _, name := range p.cfg.NginxDockerSecretNames {
		objects = append(objects, &corev1.Secret{ObjectMeta: meta(resourceName(name))})
	}

	if p.cfg.Plus && p.cfg.PlusUsageConfig != nil {
		if p.cfg.PlusUsageConfig.CASecretName != "" {
			objects = append(objects, &corev1.Secret{ObjectMeta: meta(resourceName(p.cfg.PlusUsageConfig.CASecretName))})
		}
		if p.cfg.PlusUsageConfig.ClientSSLSecretName != "" {
			objects = append(objects, &corev1.Secret{ObjectMeta: meta(resourceName(p.cfg.PlusUsageConfig.ClientSSLSecretName))})
		}
		if p.cfg.PlusUsageConfig.SecretName != "" {
			objects = append(objects, &corev1.Secret{ObjectMeta: meta(resourceName(p.cfg.PlusUsageConfig.SecretName))})
		}
	}

	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		objects = append(
			objects,
			&corev1.Secret{ObjectMeta: meta(resourceName(p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName))},
		)
	}

	return objects
}

// buildReadinessProbe creates a readiness probe configuration for the NGINX container.
func (p *NginxProvisioner) buildReadinessProbe(nProxyCfg *graph.EffectiveNginxProxy) *corev1.Probe {
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: dataplane.GetNginxReadinessProbePath(nProxyCfg),
				Port: intstr.FromInt32(dataplane.GetNginxReadinessProbePort(nProxyCfg)),
			},
		},
		InitialDelaySeconds: defaultInitialDelaySeconds,
	}

	var containerSpec *ngfAPIv1alpha2.ContainerSpec
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil {
		if nProxyCfg.Kubernetes.Deployment != nil {
			containerSpec = &nProxyCfg.Kubernetes.Deployment.Container
		} else if nProxyCfg.Kubernetes.DaemonSet != nil {
			containerSpec = &nProxyCfg.Kubernetes.DaemonSet.Container
		}
	}

	if containerSpec == nil || containerSpec.ReadinessProbe == nil {
		return probe
	}

	if containerSpec.ReadinessProbe.InitialDelaySeconds != nil {
		probe.InitialDelaySeconds = *containerSpec.ReadinessProbe.InitialDelaySeconds
	}

	return probe
}

// isNginxReadinessProbeExposed returns true if the readiness probe should be exposed
// through the Gateway Service object for external load balancer healthchecks.
func isNginxReadinessProbeExposed(nProxyCfg *graph.EffectiveNginxProxy) bool {
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil {
		var containerSpec *ngfAPIv1alpha2.ContainerSpec
		if nProxyCfg.Kubernetes.Deployment != nil {
			containerSpec = &nProxyCfg.Kubernetes.Deployment.Container
		} else if nProxyCfg.Kubernetes.DaemonSet != nil {
			containerSpec = &nProxyCfg.Kubernetes.DaemonSet.Container
		}
		if containerSpec != nil && containerSpec.ReadinessProbe != nil && containerSpec.ReadinessProbe.Expose != nil {
			return *containerSpec.ReadinessProbe.Expose
		}
	}
	return false
}

func DetermineNginxImageName(
	nProxyCfg *graph.EffectiveNginxProxy,
	isPlus bool, version string,
) (string, corev1.PullPolicy) {
	image := defaultNginxImagePath
	if isPlus {
		image = defaultNginxPlusImagePath
	}
	tag := version
	pullPolicy := defaultImagePullPolicy

	getImageAndPullPolicy := func(container ngfAPIv1alpha2.ContainerSpec) (string, string, corev1.PullPolicy) {
		if container.Image != nil {
			if container.Image.Repository != nil {
				image = *container.Image.Repository
			}
			if container.Image.Tag != nil {
				tag = *container.Image.Tag
			}
			if container.Image.PullPolicy != nil {
				pullPolicy = corev1.PullPolicy(*container.Image.PullPolicy)
			}
		}

		return image, tag, pullPolicy
	}

	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil {
		if nProxyCfg.Kubernetes.Deployment != nil {
			image, tag, pullPolicy = getImageAndPullPolicy(nProxyCfg.Kubernetes.Deployment.Container)
		} else if nProxyCfg.Kubernetes.DaemonSet != nil {
			image, tag, pullPolicy = getImageAndPullPolicy(nProxyCfg.Kubernetes.DaemonSet.Container)
		}
	}

	return fmt.Sprintf("%s:%s", image, tag), pullPolicy
}
