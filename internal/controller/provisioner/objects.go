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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	nginxTypes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
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
)

var emptyDirVolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}

//nolint:gocyclo // will refactor at some point
func (p *NginxProvisioner) buildNginxResourceObjects(
	resourceName string,
	gateway *gatewayv1.Gateway,
	nProxyCfg *graph.EffectiveNginxProxy,
) ([]client.Object, error) {
	var errs []error

	// Need to ensure nginx resource objects are generated deterministically. Specifically when generating
	// an object's field by ranging over a map, since ranging over a map is done in random order, we need to
	// do some processing to ensure the generated results are the same each time.

	ngxIncludesConfigMapName := controller.CreateNginxResourceName(resourceName, nginxIncludesConfigMapNameSuffix)
	ngxAgentConfigMapName := controller.CreateNginxResourceName(resourceName, nginxAgentConfigMapNameSuffix)
	agentTLSSecretName := controller.CreateNginxResourceName(resourceName, p.cfg.AgentTLSSecretName)

	var jwtSecretName, caSecretName, clientSSLSecretName string
	if p.cfg.Plus {
		jwtSecretName = controller.CreateNginxResourceName(resourceName, p.cfg.PlusUsageConfig.SecretName)
		if p.cfg.PlusUsageConfig.CASecretName != "" {
			caSecretName = controller.CreateNginxResourceName(resourceName, p.cfg.PlusUsageConfig.CASecretName)
		}
		if p.cfg.PlusUsageConfig.ClientSSLSecretName != "" {
			clientSSLSecretName = controller.CreateNginxResourceName(resourceName, p.cfg.PlusUsageConfig.ClientSSLSecretName)
		}
	}

	var dataplaneKeySecretName string
	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		dataplaneKeySecretName = controller.CreateNginxResourceName(
			resourceName,
			p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName,
		)
	}

	// map key is the new name, value is the original name
	dockerSecretNames := make(map[string]string)
	for _, name := range p.cfg.NginxDockerSecretNames {
		newName := controller.CreateNginxResourceName(resourceName, name)
		dockerSecretNames[newName] = name
	}

	selectorLabels := make(map[string]string)
	maps.Copy(selectorLabels, p.baseLabelSelector.MatchLabels)
	selectorLabels[controller.AppNameLabel] = resourceName

	labels := make(map[string]string)
	annotations := make(map[string]string)

	if len(gateway.GetName()) > controller.MaxServiceNameLen {
		annotations[controller.GatewayLabel] = gateway.GetName()
	} else {
		selectorLabels[controller.GatewayLabel] = gateway.GetName()
	}

	maps.Copy(labels, selectorLabels)

	if gateway.Spec.Infrastructure != nil {
		for key, value := range gateway.Spec.Infrastructure.Labels {
			labels[string(key)] = string(value)
		}

		for key, value := range gateway.Spec.Infrastructure.Annotations {
			annotations[string(key)] = string(value)
		}
	}

	objectMeta := metav1.ObjectMeta{
		Name:        resourceName,
		Namespace:   gateway.GetNamespace(),
		Labels:      labels,
		Annotations: annotations,
	}

	secrets, err := p.buildNginxSecrets(
		objectMeta,
		agentTLSSecretName,
		dockerSecretNames,
		jwtSecretName,
		caSecretName,
		clientSSLSecretName,
		dataplaneKeySecretName,
	)
	if err != nil {
		errs = append(errs, err)
	}

	configmaps := p.buildNginxConfigMaps(
		objectMeta,
		nProxyCfg,
		ngxIncludesConfigMapName,
		ngxAgentConfigMapName,
		caSecretName != "",
		clientSSLSecretName != "",
	)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta:                   objectMeta,
		AutomountServiceAccountToken: helpers.GetPointer(false),
	}

	var openshiftObjs []client.Object
	if p.isOpenshift {
		openshiftObjs = p.buildOpenshiftObjects(objectMeta)
	}

	ports := make(map[int32]corev1.Protocol)
	for _, listener := range gateway.Spec.Listeners {
		var protocol corev1.Protocol
		switch listener.Protocol {
		case gatewayv1.TCPProtocolType:
			protocol = corev1.ProtocolTCP
		case gatewayv1.UDPProtocolType:
			protocol = corev1.ProtocolUDP
		default:
			protocol = corev1.ProtocolTCP
		}
		ports[listener.Port] = protocol
	}

	// Create separate copies of objectMeta for service and deployment to avoid shared map references
	serviceObjectMeta := metav1.ObjectMeta{
		Name:        objectMeta.Name,
		Namespace:   objectMeta.Namespace,
		Labels:      maps.Clone(objectMeta.Labels),
		Annotations: maps.Clone(objectMeta.Annotations),
	}

	deploymentObjectMeta := metav1.ObjectMeta{
		Name:        objectMeta.Name,
		Namespace:   objectMeta.Namespace,
		Labels:      maps.Clone(objectMeta.Labels),
		Annotations: maps.Clone(objectMeta.Annotations),
	}

	service, err := buildNginxService(serviceObjectMeta, nProxyCfg, ports, selectorLabels, gateway.Spec.Addresses)
	if err != nil {
		errs = append(errs, err)
	}

	deployment, err := p.buildNginxDeployment(
		deploymentObjectMeta,
		nProxyCfg,
		ngxIncludesConfigMapName,
		ngxAgentConfigMapName,
		ports,
		selectorLabels,
		agentTLSSecretName,
		dockerSecretNames,
		jwtSecretName,
		caSecretName,
		clientSSLSecretName,
		dataplaneKeySecretName,
	)
	if err != nil {
		errs = append(errs, err)
	}

	// order to install resources:
	// secrets
	// configmaps
	// serviceaccount
	// role/binding (if openshift)
	// service
	// deployment/daemonset
	// hpa

	objects := make([]client.Object, 0, len(configmaps)+len(secrets)+len(openshiftObjs)+3)
	objects = append(objects, secrets...)
	objects = append(objects, configmaps...)
	objects = append(objects, serviceAccount)
	if p.isOpenshift {
		objects = append(objects, openshiftObjs...)
	}

	objects = append(objects, service, deployment)

	if hpa := p.buildHPA(objectMeta, nProxyCfg); hpa != nil {
		objects = append(objects, hpa)
	}

	return objects, errors.Join(errs...)
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
	agentTLSSecretName string,
	dockerSecretNames map[string]string,
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
	dataplaneKeySecretName string,
) ([]client.Object, error) {
	var secrets []client.Object
	var errs []error

	if agentTLSSecretName != "" {
		newSecret, err := p.getAndUpdateSecret(
			p.cfg.AgentTLSSecretName,
			metav1.ObjectMeta{
				Name:        agentTLSSecretName,
				Namespace:   objectMeta.Namespace,
				Labels:      objectMeta.Labels,
				Annotations: objectMeta.Annotations,
			},
			corev1.SecretTypeTLS,
		)
		if err != nil {
			errs = append(errs, err)
		} else {
			secrets = append(secrets, newSecret)
		}
	}

	for newName, origName := range dockerSecretNames {
		newSecret, err := p.getAndUpdateSecret(
			origName,
			metav1.ObjectMeta{
				Name:        newName,
				Namespace:   objectMeta.Namespace,
				Labels:      objectMeta.Labels,
				Annotations: objectMeta.Annotations,
			},
			corev1.SecretTypeDockerConfigJson,
		)
		if err != nil {
			errs = append(errs, err)
		} else {
			secrets = append(secrets, newSecret)
		}
	}

	// need to sort secrets so everytime buildNginxSecrets is called it will generate the exact same
	// array of secrets. This is needed to satisfy deterministic results of the method.
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].GetName() < secrets[j].GetName()
	})

	if jwtSecretName != "" {
		newSecret, err := p.getAndUpdateSecret(
			p.cfg.PlusUsageConfig.SecretName,
			metav1.ObjectMeta{
				Name:        jwtSecretName,
				Namespace:   objectMeta.Namespace,
				Labels:      objectMeta.Labels,
				Annotations: objectMeta.Annotations,
			},
			corev1.SecretTypeOpaque,
		)
		if err != nil {
			errs = append(errs, err)
		} else {
			secrets = append(secrets, newSecret)
		}
	}

	if caSecretName != "" {
		newSecret, err := p.getAndUpdateSecret(
			p.cfg.PlusUsageConfig.CASecretName,
			metav1.ObjectMeta{
				Name:        caSecretName,
				Namespace:   objectMeta.Namespace,
				Labels:      objectMeta.Labels,
				Annotations: objectMeta.Annotations,
			},
			corev1.SecretTypeOpaque,
		)
		if err != nil {
			errs = append(errs, err)
		} else {
			secrets = append(secrets, newSecret)
		}
	}

	if clientSSLSecretName != "" {
		newSecret, err := p.getAndUpdateSecret(
			p.cfg.PlusUsageConfig.ClientSSLSecretName,
			metav1.ObjectMeta{
				Name:        clientSSLSecretName,
				Namespace:   objectMeta.Namespace,
				Labels:      objectMeta.Labels,
				Annotations: objectMeta.Annotations,
			},
			corev1.SecretTypeTLS,
		)
		if err != nil {
			errs = append(errs, err)
		} else {
			secrets = append(secrets, newSecret)
		}
	}

	if dataplaneKeySecretName != "" {
		newSecret, err := p.getAndUpdateSecret(
			p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName,
			metav1.ObjectMeta{
				Name:        dataplaneKeySecretName,
				Namespace:   objectMeta.Namespace,
				Labels:      objectMeta.Labels,
				Annotations: objectMeta.Annotations,
			},
			corev1.SecretTypeOpaque,
		)
		if err != nil {
			errs = append(errs, err)
		} else {
			secrets = append(secrets, newSecret)
		}
	}

	return secrets, errors.Join(errs...)
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
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	caSecret bool,
	clientSSLSecret bool,
) []client.Object {
	var logging *ngfAPIv1alpha2.NginxLogging
	if nProxyCfg != nil && nProxyCfg.Logging != nil {
		logging = nProxyCfg.Logging
	}

	logLevel := defaultNginxErrorLogLevel
	if logging != nil && logging.ErrorLevel != nil {
		logLevel = string(*nProxyCfg.Logging.ErrorLevel)
	}

	workerConnections := dataplane.DefaultWorkerConnections
	if nProxyCfg != nil && nProxyCfg.WorkerConnections != nil {
		workerConnections = *nProxyCfg.WorkerConnections
	}

	mainFields := map[string]interface{}{
		"ErrorLevel":        logLevel,
		"WorkerConnections": workerConnections,
	}

	// Create events ConfigMap data using template
	eventsFields := map[string]interface{}{
		"WorkerConnections": workerConnections,
	}

	bootstrapCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ngxIncludesConfigMapName,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
		},
		Data: map[string]string{
			"main.conf":   string(helpers.MustExecuteTemplate(mainTemplate, mainFields)),
			"events.conf": string(helpers.MustExecuteTemplate(eventsTemplate, eventsFields)),
		},
	}

	if p.cfg.Plus {
		mgmtFields := map[string]interface{}{
			"UsageEndpoint":        p.cfg.PlusUsageConfig.Endpoint,
			"SkipVerify":           p.cfg.PlusUsageConfig.SkipVerify,
			"UsageCASecret":        caSecret,
			"UsageClientSSLSecret": clientSSLSecret,
		}

		bootstrapCM.Data["mgmt.conf"] = string(helpers.MustExecuteTemplate(mgmtTemplate, mgmtFields))
	}

	metricsPort := config.DefaultNginxMetricsPort
	port, enableMetrics := graph.MetricsEnabledForNginxProxy(nProxyCfg)
	if port != nil {
		metricsPort = *port
	}

	depType := nginxTypes.DeploymentType
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil && nProxyCfg.Kubernetes.DaemonSet != nil {
		depType = nginxTypes.DaemonSetType
	}

	p.cfg.AgentLabels[nginxTypes.AgentOwnerNameLabel] = fmt.Sprintf("%s_%s", objectMeta.Namespace, objectMeta.Name)
	p.cfg.AgentLabels[nginxTypes.AgentOwnerTypeLabel] = depType

	agentFields := map[string]interface{}{
		"Plus":          p.cfg.Plus,
		"ServiceName":   p.cfg.GatewayPodConfig.ServiceName,
		"Namespace":     p.cfg.GatewayPodConfig.Namespace,
		"EnableMetrics": enableMetrics,
		"MetricsPort":   metricsPort,
		"AgentLabels":   p.cfg.AgentLabels,
	}

	if logging != nil && logging.AgentLevel != nil {
		agentFields["LogLevel"] = *logging.AgentLevel
	}

	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		agentFields["NginxOneReporting"] = true
		agentFields["EndpointHost"] = p.cfg.NginxOneConsoleTelemetryConfig.EndpointHost
		agentFields["EndpointPort"] = strconv.Itoa(p.cfg.NginxOneConsoleTelemetryConfig.EndpointPort)
		agentFields["EndpointTLSSkipVerify"] = p.cfg.NginxOneConsoleTelemetryConfig.EndpointTLSSkipVerify
	}

	agentCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ngxAgentConfigMapName,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
		},
		Data: map[string]string{
			"nginx-agent.conf": string(helpers.MustExecuteTemplate(agentTemplate, agentFields)),
		},
	}

	return []client.Object{bootstrapCM, agentCM}
}

func (p *NginxProvisioner) buildOpenshiftObjects(objectMeta metav1.ObjectMeta) []client.Object {
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

	return []client.Object{role, roleBinding}
}

func buildNginxService(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ports map[int32]corev1.Protocol,
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

	servicePorts := make([]corev1.ServicePort, 0, len(ports))
	for port, protocol := range ports {
		servicePort := corev1.ServicePort{
			Name:       fmt.Sprintf("port-%d", port),
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   protocol,
		}

		if serviceType != corev1.ServiceTypeClusterIP {
			for _, nodePort := range serviceCfg.NodePorts {
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
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	ports map[int32]corev1.Protocol,
	selectorLabels map[string]string,
	agentTLSSecretName string,
	dockerSecretNames map[string]string,
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
	dataplaneKeySecretName string,
) (client.Object, error) {
	podTemplateSpec := p.buildNginxPodTemplateSpec(
		objectMeta,
		nProxyCfg,
		ngxIncludesConfigMapName,
		ngxAgentConfigMapName,
		ports,
		agentTLSSecretName,
		dockerSecretNames,
		jwtSecretName,
		caSecretName,
		clientSSLSecretName,
		dataplaneKeySecretName,
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

//nolint:gocyclo // will refactor at some point
func (p *NginxProvisioner) buildNginxPodTemplateSpec(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	ports map[int32]corev1.Protocol,
	agentTLSSecretName string,
	dockerSecretNames map[string]string,
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
	dataplaneKeySecretName string,
) corev1.PodTemplateSpec {
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
	maps.Copy(podAnnotations, objectMeta.Annotations)

	metricsPort := config.DefaultNginxMetricsPort
	if port, enabled := graph.MetricsEnabledForNginxProxy(nProxyCfg); enabled {
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

	image, pullPolicy := p.buildImage(nProxyCfg)
	tokenAudience := fmt.Sprintf("%s.%s.svc", p.cfg.GatewayPodConfig.ServiceName, p.cfg.GatewayPodConfig.Namespace)

	clusterID := "unknown"
	if val, ok := p.cfg.AgentLabels["cluster-id"]; ok {
		clusterID = val
	}

	spec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      objectMeta.Labels,
			Annotations: podAnnotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "nginx",
					Image:           image,
					ImagePullPolicy: pullPolicy,
					Ports:           containerPorts,
					ReadinessProbe:  p.buildReadinessProbe(nProxyCfg),
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
				},
			},
			InitContainers: []corev1.Container{
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
				},
			},
			ImagePullSecrets:   []corev1.LocalObjectReference{},
			ServiceAccountName: objectMeta.Name,
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
			Volumes: []corev1.Volume{
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
								Name: ngxAgentConfigMapName,
							},
						},
					},
				},
				{
					Name: "nginx-agent-tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: agentTLSSecretName,
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
								Name: ngxIncludesConfigMapName,
							},
						},
					},
				},
			},
		},
	}

	var containerResources corev1.ResourceRequirements
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil {
		var podSpec *ngfAPIv1alpha2.PodSpec
		var containerSpec *ngfAPIv1alpha2.ContainerSpec
		if nProxyCfg.Kubernetes.Deployment != nil {
			podSpec = &nProxyCfg.Kubernetes.Deployment.Pod
			containerSpec = &nProxyCfg.Kubernetes.Deployment.Container
		} else if nProxyCfg.Kubernetes.DaemonSet != nil {
			podSpec = &nProxyCfg.Kubernetes.DaemonSet.Pod
			containerSpec = &nProxyCfg.Kubernetes.DaemonSet.Container
		}

		if podSpec != nil {
			spec.Spec.TerminationGracePeriodSeconds = podSpec.TerminationGracePeriodSeconds
			spec.Spec.Affinity = podSpec.Affinity
			spec.Spec.NodeSelector = podSpec.NodeSelector
			spec.Spec.Tolerations = podSpec.Tolerations
			spec.Spec.Volumes = append(spec.Spec.Volumes, podSpec.Volumes...)
			spec.Spec.TopologySpreadConstraints = podSpec.TopologySpreadConstraints
		}

		if containerSpec != nil {
			container := spec.Spec.Containers[0]
			if containerSpec.Resources != nil {
				containerResources = *containerSpec.Resources
				container.Resources = containerResources
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

	for name := range dockerSecretNames {
		ref := corev1.LocalObjectReference{Name: name}
		spec.Spec.ImagePullSecrets = append(spec.Spec.ImagePullSecrets, ref)
	}

	// need to sort secret names so everytime buildNginxPodTemplateSpec is called it will generate the exact same
	// array of secrets. This is needed to satisfy deterministic results of the method.
	sort.Slice(spec.Spec.ImagePullSecrets, func(i, j int) bool {
		return spec.Spec.ImagePullSecrets[i].Name < spec.Spec.ImagePullSecrets[j].Name
	})

	if p.cfg.Plus {
		initCmd := spec.Spec.InitContainers[0].Command
		initCmd = append(initCmd,
			"--source", "/includes/mgmt.conf", "--destination", "/etc/nginx/main-includes", "--nginx-plus")
		spec.Spec.InitContainers[0].Command = initCmd

		volumeMounts := spec.Spec.Containers[0].VolumeMounts

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "nginx-lib",
			MountPath: "/var/lib/nginx/state",
		})
		spec.Spec.Volumes = append(spec.Spec.Volumes, corev1.Volume{
			Name:         "nginx-lib",
			VolumeSource: emptyDirVolumeSource,
		})

		if jwtSecretName != "" {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      "nginx-plus-license",
				MountPath: "/etc/nginx/license.jwt",
				SubPath:   "license.jwt",
			})
			spec.Spec.Volumes = append(spec.Spec.Volumes, corev1.Volume{
				Name:         "nginx-plus-license",
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: jwtSecretName}},
			})
		}
		if caSecretName != "" || clientSSLSecretName != "" {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      "nginx-plus-usage-certs",
				MountPath: "/etc/nginx/certs-bootstrap/",
			})

			sources := []corev1.VolumeProjection{}

			if caSecretName != "" {
				sources = append(sources, corev1.VolumeProjection{
					Secret: &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: caSecretName},
					},
				})
			}

			if clientSSLSecretName != "" {
				sources = append(sources, corev1.VolumeProjection{
					Secret: &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: clientSSLSecretName},
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

	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		volumeMounts := spec.Spec.Containers[0].VolumeMounts

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "agent-dataplane-key",
			MountPath: "/etc/nginx-agent/secrets/dataplane.key",
			SubPath:   "dataplane.key",
		})
		spec.Spec.Volumes = append(spec.Spec.Volumes, corev1.Volume{
			Name:         "agent-dataplane-key",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: dataplaneKeySecretName}},
		})

		spec.Spec.Containers[0].VolumeMounts = volumeMounts
	}

	if p.cfg.InferenceExtension {
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
			ImagePullPolicy: pullPolicy,
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

	return spec
}

func (p *NginxProvisioner) buildImage(nProxyCfg *graph.EffectiveNginxProxy) (string, corev1.PullPolicy) {
	return DetermineNginxImageName(nProxyCfg, p.cfg.Plus, p.cfg.GatewayPodConfig.Version)
}

func buildNginxDeploymentHPA(
	objectMeta metav1.ObjectMeta,
	autoScaling *ngfAPIv1alpha2.AutoscalingSpec,
) *autoscalingv2.HorizontalPodAutoscaler {
	if !autoScaling.Enable {
		return nil
	}

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

// TODO(sberman): see about how this can be made more elegant. Maybe create some sort of Object factory
// that can better store/build all the objects we need, to reduce the amount of duplicate object lists that we
// have everywhere.
func (p *NginxProvisioner) buildNginxResourceObjectsForDeletion(deploymentNSName types.NamespacedName) []client.Object {
	// order to delete:
	// deployment/daemonset
	// service
	// hpa
	// role/binding (if openshift)
	// serviceaccount
	// configmaps
	// secrets

	objectMeta := metav1.ObjectMeta{
		Name:      deploymentNSName.Name,
		Namespace: deploymentNSName.Namespace,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: objectMeta,
	}
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: objectMeta,
	}
	service := &corev1.Service{
		ObjectMeta: objectMeta,
	}
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: objectMeta,
	}

	objects := []client.Object{deployment, daemonSet, service, hpa}

	if p.isOpenshift {
		role := &rbacv1.Role{
			ObjectMeta: objectMeta,
		}
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: objectMeta,
		}
		objects = append(objects, role, roleBinding)
	}

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: objectMeta,
	}
	bootstrapCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.CreateNginxResourceName(deploymentNSName.Name, nginxIncludesConfigMapNameSuffix),
			Namespace: deploymentNSName.Namespace,
		},
	}
	agentCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.CreateNginxResourceName(deploymentNSName.Name, nginxAgentConfigMapNameSuffix),
			Namespace: deploymentNSName.Namespace,
		},
	}

	objects = append(objects, serviceAccount, bootstrapCM, agentCM)

	agentTLSSecretName := controller.CreateNginxResourceName(
		deploymentNSName.Name,
		p.cfg.AgentTLSSecretName,
	)
	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSSecretName,
			Namespace: deploymentNSName.Namespace,
		},
	}
	objects = append(objects, agentTLSSecret)

	for _, name := range p.cfg.NginxDockerSecretNames {
		newName := controller.CreateNginxResourceName(deploymentNSName.Name, name)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      newName,
				Namespace: deploymentNSName.Namespace,
			},
		}
		objects = append(objects, secret)
	}

	var jwtSecretName, caSecretName, clientSSLSecretName string
	if p.cfg.Plus {
		if p.cfg.PlusUsageConfig.CASecretName != "" {
			caSecretName = controller.CreateNginxResourceName(deploymentNSName.Name, p.cfg.PlusUsageConfig.CASecretName)
			caSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      caSecretName,
					Namespace: deploymentNSName.Namespace,
				},
			}
			objects = append(objects, caSecret)
		}
		if p.cfg.PlusUsageConfig.ClientSSLSecretName != "" {
			clientSSLSecretName = controller.CreateNginxResourceName(
				deploymentNSName.Name,
				p.cfg.PlusUsageConfig.ClientSSLSecretName,
			)
			clientSSLSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clientSSLSecretName,
					Namespace: deploymentNSName.Namespace,
				},
			}
			objects = append(objects, clientSSLSecret)
		}

		jwtSecretName = controller.CreateNginxResourceName(deploymentNSName.Name, p.cfg.PlusUsageConfig.SecretName)
		jwtSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jwtSecretName,
				Namespace: deploymentNSName.Namespace,
			},
		}
		objects = append(objects, jwtSecret)
	}

	var dataplaneKeySecretName string
	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		dataplaneKeySecretName = controller.CreateNginxResourceName(
			deploymentNSName.Name,
			p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName,
		)
		dataplaneKeySecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dataplaneKeySecretName,
				Namespace: deploymentNSName.Namespace,
			},
		}
		objects = append(objects, dataplaneKeySecret)
	}

	return objects
}

// buildReadinessProbe creates a readiness probe configuration for the NGINX container.
func (p *NginxProvisioner) buildReadinessProbe(nProxyCfg *graph.EffectiveNginxProxy) *corev1.Probe {
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.FromInt32(dataplane.DefaultNginxReadinessProbePort),
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

	if containerSpec.ReadinessProbe.Port != nil {
		probe.HTTPGet.Port = intstr.FromInt32(*containerSpec.ReadinessProbe.Port)
	}

	if containerSpec.ReadinessProbe.InitialDelaySeconds != nil {
		probe.InitialDelaySeconds = *containerSpec.ReadinessProbe.InitialDelaySeconds
	}

	return probe
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
