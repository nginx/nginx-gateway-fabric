package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
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
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{},
					ListenerNamespaces: &gatewayv1.ListenerNamespaces{
						From: helpers.GetPointer(gatewayv1.NamespacesFromSelector),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"apples": "oranges"},
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: map[types.NamespacedName]*v1.Namespace{
				{Name: "ns2"}: ns2,
			},
			name: "gateway with ListenerNamespaces selector matches namespace",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{},
					ListenerNamespaces: &gatewayv1.ListenerNamespaces{
						From: helpers.GetPointer(gatewayv1.NamespacesFromSelector),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"not": "matching"},
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: nil,
			name:          "gateway with ListenerNamespaces selector doesn't match any namespace",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{},
					ListenerNamespaces: &gatewayv1.ListenerNamespaces{
						From:     helpers.GetPointer(gatewayv1.NamespacesFromSelector),
						Selector: nil, // No selector set
					},
					Valid: true,
				},
			},
			expectedRefNS: nil,
			name:          "gateway with ListenerNamespaces From=Selector but no selector set",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{},
					ListenerNamespaces: &gatewayv1.ListenerNamespaces{
						From: helpers.GetPointer(gatewayv1.NamespacesFromSame),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"apples": "oranges"},
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: nil,
			name:          "gateway with ListenerNamespaces From=Same (not Selector) ignores selector",
		},
		{
			gws: map[types.NamespacedName]*Gateway{
				{}: {
					Listeners: []*Listener{},
					ListenerNamespaces: &gatewayv1.ListenerNamespaces{
						From: nil, // From not set
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"apples": "oranges"},
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: nil,
			name:          "gateway with ListenerNamespaces From not set ignores selector",
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
					},
					ListenerNamespaces: &gatewayv1.ListenerNamespaces{
						From: helpers.GetPointer(gatewayv1.NamespacesFromSelector),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"peaches": "bananas"},
						},
					},
					Valid: true,
				},
			},
			expectedRefNS: map[types.NamespacedName]*v1.Namespace{
				{Name: "ns2"}: ns2,
				{Name: "ns3"}: ns3,
			},
			name: "gateway with both AllowedRouteLabelSelector and ListenerNamespaces selector matching different namespaces",
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
