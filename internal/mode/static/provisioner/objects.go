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
) ([]client.Object, error) {
	// Need to ensure nginx resource objects are generated deterministically. Specifically when generating
	// an object's field by ranging over a map, since ranging over a map is done in random order, we need to
	// do some processing to ensure the generated results are the same each time.

	ngxIncludesConfigMapName := controller.CreateNginxResourceName(resourceName, nginxIncludesConfigMapNameSuffix)
	ngxAgentConfigMapName := controller.CreateNginxResourceName(resourceName, nginxAgentConfigMapNameSuffix)

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
		dockerSecretNames,
		jwtSecretName,
		caSecretName,
		clientSSLSecretName,
	)

	// order to install resources:
	// scc (if openshift)
	// secrets
	// configmaps
	// serviceaccount
	// service
	// deployment/daemonset

	objects := make([]client.Object, 0, len(configmaps)+len(secrets)+3)
	objects = append(objects, secrets...)
	objects = append(objects, configmaps...)
	objects = append(objects, serviceAccount, service, deployment)

	return objects, err
}

func (p *NginxProvisioner) buildNginxSecrets(
	objectMeta metav1.ObjectMeta,
	dockerSecretNames map[string]string,
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
) ([]client.Object, error) {
	var secrets []client.Object
	var errs []error

	for newName, origName := range dockerSecretNames {
		newSecret, err := p.getAndUpdateSecret(
			origName,
			metav1.ObjectMeta{
				Name:        newName,
				Namespace:   objectMeta.Namespace,
				Labels:      objectMeta.Labels,
				Annotations: objectMeta.Annotations,
			},
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
		dockerSecretNames,
		jwtSecretName,
		caSecretName,
		clientSSLSecretName,
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

//nolint:gocyclo // will refactor at some point
func (p *NginxProvisioner) buildNginxPodTemplateSpec(
	objectMeta metav1.ObjectMeta,
	nProxyCfg *graph.EffectiveNginxProxy,
	ngxIncludesConfigMapName string,
	ngxAgentConfigMapName string,
	ports map[int32]struct{},
	dockerSecretNames map[string]string,
	jwtSecretName string,
	caSecretName string,
	clientSSLSecretName string,
) corev1.PodTemplateSpec {
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

	// need to sort ports so everytime buildNginxPodTemplateSpec is called it will generate the exact same
	// array of ports. This is needed to satisfy deterministic results of the method.
	sort.Slice(containerPorts, func(i, j int) bool {
		return containerPorts[i].ContainerPort < containerPorts[j].ContainerPort
	})

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
			ImagePullSecrets:   []corev1.LocalObjectReference{},
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

			if containerSpec.Debug != nil && *containerSpec.Debug {
				container.Command = append(container.Command, "/agent/entrypoint.sh")
				container.Args = append(container.Args, "debug")
			}
			spec.Spec.Containers[0] = container
		}
	}

	for name := range dockerSecretNames {
		ref := corev1.LocalObjectReference{Name: name}
		spec.Spec.ImagePullSecrets = append(spec.Spec.ImagePullSecrets, ref)
	}

	// need to sort ports so everytime buildNginxPodTemplateSpec is called it will generate the exact same
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

// TODO(sberman): see about how this can be made more elegant. Maybe create some sort of Object factory
// that can better store/build all the objects we need, to reduce the amount of duplicate object lists that we
// have everywhere.
func (p *NginxProvisioner) buildNginxResourceObjectsForDeletion(deploymentNSName types.NamespacedName) []client.Object {
	// order to delete:
	// deployment/daemonset
	// service
	// serviceaccount
	// configmaps
	// secrets
	// scc (if openshift)

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

	objects := []client.Object{deployment, service, serviceAccount, bootstrapCM, agentCM}

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
