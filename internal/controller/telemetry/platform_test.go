package telemetry

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPlatform(t *testing.T) {
	t.Parallel()
	tests := []struct {
		node             *v1.Node
		namespaces       *v1.NamespaceList
		expectedPlatform string
		name             string
	}{
		{
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "kind://docker/kind/kind-control-plane",
				},
			},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "kind",
			name:             "kind platform",
		},
		{
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "k3s://ip-172-16-0-210",
				},
			},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "k3s",
			name:             "k3s platform",
		},
		{
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "gce://test-data/us-central1-c/test-data",
				},
			},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "gke",
			name:             "gke platform",
		},
		{
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "azure://test-data/us-central1-c/test-data",
				},
			},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "aks",
			name:             "aks platform",
		},
		{
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "aws://test-data/us-central1-c/test-data",
				},
			},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "eks",
			name:             "eks platform",
		},
		{
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "k3s://ip-172-16-0-210",
				},
			},
			namespaces: &v1.NamespaceList{
				Items: []v1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "cattle-system",
						},
					},
				},
			},
			expectedPlatform: "rancher",
			name:             "rancher platform on k3s",
		},
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"node.openshift.io/os_id": "test"},
				},
				Spec: v1.NodeSpec{
					ProviderID: "k3s://ip-172-16-0-210",
				},
			},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "openshift",
			name:             "openshift platform on k3s",
		},
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"node.openshift.io/os_id": "test"},
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws://test-data/us-central1-c/test-data",
				},
			},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "openshift",
			name:             "openshift platform on aws",
		},
		{
			node: &v1.Node{
				Spec: v1.NodeSpec{
					ProviderID: "different-platform://ip-172-16-0-210",
				},
			},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "other_different-platform",
			name:             "other platform",
		},
		{
			node:             &v1.Node{},
			namespaces:       &v1.NamespaceList{},
			expectedPlatform: "other",
			name:             "missing providerID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			platform := getPlatform(*test.node, *test.namespaces)
			g.Expect(platform).To(Equal(test.expectedPlatform))
		})
	}
}
