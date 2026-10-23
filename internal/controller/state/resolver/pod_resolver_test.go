package resolver_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
)

const podResolverNs = "test"

func createResolverPod(name, ip string, ready bool, labels map[string]string) *v1.Pod {
	status := v1.PodStatus{
		PodIP: ip,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodReady,
				Status: v1.ConditionFalse,
			},
		},
	}

	if ready {
		status.Conditions[0].Status = v1.ConditionTrue
	}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: podResolverNs,
			Name:      name,
			Labels:    labels,
		},
		Status: status,
	}
}

func newPodResolver(pods ...*v1.Pod) (resolver.PodResolver, *resolver.PodOwnership) {
	ownership := resolver.NewPodOwnership()

	// The fake client stamps the resourceVersion of the objects it is built with, and the subtests
	// run in parallel and share the pods of a test table, so each client needs its own copies.
	objects := make([]client.Object, 0, len(pods))
	for _, pod := range pods {
		objects = append(objects, pod.DeepCopy())
	}

	fakeK8sClient := fake.NewClientBuilder().
		WithObjects(objects...).
		Build()

	return resolver.NewPodResolverImpl(fakeK8sClient, ownership), ownership
}

func TestPodResolverResolve(t *testing.T) {
	t.Parallel()

	appLabel := map[string]string{"app": "app1"}
	otherLabel := map[string]string{"app": "app2"}

	readyPod1 := createResolverPod("pod-1", "1.2.3.4", true, appLabel)
	readyPod2 := createResolverPod("pod-2", "1.2.3.5", true, appLabel)
	readyIPv6Pod := createResolverPod("pod-3", "2001:db8::1", true, appLabel)
	notReadyPod := createResolverPod("pod-4", "1.2.3.6", false, appLabel)
	noIPPod := createResolverPod("pod-5", "", true, appLabel)
	otherPod := createResolverPod("pod-6", "1.2.3.7", true, otherLabel)
	otherNamespacePod := createResolverPod("pod-7", "1.2.3.8", true, appLabel)
	otherNamespacePod.Namespace = "other"

	xbackendNsName := types.NamespacedName{Namespace: podResolverNs, Name: "xbackend"}

	tests := []struct {
		name            string
		pods            []*v1.Pod
		selector        map[string]string
		expectedError   string
		expectedAddress []resolver.Endpoint
		port            int32
	}{
		{
			name:     "ready pods with an IP address become endpoints, sorted by address",
			pods:     []*v1.Pod{readyPod2, readyPod1, otherPod, otherNamespacePod},
			selector: appLabel,
			port:     8080,
			expectedAddress: []resolver.Endpoint{
				{Address: "1.2.3.4", Port: 8080},
				{Address: "1.2.3.5", Port: 8080},
			},
		},
		{
			name:     "an IPv6 pod is reported as an IPv6 endpoint",
			pods:     []*v1.Pod{readyIPv6Pod},
			selector: appLabel,
			port:     8080,
			expectedAddress: []resolver.Endpoint{
				{Address: "2001:db8::1", Port: 8080, IPv6: true},
			},
		},
		{
			name:     "pods that are not ready or have no IP address are excluded",
			pods:     []*v1.Pod{readyPod1, notReadyPod, noIPPod},
			selector: appLabel,
			port:     8081,
			expectedAddress: []resolver.Endpoint{
				{Address: "1.2.3.4", Port: 8081},
			},
		},
		{
			name:          "a selector that matches no pod is an error",
			pods:          []*v1.Pod{otherPod},
			selector:      appLabel,
			port:          8080,
			expectedError: "no valid endpoints found for XBackend test/xbackend and port 8080",
		},
		{
			name:          "a pod that is not ready leaves the XBackend with no endpoints",
			pods:          []*v1.Pod{notReadyPod},
			selector:      appLabel,
			port:          8080,
			expectedError: "no valid endpoints found for XBackend test/xbackend and port 8080",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			podResolver, _ := newPodResolver(test.pods...)

			endpoints, err := podResolver.Resolve(
				context.Background(),
				logr.Discard(),
				xbackendNsName,
				test.selector,
				test.port,
			)

			if test.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(test.expectedError))
				g.Expect(endpoints).To(BeNil())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(endpoints).To(Equal(test.expectedAddress))
		})
	}
}

func TestPodResolverResolveRecordsOwnership(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	appLabel := map[string]string{"app": "app1"}
	xbackendNsName := types.NamespacedName{Namespace: podResolverNs, Name: "xbackend"}

	// The pod is selected by the XBackend but has no IP address, so it does not become an endpoint. It
	// must still be attributed to the XBackend so that its deletion can be attributed later.
	selectedPod := createResolverPod("pod-1", "", true, appLabel)
	unselectedPod := createResolverPod("pod-2", "1.2.3.4", true, map[string]string{"app": "app2"})

	podResolver, ownership := newPodResolver(selectedPod, unselectedPod)

	_, err := podResolver.Resolve(
		context.Background(),
		logr.Discard(),
		xbackendNsName,
		appLabel,
		8080,
	)
	g.Expect(err).To(HaveOccurred())

	g.Expect(ownership.Owners(client.ObjectKeyFromObject(selectedPod))).
		To(Equal([]types.NamespacedName{xbackendNsName}))
	g.Expect(ownership.Owners(client.ObjectKeyFromObject(unselectedPod))).To(BeNil())
}

func TestPodResolverResolvePanics(t *testing.T) {
	t.Parallel()

	appLabel := map[string]string{"app": "app1"}
	validNsName := types.NamespacedName{Namespace: podResolverNs, Name: "xbackend"}

	podResolver, _ := newPodResolver(createResolverPod("pod-1", "1.2.3.4", true, appLabel))

	invalidCalls := []struct {
		selector    map[string]string
		xbackend    types.NamespacedName
		expectPanic string
		name        string
		port        int32
	}{
		{
			name:        "empty namespace",
			xbackend:    types.NamespacedName{Name: "xbackend"},
			selector:    appLabel,
			port:        8080,
			expectPanic: "expected the following fields to be non-empty: name: xbackend, ns: , port: 8080",
		},
		{
			name:        "zero port",
			xbackend:    validNsName,
			selector:    appLabel,
			port:        0,
			expectPanic: "expected the following fields to be non-empty: name: xbackend, ns: test, port: 0",
		},
		{
			name:        "empty selector",
			xbackend:    validNsName,
			selector:    nil,
			port:        8080,
			expectPanic: "expected a non-empty endpoint selector for XBackend test/xbackend",
		},
	}

	for _, invalid := range invalidCalls {
		t.Run(invalid.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(func() {
				_, err := podResolver.Resolve(
					context.Background(),
					logr.Discard(),
					invalid.xbackend,
					invalid.selector,
					invalid.port,
				)
				g.Expect(err).ToNot(HaveOccurred())
			}).To(PanicWith(fmt.Errorf("%s", invalid.expectPanic)))
		})
	}
}
