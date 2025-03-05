package provisioner

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
)

func TestNewStore(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore([]string{"docker-secret"}, "jwt-secret", "ca-secret", "client-ssl-secret")

	g.Expect(store).NotTo(BeNil())
	g.Expect(store.dockerSecretNames).To(HaveKey("docker-secret"))
	g.Expect(store.jwtSecretName).To(Equal("jwt-secret"))
	g.Expect(store.caSecretName).To(Equal("ca-secret"))
	g.Expect(store.clientSSLSecretName).To(Equal("client-ssl-secret"))
}

func TestUpdateGateway(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore(nil, "", "", "")
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gateway",
			Namespace: "default",
		},
	}
	nsName := client.ObjectKeyFromObject(gateway)

	store.updateGateway(gateway)

	g.Expect(store.gateways).To(HaveKey(nsName))
	g.Expect(store.getGateway(nsName)).To(Equal(gateway))
}

func TestDeleteGateway(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore(nil, "", "", "")
	nsName := types.NamespacedName{Name: "test-gateway", Namespace: "default"}
	store.gateways[nsName] = &gatewayv1.Gateway{}

	store.deleteGateway(nsName)

	g.Expect(store.gateways).NotTo(HaveKey(nsName))
	g.Expect(store.getGateway(nsName)).To(BeNil())
}

func TestGetGateways(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore(nil, "", "", "")
	gateway1 := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gateway-1",
			Namespace: "default",
		},
	}
	gateway2 := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gateway-2",
			Namespace: "default",
		},
	}
	nsName1 := client.ObjectKeyFromObject(gateway1)
	nsName2 := client.ObjectKeyFromObject(gateway2)

	store.updateGateway(gateway1)
	store.updateGateway(gateway2)

	gateways := store.getGateways()

	g.Expect(gateways).To(HaveKey(nsName1))
	g.Expect(gateways).To(HaveKey(nsName2))
	g.Expect(gateways[nsName1]).To(Equal(gateway1))
	g.Expect(gateways[nsName2]).To(Equal(gateway2))
}

func TestRegisterResourceInGatewayConfig(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore([]string{"docker-secret"}, "jwt-secret", "ca-secret", "client-ssl-secret")
	nsName := types.NamespacedName{Name: "test-gateway", Namespace: "default"}

	registerAndGetResources := func(obj interface{}) *NginxResources {
		changed := store.registerResourceInGatewayConfig(nsName, obj)
		g.Expect(changed).To(BeTrue(), fmt.Sprintf("failed: %T", obj))
		g.Expect(store.nginxResources).To(HaveKey(nsName), fmt.Sprintf("failed: %T", obj))

		return store.getNginxResourcesForGateway(nsName)
	}

	// Gateway, new config
	gw := &graph.Gateway{}
	resources := registerAndGetResources(gw)
	g.Expect(resources.Gateway).To(Equal(gw))

	// Gateway, updated config
	gw = &graph.Gateway{
		Valid: true,
	}
	resources = registerAndGetResources(gw)
	g.Expect(resources.Gateway).To(Equal(gw))

	defaultMeta := metav1.ObjectMeta{
		Name:      "test-resource",
		Namespace: "default",
	}

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// Deployment
	dep := &appsv1.Deployment{ObjectMeta: defaultMeta}
	resources = registerAndGetResources(dep)
	g.Expect(resources.Deployment).To(Equal(defaultMeta))

	// Deployment again, already exists
	resources = registerAndGetResources(dep)
	g.Expect(resources.Deployment).To(Equal(defaultMeta))

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// Service
	svc := &corev1.Service{ObjectMeta: defaultMeta}
	resources = registerAndGetResources(svc)
	g.Expect(resources.Service).To(Equal(defaultMeta))

	// Service again, already exists
	resources = registerAndGetResources(svc)
	g.Expect(resources.Service).To(Equal(defaultMeta))

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// ServiceAccount
	svcAcct := &corev1.ServiceAccount{ObjectMeta: defaultMeta}
	resources = registerAndGetResources(svcAcct)
	g.Expect(resources.ServiceAccount).To(Equal(defaultMeta))

	// ServiceAccount again, already exists
	resources = registerAndGetResources(svcAcct)
	g.Expect(resources.ServiceAccount).To(Equal(defaultMeta))

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// ConfigMap
	bootstrapCMMeta := metav1.ObjectMeta{
		Name:      controller.CreateNginxResourceName(defaultMeta.Name, nginxIncludesConfigMapNameSuffix),
		Namespace: defaultMeta.Namespace,
	}
	bootstrapCM := &corev1.ConfigMap{ObjectMeta: bootstrapCMMeta}
	resources = registerAndGetResources(bootstrapCM)
	g.Expect(resources.BootstrapConfigMap).To(Equal(bootstrapCMMeta))

	// ConfigMap again, already exists
	resources = registerAndGetResources(bootstrapCM)
	g.Expect(resources.BootstrapConfigMap).To(Equal(bootstrapCMMeta))

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// ConfigMap
	agentCMMeta := metav1.ObjectMeta{
		Name:      controller.CreateNginxResourceName(defaultMeta.Name, nginxAgentConfigMapNameSuffix),
		Namespace: defaultMeta.Namespace,
	}
	agentCM := &corev1.ConfigMap{ObjectMeta: agentCMMeta}
	resources = registerAndGetResources(agentCM)
	g.Expect(resources.AgentConfigMap).To(Equal(agentCMMeta))

	// ConfigMap again, already exists
	resources = registerAndGetResources(agentCM)
	g.Expect(resources.AgentConfigMap).To(Equal(agentCMMeta))

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// Secret
	jwtSecretMeta := metav1.ObjectMeta{
		Name:      controller.CreateNginxResourceName(defaultMeta.Name, store.jwtSecretName),
		Namespace: defaultMeta.Namespace,
	}
	jwtSecret := &corev1.Secret{ObjectMeta: jwtSecretMeta}
	resources = registerAndGetResources(jwtSecret)
	g.Expect(resources.PlusJWTSecret).To(Equal(jwtSecretMeta))

	// Secret again, already exists
	resources = registerAndGetResources(jwtSecret)
	g.Expect(resources.PlusJWTSecret).To(Equal(jwtSecretMeta))

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// Secret
	caSecretMeta := metav1.ObjectMeta{
		Name:      controller.CreateNginxResourceName(defaultMeta.Name, store.caSecretName),
		Namespace: defaultMeta.Namespace,
	}
	caSecret := &corev1.Secret{ObjectMeta: caSecretMeta}
	resources = registerAndGetResources(caSecret)
	g.Expect(resources.PlusCASecret).To(Equal(caSecretMeta))

	// Secret again, already exists
	resources = registerAndGetResources(caSecret)
	g.Expect(resources.PlusCASecret).To(Equal(caSecretMeta))

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// Secret
	clientSSLSecretMeta := metav1.ObjectMeta{
		Name:      controller.CreateNginxResourceName(defaultMeta.Name, store.clientSSLSecretName),
		Namespace: defaultMeta.Namespace,
	}
	clientSSLSecret := &corev1.Secret{ObjectMeta: clientSSLSecretMeta}
	resources = registerAndGetResources(clientSSLSecret)
	g.Expect(resources.PlusClientSSLSecret).To(Equal(clientSSLSecretMeta))

	// Secret again, already exists
	resources = registerAndGetResources(clientSSLSecret)
	g.Expect(resources.PlusClientSSLSecret).To(Equal(clientSSLSecretMeta))

	// clear out resources before next test
	store.deleteResourcesForGateway(nsName)

	// Docker Secret
	dockerSecretMeta := metav1.ObjectMeta{
		Name:      controller.CreateNginxResourceName(defaultMeta.Name, "docker-secret"),
		Namespace: defaultMeta.Namespace,
	}
	dockerSecret := &corev1.Secret{ObjectMeta: dockerSecretMeta}
	resources = registerAndGetResources(dockerSecret)
	g.Expect(resources.DockerSecrets).To(ContainElements(dockerSecretMeta))

	// Docker Secret again, already exists
	resources = registerAndGetResources(dockerSecret)
	g.Expect(resources.DockerSecrets).To(ContainElement(dockerSecretMeta))
}

func TestGatewayChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		original *graph.Gateway
		updated  *graph.Gateway
		name     string
		changed  bool
	}{
		{
			name:     "nil gateway",
			original: nil,
			changed:  true,
		},
		{
			name:     "valid field changes",
			original: &graph.Gateway{Valid: true},
			updated:  &graph.Gateway{Valid: false},
			changed:  true,
		},
		{
			name: "source changes",
			original: &graph.Gateway{Source: &gatewayv1.Gateway{
				Spec: gatewayv1.GatewaySpec{
					Listeners: []gatewayv1.Listener{
						{
							Port: 80,
						},
					},
				},
			}},
			updated: &graph.Gateway{Source: &gatewayv1.Gateway{
				Spec: gatewayv1.GatewaySpec{
					Listeners: []gatewayv1.Listener{
						{
							Port: 81,
						},
					},
				},
			}},
			changed: true,
		},
		{
			name: "effective nginx proxy config changes",
			original: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
						Deployment: &ngfAPIv1alpha2.DeploymentSpec{
							Replicas: helpers.GetPointer[int32](1),
						},
					},
				},
			},
			updated: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
						Deployment: &ngfAPIv1alpha2.DeploymentSpec{
							Replicas: helpers.GetPointer[int32](2),
						},
					},
				},
			},
			changed: true,
		},
		{
			name: "no changes",
			original: &graph.Gateway{Source: &gatewayv1.Gateway{
				Spec: gatewayv1.GatewaySpec{
					Listeners: []gatewayv1.Listener{
						{
							Port: 80,
						},
					},
				},
			}},
			updated: &graph.Gateway{Source: &gatewayv1.Gateway{
				Spec: gatewayv1.GatewaySpec{
					Listeners: []gatewayv1.Listener{
						{
							Port: 80,
						},
					},
				},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(gatewayChanged(test.original, test.updated)).To(Equal(test.changed))
		})
	}
}

func TestDeleteResourcesForGateway(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore(nil, "", "", "")
	nsName := types.NamespacedName{Name: "test-gateway", Namespace: "default"}
	store.nginxResources[nsName] = &NginxResources{}

	store.deleteResourcesForGateway(nsName)

	g.Expect(store.nginxResources).NotTo(HaveKey(nsName))
}

func TestGatewayExistsForResource(t *testing.T) {
	t.Parallel()

	store := newStore(nil, "", "", "")
	gateway := &graph.Gateway{}
	store.nginxResources[types.NamespacedName{Name: "test-gateway", Namespace: "default"}] = &NginxResources{
		Gateway: gateway,
		Deployment: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Service: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		ServiceAccount: metav1.ObjectMeta{
			Name:      "test-serviceaccount",
			Namespace: "default",
		},
		BootstrapConfigMap: metav1.ObjectMeta{
			Name:      "test-bootstrap-configmap",
			Namespace: "default",
		},
		AgentConfigMap: metav1.ObjectMeta{
			Name:      "test-agent-configmap",
			Namespace: "default",
		},
		PlusJWTSecret: metav1.ObjectMeta{
			Name:      "test-jwt-secret",
			Namespace: "default",
		},
		PlusCASecret: metav1.ObjectMeta{
			Name:      "test-ca-secret",
			Namespace: "default",
		},
		PlusClientSSLSecret: metav1.ObjectMeta{
			Name:      "test-client-ssl-secret",
			Namespace: "default",
		},
		DockerSecrets: []metav1.ObjectMeta{
			{
				Name:      "test-docker-secret",
				Namespace: "default",
			},
		},
	}

	tests := []struct {
		expected *graph.Gateway
		object   client.Object
		name     string
	}{
		{
			name: "Deployment exists",
			object: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "Service exists",
			object: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "ServiceAccount exists",
			object: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-serviceaccount",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "Bootstrap ConfigMap exists",
			object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bootstrap-configmap",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "Agent ConfigMap exists",
			object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-configmap",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "JWT Secret exists",
			object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jwt-secret",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "CA Secret exists",
			object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ca-secret",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "Client SSL Secret exists",
			object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-client-ssl-secret",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "Docker Secret exists",
			object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-docker-secret",
					Namespace: "default",
				},
			},
			expected: gateway,
		},
		{
			name: "Resource does not exist",
			object: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-service",
					Namespace: "default",
				},
			},
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := store.gatewayExistsForResource(test.object, client.ObjectKeyFromObject(test.object))
			g.Expect(result).To(Equal(test.expected))
		})
	}
}
