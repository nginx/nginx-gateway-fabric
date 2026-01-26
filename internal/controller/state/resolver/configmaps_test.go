package resolver_test

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
)

func TestResolve(t *testing.T) {
	resources := map[resolver.ResourceKey]client.Object{
		{
			ResourceType:   resolver.ResourceTypeConfigMap,
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "configmap1"},
		}: &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap1",
				Namespace: "test",
			},
			Data: map[string]string{
				secrets.CAKey: caBlock,
			},
		},
		{
			ResourceType:   resolver.ResourceTypeConfigMap,
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "configmap2"},
		}: &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap2",
				Namespace: "test",
			},
			BinaryData: map[string][]byte{
				secrets.CAKey: []byte(caBlock),
			},
		},
		{
			ResourceType:   resolver.ResourceTypeConfigMap,
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "invalid"},
		}: &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid",
				Namespace: "test",
			},
			Data: map[string]string{
				secrets.CAKey: "invalid",
			},
		},
		{
			ResourceType:   resolver.ResourceTypeConfigMap,
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "nocaentry"},
		}: &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nocaentry",
				Namespace: "test",
			},
			Data: map[string]string{
				"noca.crt": "something else",
			},
		},
	}

	resourceResolver := resolver.NewResourceResolver(resources)

	tests := []struct {
		name          string
		nsname        types.NamespacedName
		errorExpected bool
	}{
		{
			name:          "valid configmap1",
			nsname:        types.NamespacedName{Namespace: "test", Name: "configmap1"},
			errorExpected: false,
		},
		{
			name:          "valid configmap2",
			nsname:        types.NamespacedName{Namespace: "test", Name: "configmap2"},
			errorExpected: false,
		},
		{
			name:          "invalid configmap",
			nsname:        types.NamespacedName{Namespace: "test", Name: "invalid"},
			errorExpected: true,
		},
		{
			name:          "non-existent configmap",
			nsname:        types.NamespacedName{Namespace: "test", Name: "non-existent"},
			errorExpected: true,
		},
		{
			name:          "configmap missing ca entry",
			nsname:        types.NamespacedName{Namespace: "test", Name: "nocaentry"},
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			err := resourceResolver.Resolve(resolver.ResourceTypeConfigMap, test.nsname)
			if test.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
