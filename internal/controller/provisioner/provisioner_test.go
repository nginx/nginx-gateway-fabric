package provisioner

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	k8sEvents "k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/agentfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/provisioner/openshift/openshiftfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
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

	return scheme
}

func expectResourcesToExist(t *testing.T, g *WithT, k8sClient client.Client, nsName types.NamespacedName, plus bool) {
	t.Helper()
	g.Expect(k8sClient.Get(t.Context(), nsName, &appsv1.Deployment{})).To(Succeed())

	g.Expect(k8sClient.Get(t.Context(), nsName, &corev1.Service{})).To(Succeed())

	g.Expect(k8sClient.Get(t.Context(), nsName, &corev1.ServiceAccount{})).To(Succeed())

	boostrapCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxIncludesConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), boostrapCM, &corev1.ConfigMap{})).To(Succeed())

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

	boostrapCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxIncludesConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(t.Context(), boostrapCM, &corev1.ConfigMap{})).ToNot(Succeed())

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

	provisioner.Enable(t.Context())
	g.Expect(provisioner.isLeader()).To(BeTrue())
	g.Expect(provisioner.resourcesToDeleteOnStartup).To(BeEmpty())
	expectResourcesToNotExist(t, g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"})
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

	g.Expect(provisioner.reprovisionNginx(t.Context(), "gw-nginx", nil, nil)).To(Succeed())
	expectResourcesToNotExist(t, g, fakeClient, nsName)

	g.Expect(provisioner.deprovisionNginx(t.Context(), nsName)).To(Succeed())
	expectResourcesToNotExist(t, g, fakeClient, nsName)
	g.Expect(deploymentStore.RemoveCallCount()).To(Equal(1))
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
		Data: map[string][]byte{"tls.crt": []byte("tls")},
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
		Data: map[string][]byte{"tls.crt": []byte("tls")},
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
