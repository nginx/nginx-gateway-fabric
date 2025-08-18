package graph

import (
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestNGFPolicyAncestorsFull(t *testing.T) {
	t.Parallel()
	type ancestorConfig struct {
		numCurrNGFAncestors    int
		numCurrNonNGFAncestors int
		numNewNGFAncestors     int
	}

	createPolicy := func(cfg ancestorConfig) *Policy {
		currAncestors := make([]v1alpha2.PolicyAncestorStatus, 0, cfg.numCurrNGFAncestors+cfg.numCurrNonNGFAncestors)
		ngfAncestors := make([]PolicyAncestor, 0, cfg.numNewNGFAncestors)

		for range cfg.numCurrNonNGFAncestors {
			currAncestors = append(currAncestors, v1alpha2.PolicyAncestorStatus{
				ControllerName: "non-ngf",
			})
		}

		for range cfg.numCurrNGFAncestors {
			currAncestors = append(currAncestors, v1alpha2.PolicyAncestorStatus{
				ControllerName: "nginx-gateway",
			})
		}

		for range cfg.numNewNGFAncestors {
			ngfAncestors = append(ngfAncestors, PolicyAncestor{
				Ancestor: v1.ParentReference{},
			})
		}

		return &Policy{
			Source: &ngfAPIv1alpha2.ObservabilityPolicy{
				Status: v1alpha2.PolicyStatus{
					Ancestors: currAncestors,
				},
			},
			Ancestors: ngfAncestors,
		}
	}

	tests := []struct {
		name    string
		expFull bool
		cfg     ancestorConfig
	}{
		{
			name: "current policy not full, no new NGF ancestors have been built yet",
			cfg: ancestorConfig{
				numCurrNGFAncestors:    3,
				numCurrNonNGFAncestors: 12,
				numNewNGFAncestors:     0,
			},
			expFull: false,
		},
		{
			name: "current policy not full, and some new NGF ancestors have been built (not at max)",
			cfg: ancestorConfig{
				numCurrNGFAncestors:    3,
				numCurrNonNGFAncestors: 11,
				numNewNGFAncestors:     2,
			},
			expFull: false,
		},
		{
			name: "current policy not full, and some new NGF ancestors have been built (at max)",
			cfg: ancestorConfig{
				numCurrNGFAncestors:    3,
				numCurrNonNGFAncestors: 11,
				numNewNGFAncestors:     5,
			},
			expFull: true,
		},
		{
			name: "current policy is full of non-NGF ancestors",
			cfg: ancestorConfig{
				numCurrNGFAncestors:    0,
				numCurrNonNGFAncestors: 16,
				numNewNGFAncestors:     0,
			},
			expFull: true,
		},
		{
			name: "current policy is full of a mix of ancestors, but updated list is empty",
			cfg: ancestorConfig{
				numCurrNGFAncestors:    3,
				numCurrNonNGFAncestors: 13,
				numNewNGFAncestors:     0,
			},
			expFull: false,
		},
		{
			name: "current policy is full of NGF ancestors, but updated ancestors is less than that",
			cfg: ancestorConfig{
				numCurrNGFAncestors:    16,
				numCurrNonNGFAncestors: 0,
				numNewNGFAncestors:     5,
			},
			expFull: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			policy := createPolicy(test.cfg)
			full := ngfPolicyAncestorsFull(policy, "nginx-gateway")
			g.Expect(full).To(Equal(test.expFull))
		})
	}
}

func TestAncestorContainsAncestorRef(t *testing.T) {
	t.Parallel()

	gw1 := types.NamespacedName{Namespace: testNs, Name: "gw1"}
	gw2 := types.NamespacedName{Namespace: testNs, Name: "gw2"}
	route := types.NamespacedName{Namespace: testNs, Name: "route"}
	newRoute := types.NamespacedName{Namespace: testNs, Name: "new-route"}

	ancestors := []PolicyAncestor{
		{
			Ancestor: createParentReference(v1.GroupName, kinds.Gateway, gw1),
		},
		{
			Ancestor: createParentReference(v1.GroupName, kinds.Gateway, gw2),
		},
		{
			Ancestor: createParentReference(v1.GroupName, kinds.HTTPRoute, route),
		},
	}

	tests := []struct {
		ref      v1.ParentReference
		name     string
		contains bool
	}{
		{
			name:     "contains Gateway ref",
			ref:      createParentReference(v1.GroupName, kinds.Gateway, gw1),
			contains: true,
		},
		{
			name:     "contains Route ref",
			ref:      createParentReference(v1.GroupName, kinds.HTTPRoute, route),
			contains: true,
		},
		{
			name:     "does not contain ref",
			ref:      createParentReference(v1.GroupName, kinds.HTTPRoute, newRoute),
			contains: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(ancestorsContainsAncestorRef(ancestors, test.ref)).To(Equal(test.contains))
		})
	}
}

func TestParentRefEqual(t *testing.T) {
	t.Parallel()
	ref1NsName := types.NamespacedName{Namespace: testNs, Name: "ref1"}

	ref1 := createParentReference(v1.GroupName, kinds.HTTPRoute, ref1NsName)

	tests := []struct {
		ref   v1.ParentReference
		name  string
		equal bool
	}{
		{
			name:  "kinds different",
			ref:   createParentReference(v1.GroupName, kinds.Gateway, ref1NsName),
			equal: false,
		},
		{
			name:  "groups different",
			ref:   createParentReference("diff-group", kinds.HTTPRoute, ref1NsName),
			equal: false,
		},
		{
			name: "namespace different",
			ref: createParentReference(
				v1.GroupName,
				kinds.HTTPRoute,
				types.NamespacedName{Namespace: "diff-ns", Name: "ref1"},
			),
			equal: false,
		},
		{
			name: "name different",
			ref: createParentReference(
				v1.GroupName,
				kinds.HTTPRoute,
				types.NamespacedName{Namespace: testNs, Name: "diff-name"},
			),
			equal: false,
		},
		{
			name:  "equal",
			ref:   createParentReference(v1.GroupName, kinds.HTTPRoute, ref1NsName),
			equal: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(parentRefEqual(ref1, test.ref)).To(Equal(test.equal))
		})
	}
}

func TestLogAncestorLimitReached(t *testing.T) {
	t.Parallel()
	logger := testr.New(t)
	logAncestorLimitReached(logger, "test-policy", "TestPolicy", "test-ancestor")
}

func TestGetAncestorName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ref      v1.ParentReference
		expected string
	}{
		{
			name: "with namespace",
			ref: v1.ParentReference{
				Name:      "test-gw",
				Namespace: func() *v1.Namespace { ns := v1.Namespace("test-ns"); return &ns }(),
			},
			expected: "test-ns/test-gw",
		},
		{
			name: "without namespace",
			ref: v1.ParentReference{
				Name: "test-gw",
			},
			expected: "test-gw",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(getAncestorName(test.ref)).To(Equal(test.expected))
		})
	}
}

func TestGetPolicyName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	policy := &policiesfakes.FakePolicy{}
	policy.GetNameReturns("test-policy")
	policy.GetNamespaceReturns("test-ns")
	g.Expect(getPolicyName(policy)).To(Equal("test-ns/test-policy"))
}

func TestGetPolicyKind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func() policies.Policy
		expected string
	}{
		{
			name: "with kind",
			setup: func() policies.Policy {
				policy := &policiesfakes.FakePolicy{}
				objectKind := &policiesfakes.FakeObjectKind{}
				objectKind.GroupVersionKindReturns(schema.GroupVersionKind{Kind: "TestPolicy"})
				policy.GetObjectKindReturns(objectKind)
				return policy
			},
			expected: "TestPolicy",
		},
		{
			name: "without kind",
			setup: func() policies.Policy {
				policy := &policiesfakes.FakePolicy{}
				policy.GetObjectKindReturns(nil)
				return policy
			},
			expected: "Policy",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			policy := test.setup()
			g.Expect(getPolicyKind(policy)).To(Equal(test.expected))
		})
	}
}

func TestCompareNamespacedNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a, b     types.NamespacedName
		expected bool
	}{
		{
			name:     "same namespace, a name < b name",
			a:        types.NamespacedName{Namespace: "ns", Name: "a"},
			b:        types.NamespacedName{Namespace: "ns", Name: "b"},
			expected: true,
		},
		{
			name:     "same namespace, a name > b name",
			a:        types.NamespacedName{Namespace: "ns", Name: "b"},
			b:        types.NamespacedName{Namespace: "ns", Name: "a"},
			expected: false,
		},
		{
			name:     "a namespace < b namespace",
			a:        types.NamespacedName{Namespace: "a", Name: "z"},
			b:        types.NamespacedName{Namespace: "b", Name: "a"},
			expected: true,
		},
		{
			name:     "a namespace > b namespace",
			a:        types.NamespacedName{Namespace: "b", Name: "a"},
			b:        types.NamespacedName{Namespace: "a", Name: "z"},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(compareNamespacedNames(test.a, test.b)).To(Equal(test.expected))
		})
	}
}

func TestSortGatewaysByCreationTime(t *testing.T) {
	t.Parallel()
	now := time.Now()
	gw1Name := types.NamespacedName{Namespace: "test", Name: "gw1"}
	gw2Name := types.NamespacedName{Namespace: "test", Name: "gw2"}

	tests := []struct {
		name     string
		gateways map[types.NamespacedName]*Gateway
		names    []types.NamespacedName
		expected []types.NamespacedName
	}{
		{
			name: "sort by creation time",
			gateways: map[types.NamespacedName]*Gateway{
				gw1Name: {Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(time.Hour))},
				}},
				gw2Name: {Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now)},
				}},
			},
			names:    []types.NamespacedName{gw1Name, gw2Name},
			expected: []types.NamespacedName{gw2Name, gw1Name},
		},
		{
			name: "same creation time, sort by namespace/name",
			gateways: map[types.NamespacedName]*Gateway{
				gw2Name: {Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now)},
				}},
				gw1Name: {Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now)},
				}},
			},
			names:    []types.NamespacedName{gw2Name, gw1Name},
			expected: []types.NamespacedName{gw1Name, gw2Name},
		},
		{
			name:     "nil gateway fallback to namespace/name",
			gateways: map[types.NamespacedName]*Gateway{gw1Name: nil},
			names:    []types.NamespacedName{gw2Name, gw1Name},
			expected: []types.NamespacedName{gw1Name, gw2Name},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			names := make([]types.NamespacedName, len(test.names))
			copy(names, test.names)
			sortGatewaysByCreationTime(names, test.gateways)
			g.Expect(names).To(Equal(test.expected))
		})
	}
}
