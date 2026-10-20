package framework

import (
	"crypto/rand"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

func acceptedCond(status metav1.ConditionStatus, reason gatewayv1.PolicyConditionReason) metav1.Condition {
	return metav1.Condition{Type: string(gatewayv1.PolicyConditionAccepted), Status: status, Reason: string(reason)}
}

func ancestorFor(
	ns string,
	kind gatewayv1.Kind,
	name gatewayv1.ObjectName,
	conds ...metav1.Condition,
) gatewayv1.PolicyAncestorStatus {
	group := gatewayv1.Group(gatewayv1.GroupName)
	namespace := gatewayv1.Namespace(ns)
	return gatewayv1.PolicyAncestorStatus{
		AncestorRef: gatewayv1.ParentReference{
			Group:     &group,
			Kind:      &kind,
			Namespace: &namespace,
			Name:      name,
		},
		ControllerName: NgfControllerName,
		Conditions:     conds,
	}
}

func TestCheckPolicyStatusMatchingTargetRefs(t *testing.T) {
	t.Parallel()

	// Random names guard against assertions that only pass for fixed values.
	suffix := strings.ToLower(rand.Text()[:8])
	ns := "ns-" + suffix
	gw := gatewayv1.ObjectName("gw-" + suffix)
	route := gatewayv1.ObjectName("route-" + suffix)

	accepted := acceptedCond(metav1.ConditionTrue, gatewayv1.PolicyReasonAccepted)
	programmed := metav1.Condition{Type: "Programmed", Status: metav1.ConditionTrue, Reason: "Programmed"}
	conflicted := acceptedCond(metav1.ConditionFalse, gatewayv1.PolicyReasonConflicted)

	gwRef := gatewayv1.LocalPolicyTargetReference{Group: gatewayv1.GroupName, Kind: "Gateway", Name: gw}
	routeRef := gatewayv1.LocalPolicyTargetReference{Group: gatewayv1.GroupName, Kind: "HTTPRoute", Name: route}
	gwAncestor := func(conds ...metav1.Condition) gatewayv1.PolicyAncestorStatus {
		return ancestorFor(ns, "Gateway", gw, conds...)
	}
	routeAncestor := func(conds ...metav1.Condition) gatewayv1.PolicyAncestorStatus {
		return ancestorFor(ns, "HTTPRoute", route, conds...)
	}

	tests := []struct {
		name        string
		errContains string
		expected    PolicyCondition
		targetRefs  []gatewayv1.LocalPolicyTargetReference
		ancestors   []gatewayv1.PolicyAncestorStatus
	}{
		{
			name:        "no ancestors yet",
			targetRefs:  []gatewayv1.LocalPolicyTargetReference{gwRef},
			expected:    PolicyAccepted,
			errContains: "no ancestor status yet",
		},
		{
			name:       "one target accepted",
			targetRefs: []gatewayv1.LocalPolicyTargetReference{gwRef},
			ancestors:  []gatewayv1.PolicyAncestorStatus{gwAncestor(accepted)},
			expected:   PolicyAccepted,
		},
		{
			name:       "many targets accepted in a different order",
			targetRefs: []gatewayv1.LocalPolicyTargetReference{gwRef, routeRef},
			ancestors:  []gatewayv1.PolicyAncestorStatus{routeAncestor(accepted), gwAncestor(accepted)},
			expected:   PolicyAccepted,
		},
		{
			name:       "Programmed listed before Accepted",
			targetRefs: []gatewayv1.LocalPolicyTargetReference{gwRef},
			ancestors:  []gatewayv1.PolicyAncestorStatus{gwAncestor(programmed, accepted)},
			expected:   PolicyAccepted,
		},
		{
			name:       "non-accepted condition expected and present",
			targetRefs: []gatewayv1.LocalPolicyTargetReference{routeRef},
			ancestors:  []gatewayv1.PolicyAncestorStatus{routeAncestor(conflicted)},
			expected: PolicyCondition{
				Status: metav1.ConditionFalse,
				Reason: gatewayv1.PolicyReasonConflicted,
			},
		},
		{
			name:        "fewer ancestors than targets",
			targetRefs:  []gatewayv1.LocalPolicyTargetReference{gwRef, routeRef},
			ancestors:   []gatewayv1.PolicyAncestorStatus{gwAncestor(accepted)},
			expected:    PolicyAccepted,
			errContains: "policy has 1 ancestors, expected 2",
		},
		{
			name:        "ancestor kind differs from target",
			targetRefs:  []gatewayv1.LocalPolicyTargetReference{routeRef},
			ancestors:   []gatewayv1.PolicyAncestorStatus{ancestorFor(ns, "GRPCRoute", route, accepted)},
			expected:    PolicyAccepted,
			errContains: "no targetRef matches ancestor",
		},
		{
			name:        "ancestor in another namespace",
			targetRefs:  []gatewayv1.LocalPolicyTargetReference{gwRef},
			ancestors:   []gatewayv1.PolicyAncestorStatus{ancestorFor(ns+"-other", "Gateway", gw, accepted)},
			expected:    PolicyAccepted,
			errContains: "expected ancestor namespace",
		},
		{
			name:       "ancestor written by another controller",
			targetRefs: []gatewayv1.LocalPolicyTargetReference{gwRef},
			ancestors: func() []gatewayv1.PolicyAncestorStatus {
				a := gwAncestor(accepted)
				a.ControllerName = "example.com/other-controller"
				return []gatewayv1.PolicyAncestorStatus{a}
			}(),
			expected:    PolicyAccepted,
			errContains: "expected ancestor controller name",
		},
		{
			name:        "Accepted status differs",
			targetRefs:  []gatewayv1.LocalPolicyTargetReference{gwRef},
			ancestors:   []gatewayv1.PolicyAncestorStatus{gwAncestor(conflicted)},
			expected:    PolicyAccepted,
			errContains: "expected condition status",
		},
		{
			name:       "Accepted reason differs",
			targetRefs: []gatewayv1.LocalPolicyTargetReference{routeRef},
			ancestors: []gatewayv1.PolicyAncestorStatus{
				routeAncestor(acceptedCond(metav1.ConditionFalse, gatewayv1.PolicyReasonInvalid)),
			},
			expected: PolicyCondition{
				Status: metav1.ConditionFalse,
				Reason: gatewayv1.PolicyReasonConflicted,
			},
			errContains: "expected condition reason",
		},
		{
			name:        "no Accepted condition",
			targetRefs:  []gatewayv1.LocalPolicyTargetReference{gwRef},
			ancestors:   []gatewayv1.PolicyAncestorStatus{gwAncestor(programmed)},
			expected:    PolicyAccepted,
			errContains: "has no Accepted condition",
		},
		{
			name:        "one of many ancestors not accepted",
			targetRefs:  []gatewayv1.LocalPolicyTargetReference{gwRef, routeRef},
			ancestors:   []gatewayv1.PolicyAncestorStatus{gwAncestor(accepted), routeAncestor(conflicted)},
			expected:    PolicyAccepted,
			errContains: "expected condition status",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("start: %s", test.name)
			defer t.Logf("end: %s", test.name)
			g := NewWithT(t)

			policy := &ngfAPI.RateLimitPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "rlp", Namespace: ns},
				Spec:       ngfAPI.RateLimitPolicySpec{TargetRefs: test.targetRefs},
				Status:     gatewayv1.PolicyStatus{Ancestors: test.ancestors},
			}

			err := checkPolicyStatus(policy, test.expected, AncestorsMatchTargetRefs)
			if test.errContains == "" {
				g.Expect(err).ToNot(HaveOccurred())
				return
			}
			g.Expect(err).To(MatchError(ContainSubstring(test.errContains)))
		})
	}
}

func TestCheckPolicyStatusGatewayAncestor(t *testing.T) {
	t.Parallel()

	suffix := strings.ToLower(rand.Text()[:8])
	ns := "ns-" + suffix
	gw := gatewayv1.ObjectName("gw-" + suffix)
	accepted := acceptedCond(metav1.ConditionTrue, gatewayv1.PolicyReasonAccepted)

	tests := []struct {
		name        string
		errContains string
		ancestors   []gatewayv1.PolicyAncestorStatus
	}{
		{
			name:      "Gateway ancestor for a Service target",
			ancestors: []gatewayv1.PolicyAncestorStatus{ancestorFor(ns, "Gateway", gw, accepted)},
		},
		{
			name:        "different Gateway",
			ancestors:   []gatewayv1.PolicyAncestorStatus{ancestorFor(ns, "Gateway", gw+"-other", accepted)},
			errContains: "expected ancestorRef to have name",
		},
		{
			name:        "ancestor is not a Gateway",
			ancestors:   []gatewayv1.PolicyAncestorStatus{ancestorFor(ns, "HTTPRoute", gw, accepted)},
			errContains: "expected ancestorRef to have kind",
		},
		{
			name: "more than one ancestor",
			ancestors: []gatewayv1.PolicyAncestorStatus{
				ancestorFor(ns, "Gateway", gw, accepted),
				ancestorFor(ns, "Gateway", gw+"-2", accepted),
			},
			errContains: "policy has 2 ancestors, expected 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("start: %s", test.name)
			defer t.Logf("end: %s", test.name)
			g := NewWithT(t)

			// The ancestor is the owning Gateway, not the Service the policy targets.
			policy := &ngfAPI.UpstreamSettingsPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "usp", Namespace: ns},
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					TargetRefs: []gatewayv1.LocalPolicyTargetReference{{Group: "core", Kind: "Service", Name: "coffee"}},
				},
				Status: gatewayv1.PolicyStatus{Ancestors: test.ancestors},
			}

			err := checkPolicyStatus(policy, PolicyAccepted, AncestorIsGateway(string(gw)))
			if test.errContains == "" {
				g.Expect(err).ToNot(HaveOccurred())
				return
			}
			g.Expect(err).To(MatchError(ContainSubstring(test.errContains)))
		})
	}
}
