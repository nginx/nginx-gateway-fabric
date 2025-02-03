package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

func TestBackendTLSPolicyAncestorsFull(t *testing.T) {
	t.Parallel()
	createCurStatus := func(numAncestors int, ctlrName string) []v1alpha2.PolicyAncestorStatus {
		statuses := make([]v1alpha2.PolicyAncestorStatus, 0, numAncestors)

		for range numAncestors {
			statuses = append(statuses, v1alpha2.PolicyAncestorStatus{
				ControllerName: v1.GatewayController(ctlrName),
			})
		}

		return statuses
	}

	tests := []struct {
		name      string
		curStatus []v1alpha2.PolicyAncestorStatus
		expFull   bool
	}{
		{
			name:      "not full",
			curStatus: createCurStatus(15, "controller"),
			expFull:   false,
		},
		{
			name:      "full; ancestor does not exist in current status",
			curStatus: createCurStatus(16, "controller"),
			expFull:   true,
		},
		{
			name:      "full, but ancestor does exist in current status",
			curStatus: createCurStatus(16, "nginx-gateway"),
			expFull:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			full := backendTLSPolicyAncestorsFull(test.curStatus, "nginx-gateway")
			g.Expect(full).To(Equal(test.expFull))
		})
	}
}

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
