package provisioner

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestBuildNginxResourceObjects(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}
	fakeClient := fake.NewFakeClient(agentTLSSecret)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
				Version:   "1.0.0",
				Image:     "ngf-image",
			},
			AgentTLSSecretName: agentTLSTestSecretName,
			AgentLabels:        make(map[string]string),
		},
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "nginx",
			},
		},
		k8sClient: fakeClient,
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
		Spec: gatewayv1.GatewaySpec{
			Infrastructure: &gatewayv1.GatewayInfrastructure{
				Labels: map[gatewayv1.LabelKey]gatewayv1.LabelValue{
					"label": "value",
				},
				Annotations: map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue{
					"annotation": "value",
				},
			},
			Listeners: []gatewayv1.Listener{
				{
					Port: 80,
				},
				{
					Port: 8888,
				},
				{
					Port: 9999,
				},
			},
			Addresses: []gatewayv1.GatewaySpecAddress{
				{
					Type:  helpers.GetPointer(gatewayv1.IPAddressType),
					Value: "192.0.0.2",
				},
			},
		},
	}

	expLabels := map[string]string{
		"label":                                  "value",
		"app":                                    "nginx",
		"gateway.networking.k8s.io/gateway-name": "gw",
		"app.kubernetes.io/name":                 "gw-nginx",
	}
	expAnnotations := map[string]string{
		"annotation": "value",
	}

	resourceName := "gw-nginx"
	objects, err := provisioner.buildNginxResourceObjects(
		resourceName,
		gateway,
		&graph.EffectiveNginxProxy{
			Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
				Service: &ngfAPIv1alpha2.ServiceSpec{
					NodePorts: []ngfAPIv1alpha2.NodePort{
						{
							Port:         30000,
							ListenerPort: 80,
						},
						{ // ignored
							Port:         31000,
							ListenerPort: 789,
						},
					},
				},
			},
		})
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(objects).To(HaveLen(6))

	validateLabelsAndAnnotations := func(obj client.Object) {
		g.Expect(obj.GetLabels()).To(Equal(expLabels))
		g.Expect(obj.GetAnnotations()).To(Equal(expAnnotations))
	}

	validateMeta := func(obj client.Object) {
		g.Expect(obj.GetName()).To(Equal(resourceName))
		validateLabelsAndAnnotations(obj)
	}

	secretObj := objects[0]
	secret, ok := secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(secret.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, agentTLSTestSecretName)))
	g.Expect(secret.GetLabels()).To(Equal(expLabels))
	g.Expect(secret.GetAnnotations()).To(Equal(expAnnotations))
	g.Expect(secret.Data).To(HaveKey("tls.crt"))
	g.Expect(secret.Data["tls.crt"]).To(Equal([]byte("tls")))

	cmObj := objects[1]
	cm, ok := cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(cm.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, nginxIncludesConfigMapNameSuffix)))
	validateLabelsAndAnnotations(cm)
	g.Expect(cm.Data).To(HaveKey("main.conf"))
	g.Expect(cm.Data["main.conf"]).To(ContainSubstring("info"))

	cmObj = objects[2]
	cm, ok = cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(cm.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, nginxAgentConfigMapNameSuffix)))
	validateLabelsAndAnnotations(cm)
	g.Expect(cm.Data).To(HaveKey("nginx-agent.conf"))
	g.Expect(cm.Data["nginx-agent.conf"]).To(ContainSubstring("command:"))

	svcAcctObj := objects[3]
	svcAcct, ok := svcAcctObj.(*corev1.ServiceAccount)
	g.Expect(ok).To(BeTrue())
	validateMeta(svcAcct)

	svcObj := objects[4]
	svc, ok := svcObj.(*corev1.Service)
	g.Expect(ok).To(BeTrue())
	validateMeta(svc)
	g.Expect(svc.Spec.Type).To(Equal(defaultServiceType))
	g.Expect(svc.Spec.ExternalTrafficPolicy).To(Equal(defaultServicePolicy))
	g.Expect(*svc.Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicyPreferDualStack))

	// service ports is sorted in ascending order by port number when we make the nginx object
	g.Expect(svc.Spec.Ports).To(Equal([]corev1.ServicePort{
		{
			Port:       80,
			Name:       "port-80",
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(80),
			NodePort:   30000,
		},
		{
			Port:       8888,
			Name:       "port-8888",
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8888),
		},
		{
			Port:       9999,
			Name:       "port-9999",
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(9999),
		},
	}))
	g.Expect(svc.Spec.ExternalIPs).To(Equal([]string{"192.0.0.2"}))

	depObj := objects[5]
	dep, ok := depObj.(*appsv1.Deployment)
	g.Expect(ok).To(BeTrue())
	validateMeta(dep)

	template := dep.Spec.Template
	g.Expect(template.GetAnnotations()).To(HaveKey("prometheus.io/scrape"))
	g.Expect(template.Spec.Containers).To(HaveLen(1))
	container := template.Spec.Containers[0]

	// container ports is sorted in ascending order by port number when we make the nginx object
	g.Expect(container.Ports).To(Equal([]corev1.ContainerPort{
		{
			ContainerPort: 80,
			Name:          "port-80",
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: 8888,
			Name:          "port-8888",
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: config.DefaultNginxMetricsPort,
			Name:          "metrics",
		},
		{
			ContainerPort: 9999,
			Name:          "port-9999",
			Protocol:      corev1.ProtocolTCP,
		},
	}))

	g.Expect(container.Image).To(Equal(fmt.Sprintf("%s:1.0.0", defaultNginxImagePath)))
	g.Expect(container.ImagePullPolicy).To(Equal(defaultImagePullPolicy))

	g.Expect(template.Spec.InitContainers).To(HaveLen(1))
	initContainer := template.Spec.InitContainers[0]

	g.Expect(initContainer.Image).To(Equal("ngf-image"))
	g.Expect(initContainer.ImagePullPolicy).To(Equal(defaultImagePullPolicy))
}

func TestBuildNginxResourceObjects_NginxProxyConfig(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}
	fakeClient := fake.NewFakeClient(agentTLSSecret)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
				Version:   "1.0.0",
			},
			AgentTLSSecretName: agentTLSTestSecretName,
			AgentLabels:        make(map[string]string),
		},
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "nginx",
			},
		},
		k8sClient: fakeClient,
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{
				{Name: "port-8443", Port: 8443, Protocol: "tcp"},
			},
		},
	}

	resourceName := "gw-nginx"
	nProxyCfg := &graph.EffectiveNginxProxy{
		IPFamily: helpers.GetPointer(ngfAPIv1alpha2.IPv4),
		Logging: &ngfAPIv1alpha2.NginxLogging{
			ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelDebug),
			AgentLevel: helpers.GetPointer(ngfAPIv1alpha2.AgentLogLevelDebug),
		},
		Metrics: &ngfAPIv1alpha2.Metrics{
			Port: helpers.GetPointer[int32](8080),
		},
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Service: &ngfAPIv1alpha2.ServiceSpec{
				ServiceType:              helpers.GetPointer(ngfAPIv1alpha2.ServiceTypeNodePort),
				ExternalTrafficPolicy:    helpers.GetPointer(ngfAPIv1alpha2.ExternalTrafficPolicyCluster),
				LoadBalancerIP:           helpers.GetPointer("1.2.3.4"),
				LoadBalancerClass:        helpers.GetPointer("myLoadBalancerClass"),
				LoadBalancerSourceRanges: []string{"5.6.7.8"},
			},
			Deployment: &ngfAPIv1alpha2.DeploymentSpec{
				Replicas: helpers.GetPointer[int32](3),
				Autoscaling: &ngfAPIv1alpha2.AutoscalingSpec{
					Enable:                            true,
					MinReplicas:                       helpers.GetPointer[int32](1),
					MaxReplicas:                       5,
					TargetMemoryUtilizationPercentage: helpers.GetPointer[int32](60),
				},
				Pod: ngfAPIv1alpha2.PodSpec{
					TerminationGracePeriodSeconds: helpers.GetPointer[int64](25),
				},
				Container: ngfAPIv1alpha2.ContainerSpec{
					Image: &ngfAPIv1alpha2.Image{
						Repository: helpers.GetPointer("nginx-repo"),
						Tag:        helpers.GetPointer("1.1.1"),
						PullPolicy: helpers.GetPointer(ngfAPIv1alpha2.PullAlways),
					},
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.Quantity{Format: "100m"},
						},
					},
					ReadinessProbe: &ngfAPIv1alpha2.ReadinessProbeSpec{
						Port:                helpers.GetPointer[int32](9091),
						InitialDelaySeconds: helpers.GetPointer[int32](5),
					},
					HostPorts: []ngfAPIv1alpha2.HostPort{{ContainerPort: int32(8443), Port: int32(8443)}},
				},
			},
		},
	}

	objects, err := provisioner.buildNginxResourceObjects(resourceName, gateway, nProxyCfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(objects).To(HaveLen(7))

	cmObj := objects[1]
	cm, ok := cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(cm.Data).To(HaveKey("main.conf"))
	g.Expect(cm.Data["main.conf"]).To(ContainSubstring("debug"))

	cmObj = objects[2]
	cm, ok = cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(cm.Data["nginx-agent.conf"]).To(ContainSubstring("level: debug"))
	g.Expect(cm.Data["nginx-agent.conf"]).To(ContainSubstring("port: 8080"))

	svcObj := objects[4]
	svc, ok := svcObj.(*corev1.Service)
	g.Expect(ok).To(BeTrue())
	g.Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
	g.Expect(svc.Spec.ExternalTrafficPolicy).To(Equal(corev1.ServiceExternalTrafficPolicyTypeCluster))
	g.Expect(svc.Spec.LoadBalancerIP).To(Equal("1.2.3.4"))
	g.Expect(*svc.Spec.LoadBalancerClass).To(Equal("myLoadBalancerClass"))
	g.Expect(svc.Spec.LoadBalancerSourceRanges).To(Equal([]string{"5.6.7.8"}))
	g.Expect(*svc.Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicySingleStack))
	g.Expect(svc.Spec.IPFamilies).To(Equal([]corev1.IPFamily{corev1.IPv4Protocol}))

	depObj := objects[5]
	dep, ok := depObj.(*appsv1.Deployment)
	g.Expect(ok).To(BeTrue())

	template := dep.Spec.Template
	g.Expect(*template.Spec.TerminationGracePeriodSeconds).To(Equal(int64(25)))

	container := template.Spec.Containers[0]

	g.Expect(container.Ports).To(ContainElement(corev1.ContainerPort{
		ContainerPort: 8080,
		Name:          "metrics",
	}))

	g.Expect(container.Image).To(Equal("nginx-repo:1.1.1"))
	g.Expect(container.ImagePullPolicy).To(Equal(corev1.PullAlways))
	g.Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceCPU))
	g.Expect(container.Resources.Limits[corev1.ResourceCPU].Format).To(Equal(resource.Format("100m")))

	g.Expect(container.Ports).To(ContainElement(corev1.ContainerPort{
		ContainerPort: 8443,
		Name:          "port-8443",
		Protocol:      corev1.ProtocolTCP,
		HostPort:      8443,
	}))

	g.Expect(container.ReadinessProbe).ToNot(BeNil())
	g.Expect(container.ReadinessProbe.HTTPGet.Path).To(Equal("/readyz"))
	g.Expect(container.ReadinessProbe.HTTPGet.Port).To(Equal(intstr.FromInt(9091)))
	g.Expect(container.ReadinessProbe.InitialDelaySeconds).To(Equal(int32(5)))

	hpaObj := objects[6]
	hpa, ok := hpaObj.(*autoscalingv2.HorizontalPodAutoscaler)
	g.Expect(ok).To(BeTrue())
	g.Expect(hpa.Spec.MinReplicas).ToNot(BeNil())
	g.Expect(*hpa.Spec.MinReplicas).To(Equal(int32(1)))
	g.Expect(hpa.Spec.MaxReplicas).To(Equal(int32(5)))
}

func TestBuildNginxResourceObjects_DeploymentReplicasFromHPA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		currentReplicas  *int32
		configReplicas   *int32
		name             string
		description      string
		expectedValue    int32
		hpaExists        bool
		deploymentExists bool
		expectedNil      bool
	}{
		{
			name:             "HPA exists - use current deployment replicas",
			hpaExists:        true,
			deploymentExists: true,
			currentReplicas:  helpers.GetPointer(int32(8)),
			configReplicas:   helpers.GetPointer(int32(5)),
			expectedNil:      false,
			expectedValue:    8,
			description:      "When HPA exists, read current deployment replicas (set by HPA)",
		},
		{
			name:             "HPA does not exist - use configured replicas",
			hpaExists:        false,
			deploymentExists: false,
			configReplicas:   helpers.GetPointer(int32(3)),
			expectedNil:      false,
			expectedValue:    3,
			description:      "When HPA doesn't exist yet (initial creation), use configured replicas",
		},
		{
			name:             "HPA enabled but doesn't exist, no configured replicas",
			hpaExists:        false,
			deploymentExists: false,
			configReplicas:   nil,
			expectedNil:      true,
			description:      "When HPA enabled but doesn't exist and no replicas configured, don't set replicas",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			agentTLSSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentTLSTestSecretName,
					Namespace: ngfNamespace,
				},
				Data: map[string][]byte{"tls.crt": []byte("tls")},
			}

			var fakeClient client.Client
			switch {
			case tc.hpaExists && tc.deploymentExists:
				// Create a fake HPA and existing deployment
				hpa := &autoscalingv2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw-nginx",
						Namespace: "default",
					},
					Status: autoscalingv2.HorizontalPodAutoscalerStatus{
						DesiredReplicas: 7,
					},
				}
				existingDeployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw-nginx",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: tc.currentReplicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "nginx"},
						},
					},
				}
				fakeClient = fake.NewFakeClient(agentTLSSecret, hpa, existingDeployment)
			case tc.hpaExists:
				hpa := &autoscalingv2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw-nginx",
						Namespace: "default",
					},
					Status: autoscalingv2.HorizontalPodAutoscalerStatus{
						DesiredReplicas: 7,
					},
				}
				fakeClient = fake.NewFakeClient(agentTLSSecret, hpa)
			default:
				fakeClient = fake.NewFakeClient(agentTLSSecret)
			}

			provisioner := &NginxProvisioner{
				cfg: Config{
					GatewayPodConfig: &config.GatewayPodConfig{
						Namespace: ngfNamespace,
						Version:   "1.0.0",
						Image:     "ngf-image",
					},
					AgentTLSSecretName: agentTLSTestSecretName,
					AgentLabels:        make(map[string]string),
				},
				baseLabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "nginx"},
				},
				k8sClient: fakeClient,
			}

			gateway := &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gw",
					Namespace: "default",
				},
				Spec: gatewayv1.GatewaySpec{
					Listeners: []gatewayv1.Listener{{Port: 80}},
				},
			}

			resourceName := "gw-nginx"
			nProxyCfg := &graph.EffectiveNginxProxy{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Replicas:    tc.configReplicas,
						Autoscaling: &ngfAPIv1alpha2.AutoscalingSpec{Enable: true},
					},
				},
			}

			objects, err := provisioner.buildNginxResourceObjects(resourceName, gateway, nProxyCfg)
			g.Expect(err).ToNot(HaveOccurred())

			// Find the deployment object
			var deployment *appsv1.Deployment
			for _, obj := range objects {
				if d, ok := obj.(*appsv1.Deployment); ok {
					deployment = d
					break
				}
			}
			g.Expect(deployment).ToNot(BeNil())

			if tc.expectedNil {
				g.Expect(deployment.Spec.Replicas).To(BeNil(), tc.description)
			} else {
				g.Expect(deployment.Spec.Replicas).ToNot(BeNil(), tc.description)
				g.Expect(*deployment.Spec.Replicas).To(Equal(tc.expectedValue), tc.description)
			}
		})
	}
}

func TestBuildNginxResourceObjects_Plus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}
	jwtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jwtTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"license.jwt": []byte("jwt")},
	}
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"ca.crt": []byte("ca")},
	}
	clientSSLSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clientTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}

	fakeClient := fake.NewFakeClient(agentTLSSecret, jwtSecret, caSecret, clientSSLSecret)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
				Version:   "1.0.0",
			},
			Plus: true,
			PlusUsageConfig: &config.UsageReportConfig{
				SecretName:          jwtTestSecretName,
				CASecretName:        caTestSecretName,
				ClientSSLSecretName: clientTestSecretName,
				Endpoint:            "test.com",
				SkipVerify:          true,
			},
			AgentTLSSecretName: agentTLSTestSecretName,
			AgentLabels:        make(map[string]string),
		},
		k8sClient: fakeClient,
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "nginx",
			},
		},
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
		Spec: gatewayv1.GatewaySpec{
			Infrastructure: &gatewayv1.GatewayInfrastructure{
				Labels: map[gatewayv1.LabelKey]gatewayv1.LabelValue{
					"label": "value",
				},
				Annotations: map[gatewayv1.AnnotationKey]gatewayv1.AnnotationValue{
					"annotation": "value",
				},
			},
		},
	}

	resourceName := "gw-nginx"
	objects, err := provisioner.buildNginxResourceObjects(resourceName, gateway, &graph.EffectiveNginxProxy{})
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(objects).To(HaveLen(9))

	expLabels := map[string]string{
		"label":                                  "value",
		"app":                                    "nginx",
		"gateway.networking.k8s.io/gateway-name": "gw",
		"app.kubernetes.io/name":                 "gw-nginx",
	}
	expAnnotations := map[string]string{
		"annotation": "value",
	}

	secretObj := objects[1]
	secret, ok := secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(secret.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, jwtTestSecretName)))
	g.Expect(secret.GetLabels()).To(Equal(expLabels))
	g.Expect(secret.GetAnnotations()).To(Equal(expAnnotations))
	g.Expect(secret.Data).To(HaveKey("license.jwt"))
	g.Expect(secret.Data["license.jwt"]).To(Equal([]byte("jwt")))

	secretObj = objects[2]
	secret, ok = secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(secret.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, caTestSecretName)))
	g.Expect(secret.GetLabels()).To(Equal(expLabels))
	g.Expect(secret.GetAnnotations()).To(Equal(expAnnotations))
	g.Expect(secret.Data).To(HaveKey("ca.crt"))
	g.Expect(secret.Data["ca.crt"]).To(Equal([]byte("ca")))

	secretObj = objects[3]
	secret, ok = secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(secret.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, clientTestSecretName)))
	g.Expect(secret.GetLabels()).To(Equal(expLabels))
	g.Expect(secret.GetAnnotations()).To(Equal(expAnnotations))
	g.Expect(secret.Data).To(HaveKey("tls.crt"))
	g.Expect(secret.Data["tls.crt"]).To(Equal([]byte("tls")))

	cmObj := objects[4]
	cm, ok := cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(cm.Data).To(HaveKey("mgmt.conf"))
	g.Expect(cm.Data["mgmt.conf"]).To(ContainSubstring("usage_report endpoint=test.com;"))
	g.Expect(cm.Data["mgmt.conf"]).To(ContainSubstring("ssl_verify off;"))
	g.Expect(cm.Data["mgmt.conf"]).To(ContainSubstring("ssl_trusted_certificate"))
	g.Expect(cm.Data["mgmt.conf"]).To(ContainSubstring("ssl_certificate"))
	g.Expect(cm.Data["mgmt.conf"]).To(ContainSubstring("ssl_certificate_key"))
	g.Expect(cm.Data["mgmt.conf"]).To(ContainSubstring("enforce_initial_report off"))

	cmObj = objects[5]
	cm, ok = cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(cm.Data).To(HaveKey("nginx-agent.conf"))
	g.Expect(cm.Data["nginx-agent.conf"]).To(ContainSubstring("api-action"))

	depObj := objects[8]
	dep, ok := depObj.(*appsv1.Deployment)
	g.Expect(ok).To(BeTrue())

	template := dep.Spec.Template
	container := template.Spec.Containers[0]
	initContainer := template.Spec.InitContainers[0]

	g.Expect(initContainer.Command).To(ContainElement("/includes/mgmt.conf"))
	g.Expect(container.VolumeMounts).To(ContainElement(corev1.VolumeMount{
		Name:      "nginx-plus-license",
		MountPath: "/etc/nginx/license.jwt",
		SubPath:   "license.jwt",
	}))
	g.Expect(container.VolumeMounts).To(ContainElement(corev1.VolumeMount{
		Name:      "nginx-plus-usage-certs",
		MountPath: "/etc/nginx/certs-bootstrap/",
	}))
	g.Expect(container.Image).To(Equal(fmt.Sprintf("%s:1.0.0", defaultNginxPlusImagePath)))
}

func TestBuildNginxResourceObjects_DockerSecrets(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}

	dockerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dockerTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"data": []byte("docker")},
	}

	dockerSecretRegistry1Name := dockerTestSecretName + "-registry1"
	dockerSecretRegistry1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dockerSecretRegistry1Name,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"data": []byte("docker-registry1")},
	}

	dockerSecretRegistry2Name := dockerTestSecretName + "-registry2"
	dockerSecretRegistry2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dockerSecretRegistry2Name,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"data": []byte("docker-registry2")},
	}
	fakeClient := fake.NewFakeClient(agentTLSSecret, dockerSecret, dockerSecretRegistry1, dockerSecretRegistry2)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
			},
			NginxDockerSecretNames: []string{dockerTestSecretName, dockerSecretRegistry1Name, dockerSecretRegistry2Name},
			AgentTLSSecretName:     agentTLSTestSecretName,
			AgentLabels:            make(map[string]string),
		},
		k8sClient: fakeClient,
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "nginx",
			},
		},
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
	}

	resourceName := "gw-nginx"
	objects, err := provisioner.buildNginxResourceObjects(resourceName, gateway, &graph.EffectiveNginxProxy{})
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(objects).To(HaveLen(9))

	expLabels := map[string]string{
		"app":                                    "nginx",
		"gateway.networking.k8s.io/gateway-name": "gw",
		"app.kubernetes.io/name":                 "gw-nginx",
	}

	secretObj := objects[0]
	secret, ok := secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(secret.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, agentTLSTestSecretName)))
	g.Expect(secret.GetLabels()).To(Equal(expLabels))

	// the (docker-only) secret order in the object list is sorted by secret name

	secretObj = objects[1]
	secret, ok = secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(secret.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, dockerTestSecretName)))
	g.Expect(secret.GetLabels()).To(Equal(expLabels))

	registry1SecretObj := objects[2]
	secret, ok = registry1SecretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(secret.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, dockerSecretRegistry1Name)))
	g.Expect(secret.GetLabels()).To(Equal(expLabels))

	registry2SecretObj := objects[3]
	secret, ok = registry2SecretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	g.Expect(secret.GetName()).To(Equal(controller.CreateNginxResourceName(resourceName, dockerSecretRegistry2Name)))
	g.Expect(secret.GetLabels()).To(Equal(expLabels))

	depObj := objects[8]
	dep, ok := depObj.(*appsv1.Deployment)
	g.Expect(ok).To(BeTrue())

	// imagePullSecrets is sorted by name when we make the nginx object
	g.Expect(dep.Spec.Template.Spec.ImagePullSecrets).To(Equal([]corev1.LocalObjectReference{
		{
			Name: controller.CreateNginxResourceName(resourceName, dockerTestSecretName),
		},
		{
			Name: controller.CreateNginxResourceName(resourceName, dockerSecretRegistry1Name),
		},
		{
			Name: controller.CreateNginxResourceName(resourceName, dockerSecretRegistry2Name),
		},
	}))
}

func TestBuildNginxResourceObjects_DaemonSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}
	fakeClient := fake.NewFakeClient(agentTLSSecret)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
			},
			AgentTLSSecretName: agentTLSTestSecretName,
			AgentLabels:        make(map[string]string),
		},
		k8sClient: fakeClient,
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "nginx",
			},
		},
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
	}

	nProxyCfg := &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{
				Pod: ngfAPIv1alpha2.PodSpec{
					TerminationGracePeriodSeconds: helpers.GetPointer[int64](25),
				},
				Container: ngfAPIv1alpha2.ContainerSpec{
					Image: &ngfAPIv1alpha2.Image{
						Repository: helpers.GetPointer("nginx-repo"),
						Tag:        helpers.GetPointer("1.1.1"),
						PullPolicy: helpers.GetPointer(ngfAPIv1alpha2.PullAlways),
					},
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.Quantity{Format: "100m"},
						},
					},
				},
			},
		},
	}

	resourceName := "gw-nginx"
	objects, err := provisioner.buildNginxResourceObjects(resourceName, gateway, nProxyCfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(objects).To(HaveLen(6))

	expLabels := map[string]string{
		"app":                                    "nginx",
		"gateway.networking.k8s.io/gateway-name": "gw",
		"app.kubernetes.io/name":                 "gw-nginx",
	}

	dsObj := objects[5]
	ds, ok := dsObj.(*appsv1.DaemonSet)
	g.Expect(ok).To(BeTrue())
	g.Expect(ds.GetLabels()).To(Equal(expLabels))

	template := ds.Spec.Template
	g.Expect(template.GetAnnotations()).To(HaveKey("prometheus.io/scrape"))
	g.Expect(*template.Spec.TerminationGracePeriodSeconds).To(Equal(int64(25)))

	container := template.Spec.Containers[0]
	g.Expect(container.Image).To(Equal("nginx-repo:1.1.1"))
	g.Expect(container.ImagePullPolicy).To(Equal(corev1.PullAlways))
	g.Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceCPU))
	g.Expect(container.Resources.Limits[corev1.ResourceCPU].Format).To(Equal(resource.Format("100m")))
}

func TestBuildNginxResourceObjects_OpenShift(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}
	fakeClient := fake.NewFakeClient(agentTLSSecret)

	provisioner := &NginxProvisioner{
		isOpenshift: true,
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
			},
			AgentTLSSecretName: agentTLSTestSecretName,
			AgentLabels:        make(map[string]string),
		},
		k8sClient: fakeClient,
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "nginx",
			},
		},
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
	}

	resourceName := "gw-nginx"
	objects, err := provisioner.buildNginxResourceObjects(resourceName, gateway, &graph.EffectiveNginxProxy{})
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(objects).To(HaveLen(8))

	expLabels := map[string]string{
		"app":                                    "nginx",
		"gateway.networking.k8s.io/gateway-name": "gw",
		"app.kubernetes.io/name":                 "gw-nginx",
	}

	roleObj := objects[4]
	role, ok := roleObj.(*rbacv1.Role)
	g.Expect(ok).To(BeTrue())
	g.Expect(role.GetLabels()).To(Equal(expLabels))

	roleBindingObj := objects[5]
	roleBinding, ok := roleBindingObj.(*rbacv1.RoleBinding)
	g.Expect(ok).To(BeTrue())
	g.Expect(roleBinding.GetLabels()).To(Equal(expLabels))
}

func TestBuildNginxResourceObjects_DataplaneKeySecret(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}
	dataplaneKeySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dataplane-key-secret",
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"dataplane.key": []byte("keydata")},
	}
	fakeClient := fake.NewFakeClient(agentTLSSecret, dataplaneKeySecret)

	dataplaneKeySecretName := "dataplane-key-secret" //nolint:gosec // not credentials

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
			},
			AgentTLSSecretName: agentTLSTestSecretName,
			NginxOneConsoleTelemetryConfig: config.NginxOneConsoleTelemetryConfig{
				DataplaneKeySecretName: dataplaneKeySecretName,
				EndpointHost:           "my.endpoint.com",
				EndpointPort:           443,
				EndpointTLSSkipVerify:  false,
			},
			AgentLabels: make(map[string]string),
		},
		k8sClient: fakeClient,
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "nginx",
			},
		},
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
	}

	resourceName := "gw-nginx"
	objects, err := provisioner.buildNginxResourceObjects(resourceName, gateway, &graph.EffectiveNginxProxy{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(HaveLen(7)) // 2 secrets, 2 configmaps, serviceaccount, service, deployment

	// Find the dataplane key secret
	var found bool
	for _, obj := range objects {
		if s, ok := obj.(*corev1.Secret); ok {
			if s.GetName() == controller.CreateNginxResourceName(resourceName, dataplaneKeySecretName) {
				found = true
				g.Expect(s.Data).To(HaveKey("dataplane.key"))
				g.Expect(s.Data["dataplane.key"]).To(Equal([]byte("keydata")))
			}
		}
	}
	g.Expect(found).To(BeTrue())

	// Check deployment mounts the secret
	dep, ok := objects[6].(*appsv1.Deployment)
	g.Expect(ok).To(BeTrue())
	g.Expect(dep).ToNot(BeNil())
	container := dep.Spec.Template.Spec.Containers[0]
	g.Expect(container.VolumeMounts).To(ContainElement(corev1.VolumeMount{
		Name:      "agent-dataplane-key",
		MountPath: "/etc/nginx-agent/secrets/dataplane.key",
		SubPath:   "dataplane.key",
	}))
}

func TestGetAndUpdateSecret_NotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeClient := fake.NewFakeClient()

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: "default",
			},
		},
		k8sClient: fakeClient,
	}

	_, err := provisioner.getAndUpdateSecret(
		"non-existent-secret",
		metav1.ObjectMeta{
			Name:      "new-secret",
			Namespace: "default",
		},
		corev1.SecretTypeOpaque,
	)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("error getting secret"))
}

func TestBuildNginxResourceObjectsForDeletion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	provisioner := &NginxProvisioner{}

	deploymentNSName := types.NamespacedName{
		Name:      "gw-nginx",
		Namespace: "default",
	}

	objects := provisioner.buildNginxResourceObjectsForDeletion(deploymentNSName)

	g.Expect(objects).To(HaveLen(8))

	validateMeta := func(obj client.Object, name string) {
		g.Expect(obj.GetName()).To(Equal(name))
		g.Expect(obj.GetNamespace()).To(Equal(deploymentNSName.Namespace))
	}

	depObj := objects[0]
	dep, ok := depObj.(*appsv1.Deployment)
	g.Expect(ok).To(BeTrue())
	validateMeta(dep, deploymentNSName.Name)

	dsObj := objects[1]
	ds, ok := dsObj.(*appsv1.DaemonSet)
	g.Expect(ok).To(BeTrue())
	validateMeta(ds, deploymentNSName.Name)

	svcObj := objects[2]
	svc, ok := svcObj.(*corev1.Service)
	g.Expect(ok).To(BeTrue())
	validateMeta(svc, deploymentNSName.Name)

	hpaObj := objects[3]
	hpa, ok := hpaObj.(*autoscalingv2.HorizontalPodAutoscaler)
	g.Expect(ok).To(BeTrue())
	validateMeta(hpa, deploymentNSName.Name)

	svcAcctObj := objects[4]
	svcAcct, ok := svcAcctObj.(*corev1.ServiceAccount)
	g.Expect(ok).To(BeTrue())
	validateMeta(svcAcct, deploymentNSName.Name)

	cmObj := objects[5]
	cm, ok := cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	validateMeta(cm, controller.CreateNginxResourceName(deploymentNSName.Name, nginxIncludesConfigMapNameSuffix))

	cmObj = objects[6]
	cm, ok = cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	validateMeta(cm, controller.CreateNginxResourceName(deploymentNSName.Name, nginxAgentConfigMapNameSuffix))
}

func TestBuildNginxResourceObjectsForDeletion_Plus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	provisioner := &NginxProvisioner{
		cfg: Config{
			Plus: true,
			PlusUsageConfig: &config.UsageReportConfig{
				SecretName:          jwtTestSecretName,
				CASecretName:        caTestSecretName,
				ClientSSLSecretName: clientTestSecretName,
			},
			NginxDockerSecretNames: []string{dockerTestSecretName},
			AgentTLSSecretName:     agentTLSTestSecretName,
		},
	}

	deploymentNSName := types.NamespacedName{
		Name:      "gw-nginx",
		Namespace: "default",
	}

	objects := provisioner.buildNginxResourceObjectsForDeletion(deploymentNSName)

	g.Expect(objects).To(HaveLen(12))

	validateMeta := func(obj client.Object, name string) {
		g.Expect(obj.GetName()).To(Equal(name))
		g.Expect(obj.GetNamespace()).To(Equal(deploymentNSName.Namespace))
	}

	depObj := objects[0]
	dep, ok := depObj.(*appsv1.Deployment)
	g.Expect(ok).To(BeTrue())
	validateMeta(dep, deploymentNSName.Name)

	dsObj := objects[1]
	ds, ok := dsObj.(*appsv1.DaemonSet)
	g.Expect(ok).To(BeTrue())
	validateMeta(ds, deploymentNSName.Name)

	svcObj := objects[2]
	svc, ok := svcObj.(*corev1.Service)
	g.Expect(ok).To(BeTrue())
	validateMeta(svc, deploymentNSName.Name)

	hpaObj := objects[3]
	hpa, ok := hpaObj.(*autoscalingv2.HorizontalPodAutoscaler)
	g.Expect(ok).To(BeTrue())
	validateMeta(hpa, deploymentNSName.Name)

	svcAcctObj := objects[4]
	svcAcct, ok := svcAcctObj.(*corev1.ServiceAccount)
	g.Expect(ok).To(BeTrue())
	validateMeta(svcAcct, deploymentNSName.Name)

	cmObj := objects[5]
	cm, ok := cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	validateMeta(cm, controller.CreateNginxResourceName(deploymentNSName.Name, nginxIncludesConfigMapNameSuffix))

	cmObj = objects[6]
	cm, ok = cmObj.(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	validateMeta(cm, controller.CreateNginxResourceName(deploymentNSName.Name, nginxAgentConfigMapNameSuffix))

	secretObj := objects[7]
	secret, ok := secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	validateMeta(secret, controller.CreateNginxResourceName(
		deploymentNSName.Name,
		provisioner.cfg.AgentTLSSecretName,
	))

	secretObj = objects[8]
	secret, ok = secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	validateMeta(secret, controller.CreateNginxResourceName(
		deploymentNSName.Name,
		provisioner.cfg.NginxDockerSecretNames[0],
	))

	secretObj = objects[9]
	secret, ok = secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	validateMeta(secret, controller.CreateNginxResourceName(
		deploymentNSName.Name,
		provisioner.cfg.PlusUsageConfig.CASecretName,
	))

	secretObj = objects[10]
	secret, ok = secretObj.(*corev1.Secret)
	g.Expect(ok).To(BeTrue())
	validateMeta(secret, controller.CreateNginxResourceName(
		deploymentNSName.Name,
		provisioner.cfg.PlusUsageConfig.ClientSSLSecretName,
	))
}

func TestBuildNginxResourceObjectsForDeletion_OpenShift(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	provisioner := &NginxProvisioner{isOpenshift: true}

	deploymentNSName := types.NamespacedName{
		Name:      "gw-nginx",
		Namespace: "default",
	}

	objects := provisioner.buildNginxResourceObjectsForDeletion(deploymentNSName)

	g.Expect(objects).To(HaveLen(10))

	validateMeta := func(obj client.Object, name string) {
		g.Expect(obj.GetName()).To(Equal(name))
		g.Expect(obj.GetNamespace()).To(Equal(deploymentNSName.Namespace))
	}

	hpaObj := objects[3]
	hpa, ok := hpaObj.(*autoscalingv2.HorizontalPodAutoscaler)
	g.Expect(ok).To(BeTrue())
	validateMeta(hpa, deploymentNSName.Name)

	roleObj := objects[4]
	role, ok := roleObj.(*rbacv1.Role)
	g.Expect(ok).To(BeTrue())
	validateMeta(role, deploymentNSName.Name)

	roleBindingObj := objects[5]
	roleBinding, ok := roleBindingObj.(*rbacv1.RoleBinding)
	g.Expect(ok).To(BeTrue())
	validateMeta(roleBinding, deploymentNSName.Name)
}

func TestBuildNginxResourceObjectsForDeletion_DataplaneKeySecret(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	dataplaneKeySecretName := "dataplane-key-secret" //nolint:gosec // not credentials

	provisioner := &NginxProvisioner{
		cfg: Config{
			NginxOneConsoleTelemetryConfig: config.NginxOneConsoleTelemetryConfig{
				DataplaneKeySecretName: dataplaneKeySecretName,
			},
			AgentTLSSecretName: agentTLSTestSecretName,
		},
	}

	deploymentNSName := types.NamespacedName{
		Name:      "gw-nginx",
		Namespace: "default",
	}

	objects := provisioner.buildNginxResourceObjectsForDeletion(deploymentNSName)

	// Should include the dataplane key secret in the objects list
	// Default: deployment, daemonset, service, hpa, serviceaccount, 2 configmaps, agentTLSSecret, dataplaneKeySecret
	g.Expect(objects).To(HaveLen(9))

	validateMeta := func(obj client.Object, name string) {
		g.Expect(obj.GetName()).To(Equal(name))
		g.Expect(obj.GetNamespace()).To(Equal(deploymentNSName.Namespace))
	}

	// Validate the dataplane key secret is present
	found := false
	for _, obj := range objects {
		if s, ok := obj.(*corev1.Secret); ok {
			if s.GetName() == controller.CreateNginxResourceName(deploymentNSName.Name, dataplaneKeySecretName) {
				validateMeta(s, controller.CreateNginxResourceName(deploymentNSName.Name, dataplaneKeySecretName))
				found = true
			}
		}
	}
	g.Expect(found).To(BeTrue())
}

func TestSetIPFamily(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	newSvc := func() *corev1.Service {
		return &corev1.Service{
			Spec: corev1.ServiceSpec{},
		}
	}

	// nProxyCfg is nil, should not set anything
	svc := newSvc()
	setIPFamily(nil, svc)
	g.Expect(svc.Spec.IPFamilyPolicy).To(BeNil())
	g.Expect(svc.Spec.IPFamilies).To(BeNil())

	// nProxyCfg.IPFamily is nil, should not set anything
	svc = newSvc()
	setIPFamily(&graph.EffectiveNginxProxy{}, svc)
	g.Expect(svc.Spec.IPFamilyPolicy).To(BeNil())
	g.Expect(svc.Spec.IPFamilies).To(BeNil())

	// nProxyCfg.IPFamily is IPv4, should set SingleStack and IPFamilies to IPv4
	svc = newSvc()
	ipFamily := ngfAPIv1alpha2.IPv4
	setIPFamily(&graph.EffectiveNginxProxy{IPFamily: &ipFamily}, svc)
	g.Expect(svc.Spec.IPFamilyPolicy).To(Equal(helpers.GetPointer(corev1.IPFamilyPolicySingleStack)))
	g.Expect(svc.Spec.IPFamilies).To(Equal([]corev1.IPFamily{corev1.IPv4Protocol}))

	// nProxyCfg.IPFamily is IPv6, should set SingleStack and IPFamilies to IPv6
	svc = newSvc()
	ipFamily = ngfAPIv1alpha2.IPv6
	setIPFamily(&graph.EffectiveNginxProxy{IPFamily: &ipFamily}, svc)
	g.Expect(svc.Spec.IPFamilyPolicy).To(Equal(helpers.GetPointer(corev1.IPFamilyPolicySingleStack)))
	g.Expect(svc.Spec.IPFamilies).To(Equal([]corev1.IPFamily{corev1.IPv6Protocol}))
}

func TestBuildNginxConfigMaps_WorkerConnections(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace:   "default",
				ServiceName: "test-service",
			},
			AgentLabels: make(map[string]string),
		},
	}
	objectMeta := metav1.ObjectMeta{Name: "test", Namespace: "default"}

	// Test with default worker connections (nil NginxProxy config)
	configMaps := provisioner.buildNginxConfigMaps(objectMeta, nil, "test-bootstrap", "test-agent", false, false)
	g.Expect(configMaps).To(HaveLen(2))

	bootstrapCM, ok := configMaps[0].(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(bootstrapCM.Data["events.conf"]).To(ContainSubstring("worker_connections 1024;"))

	// Test with default worker connections (empty NginxProxy config)
	nProxyCfgEmpty := &graph.EffectiveNginxProxy{}
	configMaps = provisioner.buildNginxConfigMaps(objectMeta, nProxyCfgEmpty, "test-bootstrap", "test-agent", false, false)
	g.Expect(configMaps).To(HaveLen(2))

	bootstrapCM, ok = configMaps[0].(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(bootstrapCM.Data["events.conf"]).To(ContainSubstring("worker_connections 1024;"))

	// Test with custom worker connections
	nProxyCfg := &graph.EffectiveNginxProxy{
		WorkerConnections: helpers.GetPointer(int32(2048)),
	}

	configMaps = provisioner.buildNginxConfigMaps(objectMeta, nProxyCfg, "test-bootstrap", "test-agent", false, false)
	g.Expect(configMaps).To(HaveLen(2))

	bootstrapCM, ok = configMaps[0].(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	g.Expect(bootstrapCM.Data["events.conf"]).To(ContainSubstring("worker_connections 2048;"))
}

func TestBuildNginxConfigMaps_AgentFields(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace:   "default",
				ServiceName: "test-service",
			},
			AgentLabels: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			NginxOneConsoleTelemetryConfig: config.NginxOneConsoleTelemetryConfig{
				DataplaneKeySecretName: "dataplane-key-secret",
				EndpointHost:           "console.example.com",
				EndpointPort:           443,
				EndpointTLSSkipVerify:  false,
			},
		},
	}
	objectMeta := metav1.ObjectMeta{Name: "test", Namespace: "default"}

	nProxyCfgEmpty := &graph.EffectiveNginxProxy{}

	configMaps := provisioner.buildNginxConfigMaps(objectMeta, nProxyCfgEmpty, "test-bootstrap", "test-agent", true, true)
	g.Expect(configMaps).To(HaveLen(2))

	agentCM, ok := configMaps[1].(*corev1.ConfigMap)
	g.Expect(ok).To(BeTrue())
	data := agentCM.Data["nginx-agent.conf"]

	g.Expect(data).To(ContainSubstring("key1: val1"))
	g.Expect(data).To(ContainSubstring("key2: val2"))
	g.Expect(data).To(ContainSubstring("owner-name: default_test"))
	g.Expect(data).To(ContainSubstring("owner-type: Deployment"))
	g.Expect(data).To(ContainSubstring("host: console.example.com"))
	g.Expect(data).To(ContainSubstring("port: 443"))
	g.Expect(data).To(ContainSubstring("skip_verify: false"))
}

func TestBuildReadinessProbe(t *testing.T) {
	t.Parallel()

	defaultProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.FromInt32(dataplane.DefaultNginxReadinessProbePort),
			},
		},
		InitialDelaySeconds: 3,
	}

	provisioner := &NginxProvisioner{}

	tests := []struct {
		nProxyCfg *graph.EffectiveNginxProxy
		expected  *corev1.Probe
		name      string
	}{
		{
			name:      "nginx proxy config is nil, default probe is returned",
			nProxyCfg: nil,
			expected:  defaultProbe,
		},
		{
			name: "deployment is nil, default probe is returned",
			nProxyCfg: &graph.EffectiveNginxProxy{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: nil,
				},
			},
			expected: defaultProbe,
		},
		{
			name: "container is nil, default probe is returned",
			nProxyCfg: &graph.EffectiveNginxProxy{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Container: ngfAPIv1alpha2.ContainerSpec{},
					},
				},
			},
			expected: defaultProbe,
		},
		{
			name: "readinessProbe is nil, default probe is returned",
			nProxyCfg: &graph.EffectiveNginxProxy{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Container: ngfAPIv1alpha2.ContainerSpec{
							ReadinessProbe: nil,
						},
					},
				},
			},
			expected: defaultProbe,
		},
		{
			name: "port & initialDelaySeconds is set in readinessProbe, custom probe is returned",
			nProxyCfg: &graph.EffectiveNginxProxy{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Container: ngfAPIv1alpha2.ContainerSpec{
							ReadinessProbe: &ngfAPIv1alpha2.ReadinessProbeSpec{
								Port:                helpers.GetPointer[int32](9091),
								InitialDelaySeconds: helpers.GetPointer[int32](10),
							},
						},
					},
				},
			},
			expected: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readyz",
						Port: intstr.FromInt32(9091),
					},
				},
				InitialDelaySeconds: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			probe := provisioner.buildReadinessProbe(tt.nProxyCfg)
			g.Expect(probe).To(Equal(tt.expected))
		})
	}
}

func TestBuildNginxResourceObjects_Patches(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}
	fakeClient := fake.NewFakeClient(agentTLSSecret)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
				Version:   "1.0.0",
				Image:     "ngf-image",
			},
			AgentTLSSecretName: agentTLSTestSecretName,
			AgentLabels:        make(map[string]string),
		},
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "nginx",
			},
		},
		k8sClient: fakeClient,
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{
				{Port: 80},
			},
		},
	}

	// Test successful patches with all three resource types and all patch types
	nProxyCfg := &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Service: &ngfAPIv1alpha2.ServiceSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"svc-strategic":"true"}}}`),
						},
					},
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"svc-merge":"true"}}}`),
						},
					},
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeJSONPatch),
						Value: &apiextv1.JSON{
							Raw: []byte(`[{"op": "add", "path": "/metadata/labels/svc-json", "value": "true"}]`),
						},
					},
				},
			},
			Deployment: &ngfAPIv1alpha2.DeploymentSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"dep-patched":"true"}},"spec":{"replicas":3}}`),
						},
					},
				},
			},
		},
	}

	objects, err := provisioner.buildNginxResourceObjects("gw-nginx", gateway, nProxyCfg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(HaveLen(6))

	// Find and validate service
	var svc *corev1.Service
	for _, obj := range objects {
		if s, ok := obj.(*corev1.Service); ok {
			svc = s
			break
		}
	}
	g.Expect(svc).ToNot(BeNil())
	g.Expect(svc.Labels).To(HaveKeyWithValue("svc-strategic", "true"))
	g.Expect(svc.Labels).To(HaveKeyWithValue("svc-merge", "true"))
	g.Expect(svc.Labels).To(HaveKeyWithValue("svc-json", "true"))

	// Find and validate deployment
	var dep *appsv1.Deployment
	for _, obj := range objects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	g.Expect(dep).ToNot(BeNil())
	g.Expect(dep.Labels).To(HaveKeyWithValue("dep-patched", "true"))
	g.Expect(dep.Spec.Replicas).To(Equal(helpers.GetPointer(int32(3))))

	// Test that a later patch overrides a field set by an earlier patch
	nProxyCfg = &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Service: &ngfAPIv1alpha2.ServiceSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"override-label":"first"}}}`),
						},
					},
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"override-label":"second"}}}`),
						},
					},
				},
			},
		},
	}

	objects, err = provisioner.buildNginxResourceObjects("gw-nginx", gateway, nProxyCfg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(HaveLen(6))

	// Find and validate service label override
	svc = nil
	for _, obj := range objects {
		if s, ok := obj.(*corev1.Service); ok {
			svc = s
			break
		}
	}
	g.Expect(svc).ToNot(BeNil())
	g.Expect(svc.Labels).To(HaveKeyWithValue("override-label", "second"))

	// Test successful daemonset patch
	nProxyCfg = &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"ds-patched":"true"}}}`),
						},
					},
				},
			},
		},
	}

	objects, err = provisioner.buildNginxResourceObjects("gw-nginx", gateway, nProxyCfg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(HaveLen(6))

	// Find and validate daemonset
	var ds *appsv1.DaemonSet
	for _, obj := range objects {
		if d, ok := obj.(*appsv1.DaemonSet); ok {
			ds = d
			break
		}
	}
	g.Expect(ds).ToNot(BeNil())
	g.Expect(ds.Labels).To(HaveKeyWithValue("ds-patched", "true"))

	// Test error cases - invalid patches should return objects and errors
	nProxyCfg = &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Service: &ngfAPIv1alpha2.ServiceSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"invalid json":`),
						},
					},
				},
			},
			Deployment: &ngfAPIv1alpha2.DeploymentSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeJSONPatch),
						Value: &apiextv1.JSON{
							Raw: []byte(`[{"op": "invalid", "path": "/test"}]`),
						},
					},
				},
			},
		},
	}

	objects, err = provisioner.buildNginxResourceObjects("gw-nginx", gateway, nProxyCfg)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to apply service patches"))
	g.Expect(err.Error()).To(ContainSubstring("failed to apply deployment patches"))
	g.Expect(objects).To(HaveLen(6)) // Objects should still be returned

	// Test unsupported patch type
	nProxyCfg = &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Service: &ngfAPIv1alpha2.ServiceSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchType("unsupported")),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"test":"true"}}}`),
						},
					},
				},
			},
		},
	}

	objects, err = provisioner.buildNginxResourceObjects("gw-nginx", gateway, nProxyCfg)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unsupported patch type"))
	g.Expect(objects).To(HaveLen(6))

	// Test edge cases - nil values and empty patches should be ignored
	nProxyCfg = &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Service: &ngfAPIv1alpha2.ServiceSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type:  helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: nil, // Should be ignored
					},
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(""), // Should be ignored
						},
					},
				},
			},
		},
	}

	objects, err = provisioner.buildNginxResourceObjects("gw-nginx", gateway, nProxyCfg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(HaveLen(6))

	// Find service and verify no patches were applied
	for _, obj := range objects {
		if s, ok := obj.(*corev1.Service); ok {
			svc = s
			break
		}
	}
	g.Expect(svc).ToNot(BeNil())
	g.Expect(svc.Labels).ToNot(HaveKey("patched")) // Should not have patch-related labels

	// Test that Service patches don't affect Deployment labels and vice versa (cross-contamination)
	nProxyCfg = &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Service: &ngfAPIv1alpha2.ServiceSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"service-only":"true"}}}`),
						},
					},
				},
			},
			Deployment: &ngfAPIv1alpha2.DeploymentSpec{
				Patches: []ngfAPIv1alpha2.Patch{
					{
						Type: helpers.GetPointer(ngfAPIv1alpha2.PatchTypeStrategicMerge),
						Value: &apiextv1.JSON{
							Raw: []byte(`{"metadata":{"labels":{"deployment-only":"true"}}}`),
						},
					},
				},
			},
		},
	}

	objects, err = provisioner.buildNginxResourceObjects("gw-nginx", gateway, nProxyCfg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(HaveLen(6))

	// Find and validate service - should only have service-specific labels
	svc = nil
	for _, obj := range objects {
		if s, ok := obj.(*corev1.Service); ok {
			svc = s
			break
		}
	}
	g.Expect(svc).ToNot(BeNil())
	g.Expect(svc.Labels).To(HaveKeyWithValue("service-only", "true"))
	g.Expect(svc.Labels).ToNot(HaveKey("deployment-only"))

	// Find and validate deployment - should only have deployment-specific labels
	dep = nil
	for _, obj := range objects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			dep = d
			break
		}
	}
	g.Expect(dep).ToNot(BeNil())
	g.Expect(dep.Labels).To(HaveKeyWithValue("deployment-only", "true"))
	g.Expect(dep.Labels).ToNot(HaveKey("service-only"))

	// Both should still have the common base labels
	g.Expect(svc.Labels).To(HaveKeyWithValue("app", "nginx"))
	g.Expect(dep.Labels).To(HaveKeyWithValue("app", "nginx"))
}

func TestBuildNginxResourceObjects_InferenceExtension(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{"tls.crt": []byte("tls")},
	}
	fakeClient := fake.NewFakeClient(agentTLSSecret)

	provisioner := &NginxProvisioner{
		cfg: Config{
			GatewayPodConfig: &config.GatewayPodConfig{
				Namespace: ngfNamespace,
			},
			AgentTLSSecretName:          agentTLSTestSecretName,
			InferenceExtension:          true,
			EndpointPickerDisableTLS:    true,
			EndpointPickerTLSSkipVerify: true,
			AgentLabels:                 make(map[string]string),
		},
		k8sClient: fakeClient,
		baseLabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "nginx"},
		},
	}

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
		},
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Port: 80}},
		},
	}

	npCfg := &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Deployment: &ngfAPIv1alpha2.DeploymentSpec{
				Container: ngfAPIv1alpha2.ContainerSpec{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("500m"),
						},
					},
				},
			},
		},
	}
	objects, err := provisioner.buildNginxResourceObjects("gw-nginx", gateway, npCfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Find the deployment object
	var deployment *appsv1.Deployment
	for _, obj := range objects {
		if d, ok := obj.(*appsv1.Deployment); ok {
			deployment = d
			break
		}
	}

	expectedCommands := []string{
		"/usr/bin/gateway",
		"endpoint-picker",
		"--endpoint-picker-disable-tls",
		"--endpoint-picker-tls-skip-verify",
	}

	g.Expect(deployment).ToNot(BeNil())
	containers := deployment.Spec.Template.Spec.Containers
	g.Expect(containers).To(HaveLen(2))
	g.Expect(containers[1].Name).To(Equal("endpoint-picker-shim"))
	g.Expect(containers[1].Command).To(Equal(expectedCommands))
	g.Expect(containers[1].Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")))
}
