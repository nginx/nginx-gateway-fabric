package provisioner

import (
	"errors"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

func TestHandleEventBatch_Upsert(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore([]string{dockerTestSecretName}, "", jwtTestSecretName, "", "", "")
	provisioner, fakeClient, _ := defaultNginxProvisioner()
	provisioner.cfg.StatusQueue = status.NewQueue()

	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "nginx"},
	}
	gcName := "nginx"

	handler, err := newEventHandler(store, provisioner, provisioner.k8sClient, labelSelector, gcName)
	g.Expect(err).ToNot(HaveOccurred())

	ctx := t.Context()
	logger := logr.Discard()

	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx"},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gw-nginx",
			Namespace:       "default",
			ResourceVersion: "1",
			Labels:          map[string]string{"app": "nginx", controller.GatewayLabel: "gw"},
		},
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gw-nginx",
			Namespace:       "default",
			ResourceVersion: "1",
			Labels:          map[string]string{"app": "nginx", controller.GatewayLabel: "gw"},
		},
	}

	jwtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gw-nginx-" + jwtTestSecretName,
			Namespace:       "default",
			ResourceVersion: "1",
			Labels:          map[string]string{"app": "nginx", controller.GatewayLabel: "gw"},
		},
		Data: map[string][]byte{
			"data": []byte("oldData"),
		},
	}

	userJwtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jwtTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{
			"data": []byte("oldData"),
		},
	}
	g.Expect(fakeClient.Create(ctx, userJwtSecret)).To(Succeed())

	dockerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gw-nginx-" + dockerTestSecretName,
			Namespace:       "default",
			ResourceVersion: "1",
			Labels:          map[string]string{"app": "nginx", controller.GatewayLabel: "gw"},
		},
		Data: map[string][]byte{
			"data": []byte("oldDockerData"),
		},
	}

	userDockerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dockerTestSecretName,
			Namespace: ngfNamespace,
		},
		Data: map[string][]byte{
			"data": []byte("oldDockerData"),
		},
	}
	g.Expect(fakeClient.Create(ctx, userDockerSecret)).To(Succeed())

	// Test handling Gateway
	upsertEvent := &events.UpsertEvent{Resource: gateway}
	batch := events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(store.getGateway(client.ObjectKeyFromObject(gateway))).To(Equal(gateway))

	store.registerResourceInGatewayConfig(
		client.ObjectKeyFromObject(gateway),
		&graph.Gateway{Source: gateway, Valid: true},
	)

	// Test handling Deployment
	upsertEvent = &events.UpsertEvent{Resource: deployment}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())

	// Test handling Service
	upsertEvent = &events.UpsertEvent{Resource: service}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(provisioner.cfg.StatusQueue.Dequeue(ctx)).ToNot(BeNil())
	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(service), &corev1.Service{})).To(Succeed())

	// Test handling provisioned Secret
	upsertEvent = &events.UpsertEvent{Resource: jwtSecret}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(jwtSecret), &corev1.Secret{})).To(Succeed())

	// Test handling user Plus Secret
	secret := &corev1.Secret{}
	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(jwtSecret), secret)).To(Succeed())
	g.Expect(secret.Data).To(HaveKey("data"))
	g.Expect(secret.Data["data"]).To(Equal([]byte("oldData")))

	userJwtSecret.Data["data"] = []byte("newData")
	g.Expect(fakeClient.Update(ctx, userJwtSecret)).To(Succeed())
	upsertEvent = &events.UpsertEvent{Resource: userJwtSecret}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(jwtSecret), secret)).To(Succeed())
	g.Expect(secret.Data).To(HaveKey("data"))
	g.Expect(secret.Data["data"]).To(Equal([]byte("newData")))

	// Test handling user Docker Secret
	upsertEvent = &events.UpsertEvent{Resource: dockerSecret}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(dockerSecret), secret)).To(Succeed())
	g.Expect(secret.Data).To(HaveKey("data"))
	g.Expect(secret.Data["data"]).To(Equal([]byte("oldDockerData")))

	userDockerSecret.Data["data"] = []byte("newDockerData")
	g.Expect(fakeClient.Update(ctx, userDockerSecret)).To(Succeed())
	upsertEvent = &events.UpsertEvent{Resource: userDockerSecret}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(dockerSecret), secret)).To(Succeed())
	g.Expect(secret.Data).To(HaveKey("data"))
	g.Expect(secret.Data["data"]).To(Equal([]byte("newDockerData")))

	// remove Gateway from store and verify that Deployment UpsertEvent results in deletion of resource
	store.deleteGateway(client.ObjectKeyFromObject(gateway))
	g.Expect(store.getGateway(client.ObjectKeyFromObject(gateway))).To(BeNil())

	upsertEvent = &events.UpsertEvent{Resource: deployment}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	// do the same thing but when provisioner is not leader.
	// non-leader should not delete resources, but instead track them
	deployment.ResourceVersion = ""
	deploymentNonLeader := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-nginx-non-leader",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx", controller.GatewayLabel: "gw"},
		},
	}
	g.Expect(fakeClient.Create(ctx, deploymentNonLeader)).To(Succeed())
	provisioner.leader = false

	upsertEvent = &events.UpsertEvent{Resource: deploymentNonLeader}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(provisioner.resourcesToDeleteOnStartup).To(HaveLen(1))
	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploymentNonLeader), &appsv1.Deployment{})).To(Succeed())
}

func TestHandleEventBatch_Delete(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore(
		[]string{dockerTestSecretName},
		agentTLSTestSecretName,
		jwtTestSecretName,
		caTestSecretName,
		clientTestSecretName,
		nginxOneDataplaneKeySecretName,
	)
	provisioner, fakeClient, _ := defaultNginxProvisioner()
	provisioner.cfg.StatusQueue = status.NewQueue()

	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "nginx"},
	}
	gcName := "nginx"

	handler, err := newEventHandler(store, provisioner, provisioner.k8sClient, labelSelector, gcName)
	g.Expect(err).ToNot(HaveOccurred())

	ctx := t.Context()
	logger := logr.Discard()

	// initialize resources
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx"},
		},
	}

	store.registerResourceInGatewayConfig(
		client.ObjectKeyFromObject(gateway),
		&graph.Gateway{Source: gateway, Valid: true},
	)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-nginx",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx", controller.GatewayLabel: "gw"},
		},
	}

	originalAgentTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentTLSTestSecretName,
			Namespace: ngfNamespace,
		},
	}
	g.Expect(fakeClient.Create(ctx, originalAgentTLSSecret)).To(Succeed())

	jwtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gw-nginx-" + jwtTestSecretName,
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx", controller.GatewayLabel: "gw"},
		},
	}

	userJwtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jwtTestSecretName,
			Namespace: ngfNamespace,
		},
	}
	g.Expect(fakeClient.Create(ctx, userJwtSecret)).To(Succeed())

	userCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caTestSecretName,
			Namespace: ngfNamespace,
		},
	}
	g.Expect(fakeClient.Create(ctx, userCASecret)).To(Succeed())

	userClientSSLSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clientTestSecretName,
			Namespace: ngfNamespace,
		},
	}
	g.Expect(fakeClient.Create(ctx, userClientSSLSecret)).To(Succeed())

	userDockerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dockerTestSecretName,
			Namespace: ngfNamespace,
		},
	}
	g.Expect(fakeClient.Create(ctx, userDockerSecret)).To(Succeed())

	userDataplaneKeySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginxOneDataplaneKeySecretName,
			Namespace: ngfNamespace,
		},
	}
	g.Expect(fakeClient.Create(ctx, userDataplaneKeySecret)).To(Succeed())

	upsertEvent := &events.UpsertEvent{Resource: gateway}
	batch := events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)
	store.registerResourceInGatewayConfig(client.ObjectKeyFromObject(gateway), deployment)

	// if deployment is deleted, it should be re-created since Gateway still exists
	deleteEvent := &events.DeleteEvent{Type: deployment, NamespacedName: client.ObjectKeyFromObject(deployment)}
	batch = events.EventBatch{deleteEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())

	// if provisioned secret is deleted, it should be re-created
	deleteEvent = &events.DeleteEvent{Type: jwtSecret, NamespacedName: client.ObjectKeyFromObject(jwtSecret)}
	batch = events.EventBatch{deleteEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(jwtSecret), &corev1.Secret{})).To(Succeed())

	// if user-provided secrets are deleted, then delete the duplicates of them
	verifySecret := func(name string, userSecret *corev1.Secret) {
		key := types.NamespacedName{
			Name:      "gw-nginx-" + name,
			Namespace: "default",
		}

		secret := &corev1.Secret{}
		g.Expect(fakeClient.Get(ctx, key, secret)).To(Succeed())
		store.registerResourceInGatewayConfig(client.ObjectKeyFromObject(gateway), secret)

		g.Expect(fakeClient.Delete(ctx, userSecret)).To(Succeed())
		deleteEvent = &events.DeleteEvent{Type: userSecret, NamespacedName: client.ObjectKeyFromObject(userSecret)}
		batch = events.EventBatch{deleteEvent}
		handler.HandleEventBatch(ctx, logger, batch)

		g.Expect(fakeClient.Get(ctx, key, &corev1.Secret{})).ToNot(Succeed())
	}

	verifySecret(agentTLSTestSecretName, originalAgentTLSSecret)
	verifySecret(jwtTestSecretName, userJwtSecret)
	verifySecret(caTestSecretName, userCASecret)
	verifySecret(clientTestSecretName, userClientSSLSecret)
	verifySecret(dockerTestSecretName, userDockerSecret)
	verifySecret(nginxOneDataplaneKeySecretName, userDataplaneKeySecret)

	// delete Gateway when provisioner is not leader
	provisioner.leader = false

	deleteEvent = &events.DeleteEvent{Type: gateway, NamespacedName: client.ObjectKeyFromObject(gateway)}
	batch = events.EventBatch{deleteEvent}
	handler.HandleEventBatch(ctx, logger, batch)
	g.Expect(handler.store.isGatewayDeleting(client.ObjectKeyFromObject(gateway))).To(BeTrue())

	g.Expect(provisioner.resourcesToDeleteOnStartup).To(Equal([]types.NamespacedName{
		{
			Namespace: "default",
			Name:      "gw",
		},
	}))
	g.Expect(store.getGateway(client.ObjectKeyFromObject(gateway))).To(BeNil())
	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())

	// delete Gateway when provisioner is leader
	provisioner.leader = true

	deleteEvent = &events.DeleteEvent{Type: gateway, NamespacedName: client.ObjectKeyFromObject(gateway)}
	batch = events.EventBatch{deleteEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(store.getGateway(client.ObjectKeyFromObject(gateway))).To(BeNil())
}

func TestEventHandler_GetResourceVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		k8sGetError          error
		name                 string
		storeResourceVersion string
		k8sResourceVersion   string
		expectedResult       string
		deepCopySucceeds     bool
	}{
		{
			name:                 "resource version found in store",
			storeResourceVersion: "42",
			k8sResourceVersion:   "100", // should be ignored
			deepCopySucceeds:     true,
			expectedResult:       "42",
		},
		{
			name:                 "resource version not in store, found via k8s client",
			storeResourceVersion: "",
			k8sResourceVersion:   "100",
			deepCopySucceeds:     true,
			expectedResult:       "100",
		},
		{
			name:                 "resource version not in store, k8s client fails",
			storeResourceVersion: "",
			k8sResourceVersion:   "100", // should be ignored due to error
			k8sGetError:          errors.New("not found"),
			deepCopySucceeds:     true,
			expectedResult:       "",
		},
		{
			name:                 "resource version not in store, deepCopy fails",
			storeResourceVersion: "",
			k8sResourceVersion:   "100", // should be ignored due to deepCopy failure
			deepCopySucceeds:     false,
			expectedResult:       "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			// Create test objects
			gatewayNSName := types.NamespacedName{Name: "test-gateway", Namespace: "default"}

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-deployment",
					Namespace:       "default",
					ResourceVersion: test.k8sResourceVersion,
				},
			}

			// Create store with mock behavior
			store := newStore(nil, "", "", "", "", "")
			if test.storeResourceVersion != "" {
				// Set up the store to return the expected resource version
				store.nginxResources[gatewayNSName] = &NginxResources{
					Deployment: metav1.ObjectMeta{
						Name:            "test-deployment",
						Namespace:       "default",
						ResourceVersion: test.storeResourceVersion,
					},
				}
			}

			// Create fake k8s client
			var fakeClient client.Client
			if test.k8sGetError == nil {
				fakeClient = fake.NewClientBuilder().
					WithScheme(createScheme()).
					WithObjects(deployment).
					Build()
			} else {
				// Empty client that will return errors
				fakeClient = fake.NewClientBuilder().
					WithScheme(createScheme()).
					Build()
			}

			// Create test object that may fail deepCopy
			var testObj client.Object
			if test.deepCopySucceeds {
				testObj = deployment
			} else {
				// Create a mock object that fails deepCopy type assertion
				testObj = &mockObject{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "default",
					},
				}
			}

			// Create handler
			provisioner, _, _ := defaultNginxProvisioner()
			labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}}
			handler, err := newEventHandler(store, provisioner, fakeClient, labelSelector, "nginx")
			g.Expect(err).ToNot(HaveOccurred())

			result := handler.getResourceVersion(t.Context(), provisioner.cfg.Logger, gatewayNSName, testObj)
			g.Expect(result).To(Equal(test.expectedResult))
		})
	}
}

// mockObject is a test helper that fails the client.Object type assertion in deepCopy.
type mockObject struct {
	metav1.ObjectMeta
}

func (m *mockObject) GetObjectKind() schema.ObjectKind { return &mockObjectKind{} }
func (m *mockObject) DeepCopyObject() runtime.Object   { return &nonClientObject{} }

// nonClientObject is returned by mockObject.DeepCopyObject() and doesn't implement client.Object.
type nonClientObject struct{}

func (n *nonClientObject) GetObjectKind() schema.ObjectKind { return &mockObjectKind{} }
func (n *nonClientObject) DeepCopyObject() runtime.Object   { return &nonClientObject{} }

// mockObjectKind implements schema.ObjectKind for testing.
type mockObjectKind struct{}

func (m *mockObjectKind) SetGroupVersionKind(_ schema.GroupVersionKind) {}
func (m *mockObjectKind) GroupVersionKind() schema.GroupVersionKind     { return schema.GroupVersionKind{} }
