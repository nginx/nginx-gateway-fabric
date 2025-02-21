package provisioner

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/status"
)

func TestHandleEventBatch_Upsert(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore(nil, "", "", "")
	provisioner, fakeClient, _ := defaultNginxProvisioner()
	provisioner.cfg.StatusQueue = status.NewQueue()
	provisioner.cfg.Plus = false
	provisioner.cfg.NginxDockerSecretNames = nil

	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "nginx"},
	}
	gcName := "nginx"

	handler, err := newEventHandler(store, provisioner, labelSelector, gcName)
	g.Expect(err).ToNot(HaveOccurred())

	ctx := context.TODO()
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
			Name:      "gw-nginx",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx", controller.GatewayLabel: "gw"},
		},
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx", controller.GatewayLabel: "test-gateway"},
		},
	}

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

	// remove Gateway from store and verify that Deployment UpsertEvent results in deletion of resource
	store.deleteGateway(client.ObjectKeyFromObject(gateway))
	g.Expect(store.getGateway(client.ObjectKeyFromObject(gateway))).To(BeNil())

	upsertEvent = &events.UpsertEvent{Resource: deployment}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).ToNot(Succeed())

	// do the same thing but when provisioner is not leader.
	// non-leader should not delete resources, but instead track them
	g.Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
	provisioner.leader = false

	upsertEvent = &events.UpsertEvent{Resource: deployment}
	batch = events.EventBatch{upsertEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(provisioner.resourcesToDeleteOnStartup).To(HaveLen(1))
	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())
}

func TestHandleEventBatch_Delete(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := newStore(nil, "", "", "")
	provisioner, fakeClient, _ := defaultNginxProvisioner()
	provisioner.cfg.StatusQueue = status.NewQueue()
	provisioner.cfg.Plus = false
	provisioner.cfg.NginxDockerSecretNames = nil

	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "nginx"},
	}
	gcName := "nginx"

	handler, err := newEventHandler(store, provisioner, labelSelector, gcName)
	g.Expect(err).ToNot(HaveOccurred())

	ctx := context.TODO()
	logger := logr.Discard()

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

	store.registerResourceInGatewayConfig(client.ObjectKeyFromObject(gateway), deployment)

	// if deployment is deleted, it should be re-created since Gateway still exists
	deleteEvent := &events.DeleteEvent{Type: deployment, NamespacedName: client.ObjectKeyFromObject(deployment)}
	batch := events.EventBatch{deleteEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())

	// delete Gateway
	deleteEvent = &events.DeleteEvent{Type: gateway, NamespacedName: client.ObjectKeyFromObject(gateway)}
	batch = events.EventBatch{deleteEvent}
	handler.HandleEventBatch(ctx, logger, batch)

	g.Expect(store.getGateway(client.ObjectKeyFromObject(gateway))).To(BeNil())
	g.Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).ToNot(Succeed())
}
