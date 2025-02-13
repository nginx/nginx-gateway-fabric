package provisioner

import (
	"fmt"
	"maps"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
)

const (
	defaultNginxErrorLogLevel        = "info"
	nginxIncludesConfigMapNameSuffix = "includes-bootstrap"
	nginxAgentConfigMapNameSuffix    = "agent-config"

	defaultServiceType   = corev1.ServiceTypeLoadBalancer
	defaultServicePolicy = corev1.ServiceExternalTrafficPolicyLocal

	defaultNginxImagePath     = "ghcr.io/nginx/nginx-gateway-fabric/nginx"
	defaultNginxPlusImagePath = "private-registry.nginx.com/nginx-gateway-fabric/nginx-plus"
	defaultImagePullPolicy    = corev1.PullIfNotPresent
)

var emptyDirVolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}

func (p *NginxProvisioner) buildNginxResourceObjects(
	resourceName string,
	gateway *gatewayv1.Gateway,
	nProxyCfg *graph.EffectiveNginxProxy,
) []client.Object {
	// TODO(sberman): handle nginx plus config

	ngxIncludesConfigMapName := controller.CreateNginxResourceName(resourceName, nginxIncludesConfigMapNameSuffix)
	ngxAgentConfigMapName := controller.CreateNginxResourceName(resourceName, nginxAgentConfigMapNameSuffix)

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

	configmaps := p.buildNginxConfigMaps(
		objectMeta,
		nProxyCfg,
		ngxIncludesConfigMapName,
		ngxAgentConfigMapName,
	)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: objectMeta,
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
	)

	// order to install resources:
	// scc (if openshift)
	// secrets
	// configmaps
	// serviceaccount
	// service
	// deployment/daemonset

	objects := make([]client.Object, 0, len(configmaps)+3)
	objects = append(objects, configmaps...)
	objects = append(objects, serviceAccount, service, deployment)

	return objects
}

func (p *NginxProvisioner) buildNginxConfigMaps(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
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

	servicePolicy := defaultServicePolicy
	if serviceCfg.ExternalTrafficPolicy != nil {
		servicePolicy = corev1.ServiceExternalTrafficPolicy(*serviceCfg.ExternalTrafficPolicy)
	}

	servicePorts := make([]corev1.ServicePort, 0, len(ports))
	for port := range ports {
		servicePort := corev1.ServicePort{
			Name:       fmt.Sprintf("port-%d", port),
			Port:       port,
			TargetPort: intstr.FromInt32(port),
		}
		servicePorts = append(servicePorts, servicePort)
	}

	svc := &corev1.Service{
		ObjectMeta: objectMeta,
		Spec: corev1.ServiceSpec{
			Type:                  serviceType,
			Ports:                 servicePorts,
			ExternalTrafficPolicy: servicePolicy,
			Selector:              selectorLabels,
		},
	}

	if serviceCfg.LoadBalancerIP != nil {
		svc.Spec.LoadBalancerIP = *serviceCfg.LoadBalancerIP
	}
	if serviceCfg.LoadBalancerSourceRanges != nil {
		svc.Spec.LoadBalancerSourceRanges = serviceCfg.LoadBalancerSourceRanges
	}

	return svc
}

func (p *NginxProvisioner) buildNginxDeployment(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	ports map[int32]struct{},
	selectorLabels map[string]string,
) client.Object {
	podTemplateSpec := p.buildNginxPodTemplateSpec(
		objectMeta,
		nProxyCfg,
		ngxIncludesConfigMapName,
		ngxAgentConfigMapName,
		ports,
	)

	var object client.Object
	// TODO(sberman): daemonset support
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

	object = deployment

	return object
}

func (p *NginxProvisioner) buildNginxPodTemplateSpec(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	ports map[int32]struct{},
) corev1.PodTemplateSpec {
	// TODO(sberman): handle nginx plus; debug

	containerPorts := make([]corev1.ContainerPort, 0, len(ports))
	for port := range ports {
		containerPort := corev1.ContainerPort{
			Name:          fmt.Sprintf("port-%d", port),
			ContainerPort: port,
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

	image, pullPolicy := p.buildImage(nProxyCfg)

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
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add:  []corev1.Capability{"NET_BIND_SERVICE"},
							Drop: []corev1.Capability{"ALL"},
						},
						ReadOnlyRootFilesystem: helpers.GetPointer[bool](true),
						RunAsGroup:             helpers.GetPointer[int64](1001),
						RunAsUser:              helpers.GetPointer[int64](101),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{MountPath: "/etc/nginx-agent", Name: "nginx-agent"},
						{MountPath: "/var/log/nginx-agent", Name: "nginx-agent-log"},
						{MountPath: "/etc/nginx/conf.d", Name: "nginx-conf"},
						{MountPath: "/etc/nginx/stream-conf.d", Name: "nginx-stream-conf"},
						{MountPath: "/etc/nginx/main-includes", Name: "nginx-main-includes"},
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
						ReadOnlyRootFilesystem: helpers.GetPointer[bool](true),
						RunAsGroup:             helpers.GetPointer[int64](1001),
						RunAsUser:              helpers.GetPointer[int64](101),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			ServiceAccountName: objectMeta.Name,
			Volumes: []corev1.Volume{
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
				{Name: "nginx-agent-log", VolumeSource: emptyDirVolumeSource},
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
			},
		},
	}

	if nProxyCfg != nil && nProxyCfg.Kubernetes != nil {
		var podSpec *ngfAPIv1alpha2.PodSpec
		var containerSpec *ngfAPIv1alpha2.ContainerSpec
		if nProxyCfg.Kubernetes.Deployment != nil {
			podSpec = &nProxyCfg.Kubernetes.Deployment.Pod
			containerSpec = &nProxyCfg.Kubernetes.Deployment.Container
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
				container.Resources = *containerSpec.Resources
			}
			container.Lifecycle = containerSpec.Lifecycle
			container.VolumeMounts = append(container.VolumeMounts, containerSpec.VolumeMounts...)
			spec.Spec.Containers[0] = container
		}
	}

	return spec
}

func (p *NginxProvisioner) buildImage(nProxyCfg *graph.EffectiveNginxProxy) (string, corev1.PullPolicy) {
	image := defaultNginxImagePath
	tag := p.cfg.GatewayPodConfig.Version
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
		}
	}

	return fmt.Sprintf("%s:%s", image, tag), pullPolicy
}

func (p *NginxProvisioner) buildNginxResourceObjectsForDeletion(deploymentNSName types.NamespacedName) []client.Object {
	objectMeta := metav1.ObjectMeta{
		Name:      deploymentNSName.Name,
		Namespace: deploymentNSName.Namespace,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: objectMeta,
	}
	service := &corev1.Service{
		ObjectMeta: objectMeta,
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

	// order to delete:
	// deployment/daemonset
	// service
	// serviceaccount
	// configmaps
	// secrets
	// scc (if openshift)

	return []client.Object{deployment, service, serviceAccount, bootstrapCM, agentCM}
}
