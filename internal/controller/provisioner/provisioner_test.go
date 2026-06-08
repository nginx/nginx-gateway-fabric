package provisioner

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/rest"
	k8sEvents "k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/agentfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/provisioner/openshift/openshiftfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller/controllerfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const (
	agentTLSTestSecretName         = "agent-tls-secret"
	jwtTestSecretName              = "jwt-secret"
	caTestSecretName               = "ca-secret"
	clientTestSecretName           = "client-secret"
	dockerTestSecretName           = "docker-secret"
	ngfNamespace                   = "nginx-gateway"
	nginxOneDataplaneKeySecretName = "dataplane-key"
)

func createScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(autoscalingv2.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))

	return scheme
}

func createFakeClientWithScheme(objects ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(createScheme()).WithObjects(objects...).Build()
}

func expectResourcesToExist(t *testing.T, g *WithT, k8sClient client.Client, nsName types.NamespacedName, plus bool) {
	t.Helper()
	g.Expect(k8sClient.Get(t.Context(), nsName, &appsv1.Deployment{})).To(Succeed())

	g.Expect(k8sClient.Get(t.Context(), nsName, &corev1.Service{})).To(Succeed())

	g.Expect(k8sClient.Get(t.Context(), nsName, &corev1.ServiceAccount{})).To(Succeed())

	bootstrapCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxIncludesConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), bootstrapCM, &corev1.ConfigMap{})).To(Succeed())

	agentCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxAgentConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), agentCM, &corev1.ConfigMap{})).To(Succeed())

	agentTLSSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, agentTLSTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), agentTLSSecret, &corev1.Secret{})).To(Succeed())

	if !plus {
		return
	}

	jwtSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, jwtTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), jwtSecret, &corev1.Secret{})).To(Succeed())

	caSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, caTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), caSecret, &corev1.Secret{})).To(Succeed())

	clientSSLSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, clientTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), clientSSLSecret, &corev1.Secret{})).To(Succeed())

	dockerSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, dockerTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), dockerSecret, &corev1.Secret{})).To(Succeed())
}

func expectResourcesToNotExist(t *testing.T, g *WithT, k8sClient client.Client, nsName types.NamespacedName) {
	t.Helper()
	g.Expect(k8sClient.Get(t.Context(), nsName, &appsv1.Deployment{})).ToNot(Succeed())

	g.Expect(k8sClient.Get(t.Context(), nsName, &corev1.Service{})).ToNot(Succeed())

	g.Expect(k8sClient.Get(t.Context(), nsName, &corev1.ServiceAccount{})).ToNot(Succeed())

	bootstrapCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxIncludesConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), bootstrapCM, &corev1.ConfigMap{})).ToNot(Succeed())

	agentCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxAgentConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), agentCM, &corev1.ConfigMap{})).ToNot(Succeed())

	agentTLSSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, agentTLSTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), agentTLSSecret, &corev1.Secret{})).ToNot(Succeed())

	jwtSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, jwtTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), jwtSecret, &corev1.Secret{})).ToNot(Succeed())

	caSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, caTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), caSecret, &corev1.Secret{})).ToNot(Succeed())

	clientSSLSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, clientTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), clientSSLSecret, &corev1.Secret{})).ToNot(Succeed())

	dockerSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, dockerTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), dockerSecret, &corev1.Secret{})).ToNot(Succeed())
}

func defaultNginxProvisioner(
	objects ...client.Object,
) (*NginxProvisioner, client.Client, *agentfakes.FakeDeploymentStorer) {
	fakeClient := fake.NewClientBuilder().WithScheme(createScheme()).WithObjects(objects...).Build()
	deploymentStore := &agentfakes.FakeDeploymentStorer{}

	return &NginxProvisioner{
		store: newStore(
			[]string{dockerTestSecretName},
			agentTLSTestSecretName,
			jwtTestSecretName,
			caTestSecretName,
			clientTestSecretName,
			nginxOneDataplaneKeySecretName,
		),
		k8sClient: fakeClient,
		cfg: Config{
			DeploymentStore: deploymentStore,
			GatewayPodConfig: &config.GatewayPodConfig{
				InstanceName: "test-instance",
				Namespace:    ngfNamespace,
			},
			Logger:        logr.Discard(),
			EventRecorder: &k8sEvents.FakeRecorder{},
			GCName:        "nginx",
			Plus:          true,
			PlusUsageConfig: &config.UsageReportConfig{
				SecretName:          jwtTestSecretName,
				CASecretName:        caTestSecretName,
				ClientSSLSecretName: clientTestSecretName,
			},
			NginxDockerSecretNames: []string{dockerTestSecretName},
			AgentTLSSecretName:     agentTLSTestSecretName,
			NginxOneConsoleTelemetryConfig: config.NginxOneConsoleTelemetryConfig{
				DataplaneKeySecretName: "dataplane-key",
				EndpointHost:           "agent.connect.nginx.com",
				EndpointPort:           443,
				EndpointTLSSkipVerify:  false,
			},
			AgentLabels: map[string]string{
				"product-type":      "ngf",
				"product-version":   "ngf-version",
				"cluster-id":        "my-cluster-id",
				"control-name":      "my-control-plane-name",
				"control-id":        "my-control-plane-id",
				"control-namespace": "my-control-plane-namespace",
			},
		},
		leader: true,
	}, fakeClient, deploymentStore
}

type fakeLabelCollector struct{}

func (f *fakeLabelCollector) Collect(_ context.Context) (map[string]string, error) {
	return map[string]string{"product-type": "fake"}, nil
}

// failingClient wraps a fake client and can be configured to fail on specific operations.
type failingClient struct {
	client.Client
	failOnCreate bool
	failOnUpdate bool
}

func (f *failingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if f.failOnCreate {
		// Return an IsInvalid error to trigger the specific error logging at line 260
		return apierrors.NewInvalid(schema.GroupKind{Group: "apps", Kind: "Deployment"}, obj.GetName(), field.ErrorList{
			field.Invalid(field.NewPath("spec"), obj, "test invalid error"),
		})
	}
	return f.Client.Create(ctx, obj, opts...)
}

func (f *failingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if f.failOnUpdate {
		return apierrors.NewInvalid(schema.GroupKind{Group: "apps", Kind: "Deployment"}, obj.GetName(), field.ErrorList{
			field.Invalid(field.NewPath("spec"), obj, "test invalid error"),
		})
	}
	return f.Client.Update(ctx, obj, opts...)
}

func (f *failingClient) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	if f.failOnUpdate {
		return apierrors.NewInvalid(schema.GroupKind{Group: "apps", Kind: "Deployment"}, obj.GetName(), field.ErrorList{
			field.Invalid(field.NewPath("spec"), obj, "test invalid error"),
		})
	}
	return f.Client.Patch(ctx, obj, patch, opts...)
}

// lbClassImmutableClient wraps a client.Client and returns a LoadBalancerClass immutability
// error on the first Service Update call, simulating the immutable-field behavior of the real
// Kubernetes API. Subsequent Service Updates are delegated to the underlying client unchanged.
type lbClassImmutableClient struct {
	client.Client
	svcUpdateAttempts int
}

func (c *lbClassImmutableClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*corev1.Service); ok {
		c.svcUpdateAttempts++
		if c.svcUpdateAttempts == 1 {
			return apierrors.NewInvalid(
				schema.GroupKind{Group: "", Kind: "Service"},
				obj.GetName(),
				field.ErrorList{
					field.Invalid(
						field.NewPath("spec").Child("loadBalancerClass"),
						"gateway.nginx.org/nginx-gateway-controller",
						"may not change once set",
					),
				},
			)
		}
	}
	return c.Client.Update(ctx, obj, opts...)
}

// Delete delegates to the underlying client and clears the ResourceVersion on obj so that
// a subsequent CreateOrUpdate can Create the object fresh without a stale ResourceVersion.
func (c *lbClassImmutableClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	err := c.Client.Delete(ctx, obj, opts...)
	obj.SetResourceVersion("")
	return err
}

func TestNewNginxProvisioner(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mgr, err := manager.New(&rest.Config{}, manager.Options{Scheme: createScheme()})
	g.Expect(err).ToNot(HaveOccurred())

	cfg := Config{
		GCName: "test-gc",
		GatewayPodConfig: &config.GatewayPodConfig{
			InstanceName: "test-instance",
		},
		Logger: logr.Discard(),
		NginxOneConsoleTelemetryConfig: config.NginxOneConsoleTelemetryConfig{
			DataplaneKeySecretName: "dataplane-key",
		},
	}

	apiChecker = &openshiftfakes.FakeAPIChecker{}
	labelCollectorFactory = func(_ manager.Manager, _ Config) AgentLabelCollector {
		return &fakeLabelCollector{}
	}

	provisioner, eventLoop, err := NewNginxProvisioner(t.Context(), mgr, cfg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(provisioner).NotTo(BeNil())
	g.Expect(eventLoop).NotTo(BeNil())

	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/managed-by": "test-instance-test-gc",
			"app.kubernetes.io/instance":   "test-instance",
		},
	}
	g.Expect(provisioner.baseLabelSelector).To(Equal(labelSelector))

	g.Expect(provisioner.store.dataplaneKeySecretName).To(Equal("dataplane-key"))
}

func TestEnable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		gatewayInStore bool
		wantDepDeleted bool
	}{
		{
			name:           "gateway is not in the store, data plane resources are deprovisioned on leader promotion",
			gatewayInStore: false,
			wantDepDeleted: true,
		},
		{
			name: "gateway is in the store, " +
				"data plane resources are preserved on leader promotion because the gateway is live",
			gatewayInStore: true,
			wantDepDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gw-nginx",
					Namespace: "default",
				},
			}
			provisioner, fakeClient, _ := defaultNginxProvisioner(dep)
			provisioner.setResourceToDelete(types.NamespacedName{Name: "gw", Namespace: "default"})
			provisioner.leader = false

			if tt.gatewayInStore {
				provisioner.store.updateGateway(&gatewayv1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw",
						Namespace: "default",
					},
				})
			}

			provisioner.Enable(t.Context())
			g.Expect(provisioner.isLeader()).To(BeTrue())
			g.Expect(provisioner.resourcesToDeleteOnStartup).To(BeEmpty())

			err := fakeClient.Get(
				t.Context(),
				types.NamespacedName{Name: "gw-nginx", Namespace: "default"},
				&appsv1.Deployment{},
			)
			if tt.wantDepDeleted {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestRegisterGateway(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{
			{},
		},
		Valid: true,
	}

	objects := []client.Object{
		gateway.Source,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      agentTLSTestSecretName,
				Namespace: ngfNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jwtTestSecretName,
				Namespace: ngfNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caTestSecretName,
				Namespace: ngfNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clientTestSecretName,
				Namespace: ngfNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dockerTestSecretName,
				Namespace: ngfNamespace,
			},
		},
	}

	provisioner, fakeClient, deploymentStore := defaultNginxProvisioner(objects...)

	g.Expect(provisioner.RegisterGateway(t.Context(), gateway, "gw-nginx")).To(Succeed())
	expectResourcesToExist(t, g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, true) // plus

	// Call again, no updates so nothing should happen
	g.Expect(provisioner.RegisterGateway(t.Context(), gateway, "gw-nginx")).To(Succeed())
	expectResourcesToExist(t, g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, true) // plus

	// Now set the Gateway to invalid, and expect a deprovision to occur
	invalid := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Valid: false,
	}
	g.Expect(provisioner.RegisterGateway(t.Context(), invalid, "gw-nginx")).To(Succeed())
	expectResourcesToNotExist(t, g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"})

	resources := provisioner.store.getNginxResourcesForGateway(types.NamespacedName{Name: "gw", Namespace: "default"})
	g.Expect(resources).To(BeNil())

	g.Expect(deploymentStore.RemoveCallCount()).To(Equal(1))
}

func TestRegisterGateway_CreateOrUpdateError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{
			{},
		},
		Valid: true,
	}

	objects := []client.Object{
		gateway.Source,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      agentTLSTestSecretName,
				Namespace: ngfNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jwtTestSecretName,
				Namespace: ngfNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caTestSecretName,
				Namespace: ngfNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clientTestSecretName,
				Namespace: ngfNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dockerTestSecretName,
				Namespace: ngfNamespace,
			},
		},
	}

	provisioner, _, _ := defaultNginxProvisioner(objects...)

	// Replace the fakeClient with one that returns errors on Create operations
	provisioner.k8sClient = &failingClient{
		Client:       createFakeClientWithScheme(objects...),
		failOnCreate: true,
	}

	// Create a context with a short timeout to avoid hanging in the test
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	// This should trigger the error handling code at line 260 in provisioner.go
	// The function should return an error after the timeout
	err := provisioner.RegisterGateway(ctx, gateway, "gw-nginx")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("error provisioning nginx resources"))
}

func TestRegisterGateway_CleansUpOldDeploymentOrDaemonSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Setup: Gateway switches from Deployment to DaemonSet
	gateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{
			{},
		},
		Valid: true,
		EffectiveNginxProxy: &graph.EffectiveNginxProxy{
			Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
				DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{},
			},
		},
	}

	// Create a fake deployment that should be cleaned up
	oldDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-nginx",
			Namespace: "default",
		},
	}
	provisioner, fakeClient, _ := defaultNginxProvisioner(gateway.Source, oldDeployment)
	// Simulate store tracking an old Deployment
	provisioner.store.nginxResources[types.NamespacedName{Name: "gw", Namespace: "default"}] = &NginxResources{
		Deployment: oldDeployment.ObjectMeta,
	}

	// RegisterGateway should clean up the Deployment and create a DaemonSet
	g.Expect(provisioner.RegisterGateway(t.Context(), gateway, "gw-nginx")).To(Succeed())

	// Deployment should be deleted
	err := fakeClient.Get(t.Context(), types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, &appsv1.Deployment{})
	g.Expect(err).To(HaveOccurred())

	// DaemonSet should exist
	err = fakeClient.Get(t.Context(), types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, &appsv1.DaemonSet{})
	g.Expect(err).ToNot(HaveOccurred())

	// Now test the opposite: switch from DaemonSet to Deployment
	gateway.EffectiveNginxProxy = &graph.EffectiveNginxProxy{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Deployment: &ngfAPIv1alpha2.DeploymentSpec{},
		},
	}

	oldDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-nginx",
			Namespace: "default",
		},
	}

	provisioner, fakeClient, _ = defaultNginxProvisioner(gateway.Source, oldDaemonSet)
	provisioner.store.nginxResources[types.NamespacedName{Name: "gw", Namespace: "default"}] = &NginxResources{
		DaemonSet: oldDaemonSet.ObjectMeta,
	}

	g.Expect(provisioner.RegisterGateway(t.Context(), gateway, "gw-nginx")).To(Succeed())

	// DaemonSet should be deleted
	err = fakeClient.Get(t.Context(), types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, &appsv1.DaemonSet{})
	g.Expect(err).To(HaveOccurred())

	// Deployment should exist
	err = fakeClient.Get(t.Context(), types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, &appsv1.Deployment{})
	g.Expect(err).ToNot(HaveOccurred())
}

func TestRegisterGateway_CleansUpOldHPA(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Setup: Gateway previously referenced an HPA, but now does not
	// Previous state: HPA exists and is tracked
	oldHPA := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-nginx",
			Namespace: "default",
		},
	}
	gateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{
			{},
		},
		Valid: true,
		EffectiveNginxProxy: &graph.EffectiveNginxProxy{
			Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
				Deployment: &ngfAPIv1alpha2.DeploymentSpec{
					Autoscaling: &ngfAPIv1alpha2.AutoscalingSpec{
						Enable: false,
					},
				},
			},
		},
	}

	provisioner, fakeClient, _ := defaultNginxProvisioner(gateway.Source, oldHPA)
	provisioner.store.nginxResources[types.NamespacedName{Name: "gw", Namespace: "default"}] = &NginxResources{
		HPA: oldHPA.ObjectMeta,
	}

	// Simulate update: EffectiveNginxProxy no longer references HPA
	g.Expect(provisioner.RegisterGateway(t.Context(), gateway, "gw-nginx")).To(Succeed())

	// HPA should be deleted
	hpaErr := fakeClient.Get(
		t.Context(),
		types.NamespacedName{Name: "gw-nginx", Namespace: "default"},
		&autoscalingv2.HorizontalPodAutoscaler{},
	)
	g.Expect(hpaErr).To(HaveOccurred())
}

func TestRegisterGateway_EmptyListeners(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	gateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{}, // Empty array
		Valid:     true,
	}

	provisioner, fakeClient, _ := defaultNginxProvisioner(gateway.Source)
	g.Expect(provisioner.RegisterGateway(t.Context(), gateway, "gw-nginx")).To(Succeed())

	expectResourcesToNotExist(t, g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"})
}

func TestNonLeaderProvisioner(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	provisioner, fakeClient, deploymentStore := defaultNginxProvisioner()
	provisioner.leader = false
	nsName := types.NamespacedName{Name: "gw-nginx", Namespace: "default"}

	g.Expect(provisioner.RegisterGateway(t.Context(), nil, "gw-nginx")).To(Succeed())
	expectResourcesToNotExist(t, g, fakeClient, nsName)

	g.Expect(provisioner.provisionNginx(t.Context(), "gw-nginx", nil, nil)).To(Succeed())
	expectResourcesToNotExist(t, g, fakeClient, nsName)

	g.Expect(provisioner.reprovisionNginx(t.Context(), "gw-nginx", nil, nil, nil)).To(Succeed())
	expectResourcesToNotExist(t, g, fakeClient, nsName)

	g.Expect(provisioner.deprovisionNginxForInvalidGateway(t.Context(), nsName)).To(Succeed())
	expectResourcesToNotExist(t, g, fakeClient, nsName)
	g.Expect(deploymentStore.RemoveCallCount()).To(Equal(1))
}

func TestProvisionNginxOnLBClassImmutabilityError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Simulate a Service that was previously provisioned without LoadBalancerClass.
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-nginx", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(createScheme()).
		WithObjects(existingSvc).
		Build()

	// Wrap the client so the first Service Update returns an LBClass immutability error.
	wrappedClient := &lbClassImmutableClient{Client: fakeClient}

	provisioner := &NginxProvisioner{
		leader: true,
		store:  newStore(nil, "", "", "", "", ""),
		cfg: Config{
			Logger:           logr.Discard(),
			EventRecorder:    &k8sEvents.FakeRecorder{},
			GatewayPodConfig: &config.GatewayPodConfig{},
		},
		k8sClient: wrappedClient,
	}

	lbClass := "gateway.nginx.org/nginx-gateway-controller"
	desiredSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-nginx", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Type:              corev1.ServiceTypeLoadBalancer,
			LoadBalancerClass: &lbClass,
		},
	}

	// Gateway has no addresses so patchServiceStatus is not triggered.
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "default"},
	}

	err := provisioner.provisionNginx(t.Context(), "gw-nginx", gateway, []client.Object{desiredSvc})
	g.Expect(err).To(HaveOccurred())
	// Should not retry when its a LBClass immutability error
	g.Expect(wrappedClient.svcUpdateAttempts).To(Equal(1))
}

func TestProvisionerRestartsDeployment(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{
			{},
		},
		Valid: true,
		EffectiveNginxProxy: &graph.EffectiveNginxProxy{
			Logging: &ngfAPIv1alpha2.NginxLogging{
				AgentLevel: helpers.GetPointer(ngfAPIv1alpha2.AgentLogLevelDebug),
			},
		},
	}

	// provision everything first
	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{secrets.TLSCertKey: []byte("tls")},
	}
	provisioner, fakeClient, _ := defaultNginxProvisioner(gateway.Source, agentTLSSecret)
	provisioner.cfg.Plus = false
	provisioner.cfg.NginxDockerSecretNames = nil

	g.Expect(provisioner.RegisterGateway(t.Context(), gateway, "gw-nginx")).To(Succeed())
	// not plus
	expectResourcesToExist(t, g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, false)

	// update agent config
	updatedConfig := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{
			{},
		},
		Valid: true,
		EffectiveNginxProxy: &graph.EffectiveNginxProxy{
			Logging: &ngfAPIv1alpha2.NginxLogging{
				AgentLevel: helpers.GetPointer(ngfAPIv1alpha2.AgentLogLevelInfo),
			},
		},
	}
	g.Expect(provisioner.RegisterGateway(t.Context(), updatedConfig, "gw-nginx")).To(Succeed())

	// verify deployment was updated with the restart annotation
	dep := &appsv1.Deployment{}
	key := types.NamespacedName{Name: "gw-nginx", Namespace: "default"}
	g.Expect(fakeClient.Get(t.Context(), key, dep)).To(Succeed())

	g.Expect(dep.Spec.Template.GetAnnotations()).To(HaveKey(controller.RestartedAnnotation))
}

func TestProvisionerRestartsDaemonSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{
			{},
		},
		Valid: true,
		EffectiveNginxProxy: &graph.EffectiveNginxProxy{
			Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
				DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{},
			},
			Logging: &ngfAPIv1alpha2.NginxLogging{
				AgentLevel: helpers.GetPointer(ngfAPIv1alpha2.AgentLogLevelDebug),
			},
		},
	}

	// provision everything first
	agentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{secrets.TLSCertKey: []byte("tls")},
	}
	provisioner, fakeClient, _ := defaultNginxProvisioner(gateway.Source, agentTLSSecret)
	provisioner.cfg.Plus = false
	provisioner.cfg.NginxDockerSecretNames = nil

	key := types.NamespacedName{Name: "gw-nginx", Namespace: "default"}
	g.Expect(provisioner.RegisterGateway(t.Context(), gateway, "gw-nginx")).To(Succeed())
	g.Expect(fakeClient.Get(t.Context(), key, &appsv1.DaemonSet{})).To(Succeed())

	// update agent config
	updatedConfig := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: "default",
			},
		},
		Listeners: []*graph.Listener{
			{},
		},
		Valid: true,
		EffectiveNginxProxy: &graph.EffectiveNginxProxy{
			Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
				DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{},
			},
			Logging: &ngfAPIv1alpha2.NginxLogging{
				AgentLevel: helpers.GetPointer(ngfAPIv1alpha2.AgentLogLevelInfo),
			},
		},
	}
	g.Expect(provisioner.RegisterGateway(t.Context(), updatedConfig, "gw-nginx")).To(Succeed())

	// verify daemonset was updated with the restart annotation
	ds := &appsv1.DaemonSet{}
	g.Expect(fakeClient.Get(t.Context(), key, ds)).To(Succeed())
	g.Expect(ds.Spec.Template.GetAnnotations()).To(HaveKey(controller.RestartedAnnotation))
}

func TestDefaultLabelCollectorFactory(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mgr := &controllerfakes.FakeManager{}

	cfg := Config{
		GatewayPodConfig: &config.GatewayPodConfig{
			Namespace: "pod-namespace",
			Name:      "pod-name",
			Version:   "my-version",
		},
	}

	collector := defaultLabelCollectorFactory(mgr, cfg)
	g.Expect(collector).NotTo(BeNil())
}

func TestCreateMinimalClone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    client.Object
		validate func(*WithT, client.Object)
		name     string
	}{
		{
			name: "creates minimal Deployment",
			input: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"app": "test"},
					Annotations: map[string]string{"version": "1.0"},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: helpers.GetPointer(int32(3)),
				},
			},
			validate: func(g *WithT, obj client.Object) {
				dep, ok := obj.(*appsv1.Deployment)
				g.Expect(ok).To(BeTrue())
				g.Expect(dep.GetName()).To(Equal("test-deployment"))
				g.Expect(dep.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(dep.GetLabels()).To(BeEmpty())
				g.Expect(dep.GetAnnotations()).To(BeEmpty())
				g.Expect(dep.Spec.Replicas).To(BeNil())
			},
		},
		{
			name: "creates minimal DaemonSet",
			input: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-daemonset",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"component": "agent"},
					Annotations: map[string]string{"config": "updated"},
				},
				Spec: appsv1.DaemonSetSpec{
					UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
						Type: appsv1.RollingUpdateDaemonSetStrategyType,
					},
				},
			},
			validate: func(g *WithT, obj client.Object) {
				ds, ok := obj.(*appsv1.DaemonSet)
				g.Expect(ok).To(BeTrue())
				g.Expect(ds.GetName()).To(Equal("test-daemonset"))
				g.Expect(ds.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(ds.GetLabels()).To(BeEmpty())
				g.Expect(ds.GetAnnotations()).To(BeEmpty())
				g.Expect(ds.Spec.UpdateStrategy.Type).To(BeEmpty())
			},
		},
		{
			name: "creates minimal Service",
			input: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-service",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"tier": "frontend"},
					Annotations: map[string]string{"loadbalancer": "enabled"},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
			},
			validate: func(g *WithT, obj client.Object) {
				svc, ok := obj.(*corev1.Service)
				g.Expect(ok).To(BeTrue())
				g.Expect(svc.GetName()).To(Equal("test-service"))
				g.Expect(svc.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(svc.GetLabels()).To(BeEmpty())
				g.Expect(svc.GetAnnotations()).To(BeEmpty())
				g.Expect(svc.Spec.Type).To(BeEmpty())
			},
		},
		{
			name: "creates minimal ServiceAccount",
			input: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-sa",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"role": "service"},
					Annotations: map[string]string{"description": "test service account"},
				},
				AutomountServiceAccountToken: helpers.GetPointer(false),
			},
			validate: func(g *WithT, obj client.Object) {
				sa, ok := obj.(*corev1.ServiceAccount)
				g.Expect(ok).To(BeTrue())
				g.Expect(sa.GetName()).To(Equal("test-sa"))
				g.Expect(sa.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(sa.GetLabels()).To(BeEmpty())
				g.Expect(sa.GetAnnotations()).To(BeEmpty())
				g.Expect(sa.AutomountServiceAccountToken).To(BeNil())
			},
		},
		{
			name: "creates minimal ConfigMap",
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cm",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"config": "nginx"},
					Annotations: map[string]string{"checksum": "abc123"},
				},
				Data: map[string]string{"nginx.conf": "server {}"},
			},
			validate: func(g *WithT, obj client.Object) {
				cm, ok := obj.(*corev1.ConfigMap)
				g.Expect(ok).To(BeTrue())
				g.Expect(cm.GetName()).To(Equal("test-cm"))
				g.Expect(cm.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(cm.GetLabels()).To(BeEmpty())
				g.Expect(cm.GetAnnotations()).To(BeEmpty())
				g.Expect(cm.Data).To(BeEmpty())
			},
		},
		{
			name: "creates minimal Secret",
			input: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-secret",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"type": "tls"},
					Annotations: map[string]string{"cert-manager": "true"},
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{"tls.crt": []byte("cert"), "tls.key": []byte("key")},
			},
			validate: func(g *WithT, obj client.Object) {
				secret, ok := obj.(*corev1.Secret)
				g.Expect(ok).To(BeTrue())
				g.Expect(secret.GetName()).To(Equal("test-secret"))
				g.Expect(secret.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(secret.GetLabels()).To(BeEmpty())
				g.Expect(secret.GetAnnotations()).To(BeEmpty())
				g.Expect(secret.Type).To(BeEmpty())
				g.Expect(secret.Data).To(BeEmpty())
			},
		},
		{
			name: "creates minimal HorizontalPodAutoscaler",
			input: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-hpa",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"autoscaling": "enabled"},
					Annotations: map[string]string{"policy": "conservative"},
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					MinReplicas: helpers.GetPointer(int32(2)),
					MaxReplicas: 10,
				},
			},
			validate: func(g *WithT, obj client.Object) {
				hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
				g.Expect(ok).To(BeTrue())
				g.Expect(hpa.GetName()).To(Equal("test-hpa"))
				g.Expect(hpa.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(hpa.GetLabels()).To(BeEmpty())
				g.Expect(hpa.GetAnnotations()).To(BeEmpty())
				g.Expect(hpa.Spec.MinReplicas).To(BeNil())
				g.Expect(hpa.Spec.MaxReplicas).To(Equal(int32(0)))
			},
		},
		{
			name: "creates minimal Role",
			input: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-role",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"rbac": "enabled"},
					Annotations: map[string]string{"description": "test role"},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				},
			},
			validate: func(g *WithT, obj client.Object) {
				role, ok := obj.(*rbacv1.Role)
				g.Expect(ok).To(BeTrue())
				g.Expect(role.GetName()).To(Equal("test-role"))
				g.Expect(role.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(role.GetLabels()).To(BeEmpty())
				g.Expect(role.GetAnnotations()).To(BeEmpty())
				g.Expect(role.Rules).To(BeEmpty())
			},
		},
		{
			name: "creates minimal RoleBinding",
			input: &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-rolebinding",
					Namespace:   "test-namespace",
					Labels:      map[string]string{"binding": "service"},
					Annotations: map[string]string{"owner": "platform"},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "Role",
					Name:     "test-role",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "test-sa",
						Namespace: "test-namespace",
					},
				},
			},
			validate: func(g *WithT, obj client.Object) {
				rb, ok := obj.(*rbacv1.RoleBinding)
				g.Expect(ok).To(BeTrue())
				g.Expect(rb.GetName()).To(Equal("test-rolebinding"))
				g.Expect(rb.GetNamespace()).To(Equal("test-namespace"))
				g.Expect(rb.GetLabels()).To(BeEmpty())
				g.Expect(rb.GetAnnotations()).To(BeEmpty())
				g.Expect(rb.RoleRef).To(Equal(rbacv1.RoleRef{}))
				g.Expect(rb.Subjects).To(BeEmpty())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := createMinimalClone(tt.input)
			g.Expect(result).ToNot(BeNil())

			// Validate that the result is the same type as input
			g.Expect(result).To(BeAssignableToTypeOf(tt.input))

			// Run specific validations
			tt.validate(g, result)
		})
	}
}

func TestCreateMinimalClone_UnsupportedType(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Test with an unsupported type
	unsupported := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gateway",
			Namespace: "test-namespace",
		},
	}

	g.Expect(func() {
		createMinimalClone(unsupported)
	}).To(Panic())
}

func TestCreateMinimalClone_CreatesSeparateInstances(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "version-test",
			Namespace: "default",
		},
	}

	// First call
	result1 := createMinimalClone(deployment)

	// Second call with same type should use same factory
	result2 := createMinimalClone(deployment)

	// Both results should be the same type and have correct name/namespace
	g.Expect(result1).To(BeAssignableToTypeOf(result2))
	g.Expect(result1.GetName()).To(Equal(result2.GetName()))
	g.Expect(result1.GetNamespace()).To(Equal(result2.GetNamespace()))

	// Verify they are separate object instances, not the same underlying resource
	g.Expect(result1).ToNot(BeIdenticalTo(result2), "createMinimalClone should create separate object instances")

	// Cast to concrete types for more specific validation
	dep1, ok1 := result1.(*appsv1.Deployment)
	dep2, ok2 := result2.(*appsv1.Deployment)
	g.Expect(ok1).To(BeTrue())
	g.Expect(ok2).To(BeTrue())

	// Verify pointer addresses are different (separate objects in memory)
	g.Expect(dep1).ToNot(BeIdenticalTo(dep2), "Deployments should be separate instances with different memory addresses")

	// Verify that modifying one doesn't affect the other
	dep1.SetLabels(map[string]string{"modified": "true"})
	g.Expect(dep2.GetLabels()).To(BeEmpty(), "Modifying one object should not affect the other")

	// Verify factory map contains the expected key
	deploymentType := reflect.TypeOf(deployment)
	_, exists := minimalObjectFactory[deploymentType]
	g.Expect(exists).To(BeTrue(), "Factory should contain entry for Deployment type")
}

func TestProvisionNginxPatchesServiceStatus(t *testing.T) {
	t.Parallel()

	const (
		ctlrName     = "gateway.nginx.org/nginx-gateway-controller"
		instanceName = "test-instance"
		gcName       = "nginx"
		svcName      = "gw-nginx"
		svcNamespace = "default"
	)

	ngfLabels := map[string]string{
		controller.AppInstanceLabel:  instanceName,
		controller.AppManagedByLabel: controller.CreateNginxResourceName(instanceName, gcName),
	}

	tests := []struct {
		name          string
		svcLBClass    *string
		svcType       corev1.ServiceType
		gatewayIPs    []string
		expectIngress []corev1.LoadBalancerIngress
	}{
		{
			name:          "patches status when LBClass matches controller name and IPs present",
			svcLBClass:    helpers.GetPointer(ctlrName),
			svcType:       corev1.ServiceTypeLoadBalancer,
			gatewayIPs:    []string{"10.0.0.1"},
			expectIngress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}},
		},
		{
			name:          "patches status with multiple IPs",
			svcLBClass:    helpers.GetPointer(ctlrName),
			svcType:       corev1.ServiceTypeLoadBalancer,
			gatewayIPs:    []string{"10.0.0.1", "10.0.0.2"},
			expectIngress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}},
		},
		{
			name:          "does not patch when LoadBalancerClass is nil",
			svcLBClass:    nil,
			svcType:       corev1.ServiceTypeLoadBalancer,
			gatewayIPs:    []string{"10.0.0.1"},
			expectIngress: nil,
		},
		{
			name:          "does not patch when LoadBalancerClass does not match controller name",
			svcLBClass:    helpers.GetPointer("other.controller/name"),
			svcType:       corev1.ServiceTypeLoadBalancer,
			gatewayIPs:    []string{"10.0.0.1"},
			expectIngress: nil,
		},
		{
			name:          "does not patch when gateway has no IP-type addresses",
			svcLBClass:    helpers.GetPointer(ctlrName),
			svcType:       corev1.ServiceTypeLoadBalancer,
			gatewayIPs:    []string{},
			expectIngress: nil,
		},
		{
			name:          "does not patch when service is not of LoadBalancer type",
			svcLBClass:    helpers.GetPointer(ctlrName),
			svcType:       corev1.ServiceTypeClusterIP,
			gatewayIPs:    []string{"10.0.0.1"},
			expectIngress: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeClient := fake.NewClientBuilder().
				WithScheme(createScheme()).
				WithStatusSubresource(&corev1.Service{}).
				Build()

			provisioner := &NginxProvisioner{
				leader: true,
				store:  newStore(nil, "", "", "", "", ""),
				cfg: Config{
					Logger:        logr.Discard(),
					EventRecorder: &k8sEvents.FakeRecorder{},
					GatewayPodConfig: &config.GatewayPodConfig{
						InstanceName: instanceName,
					},
					GCName:          gcName,
					GatewayCtlrName: ctlrName,
				},
				k8sClient: fakeClient,
			}

			desiredSvc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      svcName,
					Namespace: svcNamespace,
					Labels:    ngfLabels,
				},
				Spec: corev1.ServiceSpec{
					Type:              test.svcType,
					LoadBalancerClass: test.svcLBClass,
				},
			}

			addrType := gatewayv1.IPAddressType
			gateway := &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: svcNamespace},
			}
			for _, ip := range test.gatewayIPs {
				gateway.Spec.Addresses = append(gateway.Spec.Addresses, gatewayv1.GatewaySpecAddress{
					Type:  &addrType,
					Value: ip,
				})
			}

			g.Expect(provisioner.provisionNginx(t.Context(), svcName, gateway, []client.Object{desiredSvc})).To(Succeed())

			got := &corev1.Service{}
			g.Expect(fakeClient.Get(
				t.Context(),
				types.NamespacedName{Name: svcName, Namespace: svcNamespace},
				got,
			)).To(Succeed())
			g.Expect(got.Status.LoadBalancer.Ingress).To(Equal(test.expectIngress))
		})
	}
}

func TestPatchServiceStatus(t *testing.T) {
	t.Parallel()

	const (
		instanceName = "test-instance"
		gcName       = "nginx"
		svcName      = "gw-nginx"
		svcNamespace = "default"
	)

	ngfLabels := map[string]string{
		controller.AppInstanceLabel:  instanceName,
		controller.AppManagedByLabel: controller.CreateNginxResourceName(instanceName, gcName),
	}

	makeProvisioner := func(k8sClient client.Client) *NginxProvisioner {
		return &NginxProvisioner{
			cfg: Config{
				GatewayPodConfig: &config.GatewayPodConfig{
					InstanceName: instanceName,
					Namespace:    ngfNamespace,
				},
				Logger: logr.Discard(),
				GCName: gcName,
			},
			k8sClient: k8sClient,
		}
	}

	makeSvc := func(labels map[string]string, existingIngress []corev1.LoadBalancerIngress) *corev1.Service {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: svcNamespace,
				Labels:    labels,
			},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
		}
		svc.Status.LoadBalancer.Ingress = existingIngress
		return svc
	}

	tests := []struct {
		svc              *corev1.Service
		interceptorFuncs *interceptor.Funcs
		name             string
		ips              []string
		expectIngress    []corev1.LoadBalancerIngress
		expectErr        bool
		expectNoChange   bool
	}{
		{
			name:          "empty IPs list is a no-op",
			svc:           makeSvc(ngfLabels, nil),
			ips:           []string{},
			expectErr:     false,
			expectIngress: nil,
		},
		{
			name:      "service not found returns error",
			svc:       nil,
			ips:       []string{"10.0.0.1"},
			expectErr: true,
		},
		{
			name: "service with wrong instance label is skipped",
			svc: makeSvc(map[string]string{
				controller.AppInstanceLabel:  "other-instance",
				controller.AppManagedByLabel: controller.CreateNginxResourceName(instanceName, gcName),
			}, nil),
			ips:            []string{"10.0.0.1"},
			expectErr:      false,
			expectNoChange: true,
		},
		{
			name: "service with wrong managed-by label is skipped",
			svc: makeSvc(map[string]string{
				controller.AppInstanceLabel:  instanceName,
				controller.AppManagedByLabel: "some-other-controller",
			}, nil),
			ips:            []string{"10.0.0.1"},
			expectErr:      false,
			expectNoChange: true,
		},
		{
			name: "status already matches is a no-op",
			svc: makeSvc(ngfLabels, []corev1.LoadBalancerIngress{
				{IP: "10.0.0.1"},
			}),
			ips:           []string{"10.0.0.1"},
			expectErr:     false,
			expectIngress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}},
		},
		{
			name:          "sets ingress for single IP",
			svc:           makeSvc(ngfLabels, nil),
			ips:           []string{"10.0.0.1"},
			expectErr:     false,
			expectIngress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}},
		},
		{
			name:          "sets ingress for multiple IPs",
			svc:           makeSvc(ngfLabels, nil),
			ips:           []string{"10.0.0.1", "10.0.0.2"},
			expectErr:     false,
			expectIngress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}},
		},
		{
			name:          "deduplicates duplicate IPs",
			svc:           makeSvc(ngfLabels, nil),
			ips:           []string{"10.0.0.1", "10.0.0.1", "10.0.0.2"},
			expectErr:     false,
			expectIngress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}},
		},
		{
			name:          "filters out empty string IPs",
			svc:           makeSvc(ngfLabels, nil),
			ips:           []string{"", "10.0.0.1", ""},
			expectErr:     false,
			expectIngress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}},
		},
		{
			name: "patch returns non-retryable error",
			svc:  makeSvc(ngfLabels, nil),
			ips:  []string{"10.0.0.1"},
			interceptorFuncs: &interceptor.Funcs{
				SubResourcePatch: func(
					_ context.Context,
					_ client.Client,
					_ string,
					_ client.Object,
					_ client.Patch,
					_ ...client.SubResourcePatchOption,
				) error {
					return errors.New("patch failed")
				},
			},
			expectErr: true,
		},
		{
			name: "patch returns conflict then succeeds",
			svc:  makeSvc(ngfLabels, nil),
			ips:  []string{"10.0.0.1"},
			interceptorFuncs: func() *interceptor.Funcs {
				calls := 0
				return &interceptor.Funcs{
					SubResourcePatch: func(
						ctx context.Context,
						c client.Client,
						subResource string,
						obj client.Object,
						patch client.Patch,
						opts ...client.SubResourcePatchOption,
					) error {
						calls++
						if calls == 1 {
							return apierrors.NewConflict(
								schema.GroupResource{Resource: "services"},
								svcName,
								errors.New("conflict"),
							)
						}
						return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
					},
				}
			}(),
			expectErr:     false,
			expectIngress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var k8sClient client.Client
			builder := fake.NewClientBuilder().WithScheme(createScheme())
			if test.svc != nil {
				builder = builder.WithObjects(test.svc).WithStatusSubresource(test.svc)
			}
			if test.interceptorFuncs != nil {
				builder = builder.WithInterceptorFuncs(*test.interceptorFuncs)
			}
			k8sClient = builder.Build()

			provisioner := makeProvisioner(k8sClient)
			err := provisioner.patchServiceStatus(t.Context(), svcNamespace, svcName, test.ips)

			if test.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			if test.expectNoChange {
				// Verify the status was not touched (still empty)
				got := &corev1.Service{}
				g.Expect(k8sClient.Get(
					t.Context(),
					types.NamespacedName{Name: svcName, Namespace: svcNamespace},
					got,
				)).To(Succeed())
				g.Expect(got.Status.LoadBalancer.Ingress).To(BeNil())
				return
			}

			got := &corev1.Service{}
			g.Expect(k8sClient.Get(
				t.Context(),
				types.NamespacedName{Name: svcName, Namespace: svcNamespace},
				got,
			)).To(Succeed())
			g.Expect(got.Status.LoadBalancer.Ingress).To(Equal(test.expectIngress))
		})
	}
}

func TestIsLoadBalancerClassImmutabilityErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err    error
		name   string
		expect bool
	}{
		{
			name:   "nil error",
			err:    nil,
			expect: false,
		},
		{
			name:   "non-invalid API error has no loadBalancerClass cause",
			err:    apierrors.NewNotFound(schema.GroupResource{Resource: "services"}, "test-svc"),
			expect: false,
		},
		{
			name: "invalid error for a different field",
			err: apierrors.NewInvalid(
				schema.GroupKind{Group: "", Kind: "Service"},
				"test-svc",
				field.ErrorList{
					field.Invalid(field.NewPath("spec").Child("type"), "ClusterIP", "cannot change type"),
				},
			),
			expect: false,
		},
		{
			name: "invalid error for spec.loadBalancerClass",
			err: apierrors.NewInvalid(
				schema.GroupKind{Group: "", Kind: "Service"},
				"test-svc",
				field.ErrorList{
					field.Invalid(
						field.NewPath("spec").Child("loadBalancerClass"),
						"gateway.nginx.org/nginx-gateway-controller",
						"may not change once set",
					),
				},
			),
			expect: true,
		},
		{
			name: "invalid error with multiple causes including spec.loadBalancerClass",
			err: apierrors.NewInvalid(
				schema.GroupKind{Group: "", Kind: "Service"},
				"test-svc",
				field.ErrorList{
					field.Invalid(field.NewPath("spec").Child("type"), "ClusterIP", "cannot change type"),
					field.Invalid(
						field.NewPath("spec").Child("loadBalancerClass"),
						"gateway.nginx.org/nginx-gateway-controller",
						"may not change once set",
					),
				},
			),
			expect: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(isLoadBalancerClassImmutabilityErr(test.err)).To(Equal(test.expect))
		})
	}
}
