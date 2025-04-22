package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildReferencedNamespaces(t *testing.T) {
	t.Parallel()
	ns1 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns1",
		},
	}

	ns2 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns2",
			Labels: map[string]string{
				"apples": "oranges",
			},
		},
	}

	ns3 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns3",
			Labels: map[string]string{
				"peaches": "bananas",
			},
		},
	}

	clusterNamespaces := map[types.NamespacedName]*v1.Namespace{
		{Name: "ns1"}: ns1,
		{Name: "ns2"}: ns2,
		{Name: "ns3"}: ns3,
	}

	tests := []struct {
		gws           map[types.NamespacedName]*Gateway
		expectedRefNS map[types.NamespacedName]*v1.Namespace
		name          string
	}{
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{
						{
							Name:                      "listener-2",
							Valid:                     true,
							AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"apples": "oranges"}),
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: map[types.NamespacedName]*v1.Namespace{
				{Name: "ns2"}: ns2,
			},
			name: "gateway matches labels with one namespace",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{
						{
							Name:                      "listener-1",
							Valid:                     true,
							AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"apples": "oranges"}),
						},
						{
							Name:                      "listener-2",
							Valid:                     true,
							AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"peaches": "bananas"}),
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: map[types.NamespacedName]*v1.Namespace{
				{Name: "ns2"}: ns2,
				{Name: "ns3"}: ns3,
			},
			name: "gateway matches labels with two namespaces",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{},
					Valid:     true,
				},
			},
			expectedRefNS: nil,
			name:          "gateway has no Listeners",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{
						{
							Name:  "listener-1",
							Valid: true,
						},
						{
							Name:  "listener-2",
							Valid: true,
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: nil,
			name:          "gateway has multiple listeners with no AllowedRouteLabelSelector set",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{
						{
							Name:                      "listener-1",
							Valid:                     true,
							AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"not": "matching"}),
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: nil,
			name:          "gateway doesn't match labels with any namespace",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{
						{
							Name:                      "listener-1",
							Valid:                     true,
							AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"apples": "oranges"}),
						},
						{
							Name:                      "listener-2",
							Valid:                     true,
							AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"not": "matching"}),
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: map[types.NamespacedName]*v1.Namespace{
				{Name: "ns2"}: ns2,
			},
			name: "gateway has two listeners and only matches labels with one namespace",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{
						{
							Name:                      "listener-1",
							Valid:                     true,
							AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"apples": "oranges"}),
						},
						{
							Name:  "listener-2",
							Valid: true,
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: map[types.NamespacedName]*v1.Namespace{
				{Name: "ns2"}: ns2,
			},
			name: "gateway has two listeners, one with a matching AllowedRouteLabelSelector and one without the field set",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(buildReferencedNamespaces(clusterNamespaces, test.gws)).To(Equal(test.expectedRefNS))
		})
	}
}

func TestIsNamespaceReferenced(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ns   *v1.Namespace
		gws  map[types.NamespacedName]*Gateway
		name string
		exp  bool
	}{
		{
			ns:   nil,
			gws:  nil,
			exp:  false,
			name: "namespace and gateway are nil",
		},
		{
			ns: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns1",
				},
			},
			gws:  nil,
			exp:  false,
			name: "namespace is valid but gateway is nil",
		},
		{
			ns: nil,
			gws: map[types.NamespacedName]*Gateway{
				{Name: "ns1"}: {
					Listeners: []*Listener{},
					Valid:     true,
				},
			},
			exp:  false,
			name: "gateway is valid but namespace is nil",
		},
	}

	// Other test cases should be covered by testing of BuildReferencedNamespaces
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(isNamespaceReferenced(test.ns, test.gws)).To(Equal(test.exp))
		})
	}
}
