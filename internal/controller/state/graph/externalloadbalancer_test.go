package graph

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func createELB(
	name, namespace string,
	created time.Time,
	ref v1.LocalPolicyTargetReference,
) *ngfAPIv1alpha1.ExternalLoadBalancer {
	return &ngfAPIv1alpha1.ExternalLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: ngfAPIv1alpha1.ExternalLoadBalancerSpec{
			TargetRefs: []v1.LocalPolicyTargetReference{ref},
			GatewayLink: &ngfAPIv1alpha1.GatewayLinkConfig{
				VirtualServerAddress: helpers.GetPointer("10.0.0.1"),
			},
		},
	}
}

func gatewayTargetRef(name string) v1.LocalPolicyTargetReference {
	return v1.LocalPolicyTargetReference{
		Group: v1.GroupName,
		Kind:  "Gateway",
		Name:  v1.ObjectName(name),
	}
}

func createGatewayForELB(name string) *Gateway {
	return &Gateway{
		Source: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		},
	}
}

func TestProcessExternalLoadBalancers(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		elbs         map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer
		expAttached  *ngfAPIv1alpha1.ExternalLoadBalancer
		expProcessed map[types.NamespacedName]*ExternalLoadBalancer
		name         string
	}{
		{
			name: "an ExternalLoadBalancer targeting the Gateway is attached and marked Accepted",
			elbs: map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer{
				{Namespace: "test", Name: "elb"}: createELB("elb", "test", baseTime, gatewayTargetRef("gateway")),
			},
			expAttached: createELB("elb", "test", baseTime, gatewayTargetRef("gateway")),
			expProcessed: map[types.NamespacedName]*ExternalLoadBalancer{
				{Namespace: "test", Name: "elb"}: {
					Valid:      true,
					Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerAccepted()},
				},
			},
		},
		{
			name: "an ExternalLoadBalancer targeting a Gateway that does not exist is Invalid and not attached",
			elbs: map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer{
				{Namespace: "test", Name: "elb"}: createELB("elb", "test", baseTime, gatewayTargetRef("does-not-exist")),
			},
			expAttached: nil,
			expProcessed: map[types.NamespacedName]*ExternalLoadBalancer{
				{Namespace: "test", Name: "elb"}: {
					Valid: false,
					Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerInvalid(
						"targetRef references a Gateway that does not exist",
					)},
				},
			},
		},
		{
			name: "two ExternalLoadBalancer target the same Gateway, the one created first is Accepted " +
				"and the newer one is Conflicted",
			elbs: map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer{
				{Namespace: "test", Name: "newer"}: createELB(
					"newer", "test", baseTime.Add(time.Hour), gatewayTargetRef("gateway"),
				),
				{Namespace: "test", Name: "older"}: createELB("older", "test", baseTime, gatewayTargetRef("gateway")),
			},
			expAttached: createELB("older", "test", baseTime, gatewayTargetRef("gateway")),
			expProcessed: map[types.NamespacedName]*ExternalLoadBalancer{
				{Namespace: "test", Name: "older"}: {
					Valid:      true,
					Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerAccepted()},
				},
				{Namespace: "test", Name: "newer"}: {
					Valid: false,
					Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerConflicted(
						`Gateway "test/gateway" is already fronted by ExternalLoadBalancer "older"`,
					)},
				},
			},
		},
		{
			name: "two ExternalLoadBalancer with the same creation timestamp target the same Gateway " +
				"a-elb is Accepted because it sorts first by name and b-elb is Conflicted",
			elbs: map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer{
				{Namespace: "test", Name: "b-elb"}: createELB("b-elb", "test", baseTime, gatewayTargetRef("gateway")),
				{Namespace: "test", Name: "a-elb"}: createELB("a-elb", "test", baseTime, gatewayTargetRef("gateway")),
			},
			expAttached: createELB("a-elb", "test", baseTime, gatewayTargetRef("gateway")),
			expProcessed: map[types.NamespacedName]*ExternalLoadBalancer{
				{Namespace: "test", Name: "a-elb"}: {
					Valid:      true,
					Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerAccepted()},
				},
				{Namespace: "test", Name: "b-elb"}: {
					Valid: false,
					Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerConflicted(
						`Gateway "test/gateway" is already fronted by ExternalLoadBalancer "a-elb"`,
					)},
				},
			},
		},
		{
			name: "an ExternalLoadBalancer whose additionalIngressLinkSpec sets a Common partition is Invalid " +
				"and not attached",
			elbs: map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer{
				{Namespace: "test", Name: "elb"}: elbWithAdditionalSpec(
					"elb", "test", baseTime, gatewayTargetRef("gateway"), `{"partition":"Common"}`,
				),
			},
			expAttached: nil,
			expProcessed: map[types.NamespacedName]*ExternalLoadBalancer{
				{Namespace: "test", Name: "elb"}: {
					Valid: false,
					Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerInvalid(
						"additionalIngressLinkSpec.partition cannot be Common",
					)},
				},
			},
		},
		{
			name: "an ExternalLoadBalancer whose additionalIngressLinkSpec sets a non-Common partition is Accepted",
			elbs: map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer{
				{Namespace: "test", Name: "elb"}: elbWithAdditionalSpec(
					"elb", "test", baseTime, gatewayTargetRef("gateway"), `{"partition":"k8s"}`,
				),
			},
			expAttached: elbWithAdditionalSpec(
				"elb", "test", baseTime, gatewayTargetRef("gateway"), `{"partition":"k8s"}`,
			),
			expProcessed: map[types.NamespacedName]*ExternalLoadBalancer{
				{Namespace: "test", Name: "elb"}: {
					Valid:      true,
					Conditions: []conditions.Condition{conditions.NewExternalLoadBalancerAccepted()},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gw := createGatewayForELB("gateway")
			// otherGW is never targeted, so it must stay untouched regardless of the case.
			otherGW := createGatewayForELB("other-gateway")
			gws := map[types.NamespacedName]*Gateway{
				gwNsName: gw,
				{Namespace: "test", Name: "other-gateway"}: otherGW,
			}

			processed := processExternalLoadBalancers(test.elbs, gws)

			g.Expect(processed).To(HaveLen(len(test.expProcessed)))
			for nsName, exp := range test.expProcessed {
				got := processed[nsName]
				g.Expect(got).ToNot(BeNil())
				g.Expect(got.Valid).To(Equal(exp.Valid))
				g.Expect(got.Conditions).To(Equal(exp.Conditions))
				g.Expect(got.Source).ToNot(BeNil())
			}

			g.Expect(otherGW.ExternalLoadBalancer).To(BeNil())

			if test.expAttached == nil {
				g.Expect(gw.ExternalLoadBalancer).To(BeNil())
				return
			}

			g.Expect(gw.ExternalLoadBalancer).ToNot(BeNil())
			g.Expect(gw.ExternalLoadBalancer.Name).To(Equal(test.expAttached.Name))
			g.Expect(gw.ExternalLoadBalancer.Namespace).To(Equal(test.expAttached.Namespace))
		})
	}
}

func elbWithAdditionalSpec(
	name, namespace string,
	created time.Time,
	ref v1.LocalPolicyTargetReference,
	rawSpec string,
) *ngfAPIv1alpha1.ExternalLoadBalancer {
	elb := createELB(name, namespace, created, ref)
	elb.Spec.GatewayLink.AdditionalIngressLinkSpec = &apiextv1.JSON{Raw: []byte(rawSpec)}
	return elb
}

func TestProcessExternalLoadBalancersNoOp(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("no ExternalLoadBalancer returns nil and leaves Gateways untouched", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		gw := createGatewayForELB("gateway")
		gws := map[types.NamespacedName]*Gateway{{Namespace: "test", Name: "gateway"}: gw}

		g.Expect(processExternalLoadBalancers(nil, gws)).To(BeNil())
		g.Expect(gw.ExternalLoadBalancer).To(BeNil())
	})

	t.Run("no Gateways invalidates the ExternalLoadBalancer without panicking", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		elbs := map[types.NamespacedName]*ngfAPIv1alpha1.ExternalLoadBalancer{
			{Namespace: "test", Name: "elb"}: createELB("elb", "test", baseTime, gatewayTargetRef("gateway")),
		}

		var processed map[types.NamespacedName]*ExternalLoadBalancer
		g.Expect(func() { processed = processExternalLoadBalancers(elbs, nil) }).ToNot(Panic())
		g.Expect(processed[types.NamespacedName{Namespace: "test", Name: "elb"}].Valid).To(BeFalse())
	})
}
