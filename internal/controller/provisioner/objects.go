package provisioner

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
)

const (
	defaultNginxErrorLogLevel        = "info"
	nginxIncludesConfigMapNameSuffix = "includes-bootstrap"
	nginxAgentConfigMapNameSuffix    = "agent-config"

	defaultServiceType   = corev1.ServiceTypeLoadBalancer
	defaultServicePolicy = corev1.ServiceExternalTrafficPolicyLocal

	defaultNginxImagePath        = "ghcr.io/nginx/nginx-gateway-fabric/nginx"
	defaultNginxPlusImagePath    = "private-registry.nginx.com/nginx-gateway-fabric/nginx-plus"
	defaultNginxPlusWafImagePath = "private-registry.nginx.com/nginx-gateway-fabric/nginx-plus-nap-waf"
	defaultImagePullPolicy       = corev1.PullIfNotPresent

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

func (p *NginxProvisioner) buildNginxResourceObjects(
	resourceName string,
	gateway *gatewayv1.Gateway,
	nProxyCfg *graph.EffectiveNginxProxy,
) ([]client.Object, error) {
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

	// map key is the new name, value is the original name
	dockerSecretNames := make(map[string]string)
	for _, name := range p.cfg.NginxDockerSecretNames {
		newName := controller.CreateNginxResourceName(resourceName, name)
		dockerSecretNames[newName] = name
	}

	selectorLabels := make(map[string]string)
	maps.Copy(selectorLabels, p.baseLabelSelector.MatchLabels)
	selectorLabels[controller.GatewayLabel] = gateway.GetName()
	selectorLabels[controller.AppNameLabel] = resourceName

	labels := make(map[string]string)
	annotations := make(map[string]string)

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
	)

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

	ports := make(map[int32]struct{})
	for _, listener := range gateway.Spec.Listeners {
		ports[int32(listener.Port)] = struct{}{}
	}

	service := buildNginxService(objectMeta, nProxyCfg, ports, selectorLabels)
	deployment := p.buildNginxDeployment(
		objectMeta,
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
	)

	// order to install resources:
	// secrets
	// configmaps
	// serviceaccount
	// role/binding (if openshift)
	// service
	// deployment/daemonset

	objects := make([]client.Object, 0, len(configmaps)+len(secrets)+len(openshiftObjs)+3)
	objects = append(objects, secrets...)
	objects = append(objects, configmaps...)
	objects = append(objects, serviceAccount)
	if p.isOpenshift {
		objects = append(objects, openshiftObjs...)
	}
	objects = append(objects, service, deployment)

	return objects, err
}

func (p *NginxProvisioner) buildNginxSecrets(
	objectMeta metav1.ObjectMeta,
	agentTLSSecretName string,
	dockerSecretNames map[string]string,
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
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

	mainFields := map[string]interface{}{
		"ErrorLevel": logLevel,
	}

	bootstrapCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ngxIncludesConfigMapName,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
		},
		Data: map[string]string{
			"main.conf": string(helpers.MustExecuteTemplate(mainTemplate, mainFields)),
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

	agentFields := map[string]interface{}{
		"Plus":          p.cfg.Plus,
		"ServiceName":   p.cfg.GatewayPodConfig.ServiceName,
		"Namespace":     p.cfg.GatewayPodConfig.Namespace,
		"EnableMetrics": enableMetrics,
		"MetricsPort":   metricsPort,
	}

	if logging != nil && logging.AgentLevel != nil {
		agentFields["LogLevel"] = *logging.AgentLevel
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
	ports map[int32]struct{},
	selectorLabels map[string]string,
) *corev1.Service {
	var serviceCfg ngfAPIv1alpha2.ServiceSpec
	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil && nProxyCfg.Kubernetes.Service != nil {
		serviceCfg = *nProxyCfg.Kubernetes.Service
	}

	serviceType := defaultServiceType
	if serviceCfg.ServiceType != nil {
		serviceType = corev1.ServiceType(*serviceCfg.ServiceType)
	}

	var servicePolicy corev1.ServiceExternalTrafficPolicyType
	if serviceType != corev1.ServiceTypeClusterIP {
		servicePolicy = defaultServicePolicy
		if serviceCfg.ExternalTrafficPolicy != nil {
			servicePolicy = corev1.ServiceExternalTrafficPolicy(*serviceCfg.ExternalTrafficPolicy)
		}
	}

	servicePorts := make([]corev1.ServicePort, 0, len(ports))
	for port := range ports {
		servicePort := corev1.ServicePort{
			Name:       fmt.Sprintf("port-%d", port),
			Port:       port,
			TargetPort: intstr.FromInt32(port),
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

	setIPFamily(nProxyCfg, svc)

	if serviceCfg.LoadBalancerIP != nil {
		svc.Spec.LoadBalancerIP = *serviceCfg.LoadBalancerIP
	}
	if serviceCfg.LoadBalancerClass != nil {
		svc.Spec.LoadBalancerClass = serviceCfg.LoadBalancerClass
	}
	if serviceCfg.LoadBalancerSourceRanges != nil {
		svc.Spec.LoadBalancerSourceRanges = serviceCfg.LoadBalancerSourceRanges
	}

	return svc
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

func (p *NginxProvisioner) buildNginxDeployment(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	ports map[int32]struct{},
	selectorLabels map[string]string,
	agentTLSSecretName string,
	dockerSecretNames map[string]string,
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
) client.Object {
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
	)

	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil && nProxyCfg.Kubernetes.DaemonSet != nil {
		return &appsv1.DaemonSet{
			ObjectMeta: objectMeta,
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: selectorLabels,
				},
				Template: podTemplateSpec,
			},
		}
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
	}

	if deploymentCfg.Replicas != nil {
		deployment.Spec.Replicas = deploymentCfg.Replicas
	}

	return deployment
}

// buildNginxPodTemplateSpec builds the complete pod template spec.
func (p *NginxProvisioner) buildNginxPodTemplateSpec(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	ports map[int32]struct{},
	agentTLSSecretName string,
	dockerSecretNames map[string]string,
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
) corev1.PodTemplateSpec {
	// Build container ports and pod annotations
	containerPorts, podAnnotations := p.buildContainerPortsAndAnnotations(ports, nProxyCfg, objectMeta.Annotations)

	// Build NGINX container
	nginxContainer := p.buildNginxContainer(containerPorts, nProxyCfg)

	// Build base volumes
	volumes := p.buildBaseVolumes(ngxIncludesConfigMapName, ngxAgentConfigMapName, agentTLSSecretName)

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
	p.configureImagePullSecrets(&spec, dockerSecretNames)

	// Configure NGINX Plus if enabled
	if p.cfg.Plus {
		p.configureNginxPlus(&spec, jwtSecretName, caSecretName, clientSSLSecretName)
	}

	return spec
}

// buildContainerPortsAndAnnotations builds container ports and pod annotations.
func (p *NginxProvisioner) buildContainerPortsAndAnnotations(
	ports map[int32]struct{},
	nProxyCfg *graph.EffectiveNginxProxy,
	baseAnnotations map[string]string,
) ([]corev1.ContainerPort, map[string]string) {
	containerPorts := make([]corev1.ContainerPort, 0, len(ports))
	for port := range ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          fmt.Sprintf("port-%d", port),
			ContainerPort: port,
		})
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
			{MountPath: "/etc/nginx/secrets", Name: "nginx-secrets"},
			{MountPath: "/var/run/nginx", Name: "nginx-run"},
			{MountPath: "/var/cache/nginx", Name: "nginx-cache"},
			{MountPath: "/etc/nginx/includes", Name: "nginx-includes"},
		},
	}
}

// buildBaseVolumes builds the base volumes needed for NGINX.
func (p *NginxProvisioner) buildBaseVolumes(
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	agentTLSSecretName string,
) []corev1.Volume {
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
	}
}

// buildInitContainers builds the init containers.
func (p *NginxProvisioner) buildInitContainers(nProxyCfg *graph.EffectiveNginxProxy) []corev1.Container {
	_, pullPolicy := p.buildImage(nProxyCfg)

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
			},
			VolumeMounts: []corev1.VolumeMount{
				{MountPath: "/agent", Name: "nginx-agent-config"},
				{MountPath: "/etc/nginx-agent", Name: "nginx-agent"},
				{MountPath: "/includes", Name: "nginx-includes-bootstrap"},
				{MountPath: "/etc/nginx/main-includes", Name: "nginx-main-includes"},
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
	dockerSecretNames map[string]string,
) {
	for name := range dockerSecretNames {
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
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
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

	// Add usage certs if configured
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

func (p *NginxProvisioner) buildImage(nProxyCfg *graph.EffectiveNginxProxy) (string, corev1.PullPolicy) {
	image := defaultNginxImagePath
	tag := p.cfg.GatewayPodConfig.Version
	pullPolicy := defaultImagePullPolicy

	if p.cfg.Plus {
		image = defaultNginxPlusImagePath
	}

	if graph.WAFEnabledForNginxProxy(nProxyCfg) {
		image = defaultNginxPlusWafImagePath
	}

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
func (p *NginxProvisioner) buildNginxResourceObjectsForDeletion(deploymentNSName types.NamespacedName) []client.Object {
	// order to delete:
	// deployment/daemonset
	// service
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

	objects := []client.Object{deployment, daemonSet, service}

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

	return objects
}
