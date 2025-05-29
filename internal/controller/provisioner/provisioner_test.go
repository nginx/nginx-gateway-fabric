package provisioner

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/agentfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/provisioner/openshift/openshiftfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
)

const (
	agentTLSTestSecretName = "agent-tls-secret"
	jwtTestSecretName      = "jwt-secret"
	caTestSecretName       = "ca-secret"
	clientTestSecretName   = "client-secret"
	dockerTestSecretName   = "docker-secret"
	ngfNamespace           = "nginx-gateway"
)

func createScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))

	return scheme
}

func expectResourcesToExist(g *WithT, k8sClient client.Client, nsName types.NamespacedName, plus bool) {
	g.Expect(k8sClient.Get(context.TODO(), nsName, &appsv1.Deployment{})).To(Succeed())

	g.Expect(k8sClient.Get(context.TODO(), nsName, &corev1.Service{})).To(Succeed())

	g.Expect(k8sClient.Get(context.TODO(), nsName, &corev1.ServiceAccount{})).To(Succeed())

	boostrapCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxIncludesConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), boostrapCM, &corev1.ConfigMap{})).To(Succeed())

	agentCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxAgentConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), agentCM, &corev1.ConfigMap{})).To(Succeed())

	agentTLSSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, agentTLSTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), agentTLSSecret, &corev1.Secret{})).To(Succeed())

	if !plus {
		return
	}

	jwtSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, jwtTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), jwtSecret, &corev1.Secret{})).To(Succeed())

	caSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, caTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), caSecret, &corev1.Secret{})).To(Succeed())

	clientSSLSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, clientTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), clientSSLSecret, &corev1.Secret{})).To(Succeed())

	dockerSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, dockerTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), dockerSecret, &corev1.Secret{})).To(Succeed())
}

func expectResourcesToNotExist(g *WithT, k8sClient client.Client, nsName types.NamespacedName) {
	g.Expect(k8sClient.Get(context.TODO(), nsName, &appsv1.Deployment{})).ToNot(Succeed())

	g.Expect(k8sClient.Get(context.TODO(), nsName, &corev1.Service{})).ToNot(Succeed())

	g.Expect(k8sClient.Get(context.TODO(), nsName, &corev1.ServiceAccount{})).ToNot(Succeed())

	boostrapCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxIncludesConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), boostrapCM, &corev1.ConfigMap{})).ToNot(Succeed())

	agentCM := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, nginxAgentConfigMapNameSuffix),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), agentCM, &corev1.ConfigMap{})).ToNot(Succeed())

	agentTLSSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, agentTLSTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), agentTLSSecret, &corev1.Secret{})).ToNot(Succeed())

	jwtSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, jwtTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), jwtSecret, &corev1.Secret{})).ToNot(Succeed())

	caSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, caTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), caSecret, &corev1.Secret{})).ToNot(Succeed())

	clientSSLSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, clientTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), clientSSLSecret, &corev1.Secret{})).ToNot(Succeed())

	dockerSecret := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(nsName.Name, dockerTestSecretName),
		Namespace: nsName.Namespace,
	}
	g.Expect(k8sClient.Get(context.TODO(), dockerSecret, &corev1.Secret{})).ToNot(Succeed())
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
		),
		k8sClient: fakeClient,
		cfg: Config{
			DeploymentStore: deploymentStore,
			GatewayPodConfig: &config.GatewayPodConfig{
				InstanceName: "test-instance",
				Namespace:    ngfNamespace,
			},
			Logger:        logr.Discard(),
			EventRecorder: &record.FakeRecorder{},
			GCName:        "nginx",
			Plus:          true,
			PlusUsageConfig: &config.UsageReportConfig{
				SecretName:          jwtTestSecretName,
				CASecretName:        caTestSecretName,
				ClientSSLSecretName: clientTestSecretName,
			},
			NginxDockerSecretNames: []string{dockerTestSecretName},
			AgentTLSSecretName:     agentTLSTestSecretName,
		},
		leader: true,
	}, fakeClient, deploymentStore
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
	}

	apiChecker = &openshiftfakes.FakeAPIChecker{}
	provisioner, eventLoop, err := NewNginxProvisioner(context.TODO(), mgr, cfg)
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

	provisioner.Enable(context.TODO())
	g.Expect(provisioner.isLeader()).To(BeTrue())
	g.Expect(provisioner.resourcesToDeleteOnStartup).To(BeEmpty())
	expectResourcesToNotExist(g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"})
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

	g.Expect(provisioner.RegisterGateway(context.TODO(), gateway, "gw-nginx")).To(Succeed())
	expectResourcesToExist(g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, true) // plus

	// Call again, no updates so nothing should happen
	g.Expect(provisioner.RegisterGateway(context.TODO(), gateway, "gw-nginx")).To(Succeed())
	expectResourcesToExist(g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, true) // plus

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
	g.Expect(provisioner.RegisterGateway(context.TODO(), invalid, "gw-nginx")).To(Succeed())
	expectResourcesToNotExist(g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"})

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

func TestNonLeaderProvisioner(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	provisioner, fakeClient, deploymentStore := defaultNginxProvisioner()
	provisioner.leader = false
	nsName := types.NamespacedName{Name: "gw-nginx", Namespace: "default"}

	g.Expect(provisioner.RegisterGateway(context.TODO(), nil, "gw-nginx")).To(Succeed())
	expectResourcesToNotExist(g, fakeClient, nsName)

	g.Expect(provisioner.provisionNginx(context.TODO(), "gw-nginx", nil, nil)).To(Succeed())
	expectResourcesToNotExist(g, fakeClient, nsName)

	g.Expect(provisioner.reprovisionNginx(context.TODO(), "gw-nginx", nil, nil)).To(Succeed())
	expectResourcesToNotExist(g, fakeClient, nsName)

	g.Expect(provisioner.deprovisionNginx(context.TODO(), nsName)).To(Succeed())
	expectResourcesToNotExist(g, fakeClient, nsName)
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

	g.Expect(provisioner.RegisterGateway(context.TODO(), gateway, "gw-nginx")).To(Succeed())
	expectResourcesToExist(g, fakeClient, types.NamespacedName{Name: "gw-nginx", Namespace: "default"}, false) // not plus

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
	g.Expect(provisioner.RegisterGateway(context.TODO(), updatedConfig, "gw-nginx")).To(Succeed())

	// verify deployment was updated with the restart annotation
	dep := &appsv1.Deployment{}
	key := types.NamespacedName{Name: "gw-nginx", Namespace: "default"}
	g.Expect(fakeClient.Get(context.TODO(), key, dep)).To(Succeed())

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
	g.Expect(provisioner.RegisterGateway(context.TODO(), gateway, "gw-nginx")).To(Succeed())
	g.Expect(fakeClient.Get(context.TODO(), key, &appsv1.DaemonSet{})).To(Succeed())

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
	g.Expect(provisioner.RegisterGateway(context.TODO(), updatedConfig, "gw-nginx")).To(Succeed())

	// verify daemonset was updated with the restart annotation
	ds := &appsv1.DaemonSet{}
	g.Expect(fakeClient.Get(context.TODO(), key, ds)).To(Succeed())
	g.Expect(ds.Spec.Template.GetAnnotations()).To(HaveKey(controller.RestartedAnnotation))
}
