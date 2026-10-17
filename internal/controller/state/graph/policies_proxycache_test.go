package graph

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func newProxyCachePolicy(caching *ngfAPIv1alpha1.ProxyCaching, targetRefs ...PolicyTargetRef) *Policy {
	return &Policy{
		Source: &ngfAPIv1alpha1.ProxySettingsPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "psp", Namespace: testNs},
			Spec:       ngfAPIv1alpha1.ProxySettingsPolicySpec{Caching: caching},
		},
		TargetRefs:         targetRefs,
		InvalidForGateways: map[types.NamespacedName]struct{}{},
	}
}

func TestProxyCacheZoneMissing(t *testing.T) {
	t.Parallel()

	caching := &ngfAPIv1alpha1.ProxyCaching{Enable: helpers.GetPointer(true)}
	zone := &EffectiveNginxProxy{Cache: &ngfAPIv1alpha2.ProxyCache{MaxSize: "1g"}}

	tests := []struct {
		policy *Policy
		np     *EffectiveNginxProxy
		name   string
		exp    bool
	}{
		{
			name:   "caching configured, no NginxProxy",
			policy: newProxyCachePolicy(caching),
			np:     nil,
			exp:    true,
		},
		{
			name:   "caching configured, NginxProxy without cache zone",
			policy: newProxyCachePolicy(caching),
			np:     &EffectiveNginxProxy{},
			exp:    true,
		},
		{
			name:   "caching disabled but configured, no cache zone",
			policy: newProxyCachePolicy(&ngfAPIv1alpha1.ProxyCaching{Enable: helpers.GetPointer(false)}),
			np:     &EffectiveNginxProxy{},
			exp:    true,
		},
		{
			name:   "caching configured, cache zone present",
			policy: newProxyCachePolicy(caching),
			np:     zone,
			exp:    false,
		},
		{
			name:   "caching not configured, no NginxProxy",
			policy: newProxyCachePolicy(nil),
			np:     nil,
			exp:    false,
		},
		{
			name: "not a ProxySettingsPolicy",
			policy: &Policy{Source: &policiesfakes.PolicyMock{
				GetNameFunc: func() string { return "other" },
			}},
			np:  nil,
			exp: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(proxyCacheZoneMissing(test.policy, test.np)).To(Equal(test.exp))
		})
	}
}

func TestAttachProxyCachePolicyToRoute(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: testNs, Name: "gw"}
	routeNsName := types.NamespacedName{Namespace: testNs, Name: "hr"}
	caching := &ngfAPIv1alpha1.ProxyCaching{Enable: helpers.GetPointer(true)}

	validator := &policiesfakes.FakeValidator{
		ValidateGlobalSettingsStub: func(_ policies.Policy, _ *policies.GlobalSettings) []conditions.Condition {
			return nil
		},
	}

	newRoute := func(np *EffectiveNginxProxy) *L7Route {
		return &L7Route{
			Source: &v1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Name: routeNsName.Name, Namespace: routeNsName.Namespace},
			},
			Valid:      true,
			Attachable: true,
			RouteType:  RouteTypeHTTP,
			ParentRefs: []ParentRef{
				{
					Kind:                kinds.Gateway,
					GatewayNsName:       gwNsName,
					EffectiveNginxProxy: np,
					Attachment:          &ParentRefAttachmentStatus{Attached: true},
				},
			},
		}
	}

	tests := []struct {
		np          *EffectiveNginxProxy
		name        string
		expAttached bool
	}{
		{
			// Without this guard the policy would render "proxy_cache ngf_cache" with no zone defined,
			// and NGINX would reject the whole configuration.
			name:        "not attached when there is no NginxProxy",
			np:          nil,
			expAttached: false,
		},
		{
			name:        "not attached when NginxProxy has no cache zone",
			np:          &EffectiveNginxProxy{},
			expAttached: false,
		},
		{
			name:        "attached when NginxProxy has a cache zone",
			np:          &EffectiveNginxProxy{Cache: &ngfAPIv1alpha2.ProxyCache{MaxSize: "1g"}},
			expAttached: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			route := newRoute(test.np)
			policy := newProxyCachePolicy(caching)

			attachPolicyToRoute(policy, route, validator, "nginx-gateway", logr.Discard())

			if test.expAttached {
				g.Expect(route.Policies).To(HaveLen(1))
				g.Expect(policy.InvalidForGateways).To(BeEmpty())
				return
			}

			g.Expect(route.Policies).To(BeEmpty())
			g.Expect(policy.InvalidForGateways).To(HaveKey(gwNsName))
			g.Expect(policy.Ancestors).To(HaveLen(1))
			g.Expect(policy.Ancestors[0].Conditions).To(ContainElement(
				conditions.NewPolicyInvalid(conditions.PolicyMessageProxyCacheZoneMissing),
			))
		})
	}
}

func TestAttachProxyCachePolicyToGateway(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: testNs, Name: "gw"}
	caching := &ngfAPIv1alpha1.ProxyCaching{Enable: helpers.GetPointer(true)}

	validator := &policiesfakes.FakeValidator{
		ValidateGlobalSettingsStub: func(_ policies.Policy, _ *policies.GlobalSettings) []conditions.Condition {
			return nil
		},
	}

	tests := []struct {
		np          *EffectiveNginxProxy
		name        string
		expAttached bool
	}{
		{name: "not attached when there is no NginxProxy", np: nil, expAttached: false},
		{name: "not attached when NginxProxy has no cache zone", np: &EffectiveNginxProxy{}, expAttached: false},
		{
			name:        "attached when NginxProxy has a cache zone",
			np:          &EffectiveNginxProxy{Cache: &ngfAPIv1alpha2.ProxyCache{MaxSize: "1g"}},
			expAttached: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gws := map[types.NamespacedName]*Gateway{
				gwNsName: {
					Source: &v1.Gateway{
						ObjectMeta: metav1.ObjectMeta{Name: gwNsName.Name, Namespace: gwNsName.Namespace},
					},
					Valid:               true,
					EffectiveNginxProxy: test.np,
				},
			}
			ref := PolicyTargetRef{Nsname: gwNsName, Kind: kinds.Gateway}
			policy := newProxyCachePolicy(caching, ref)

			attachPolicyToGateway(policy, ref, gws, nil, "nginx-gateway", logr.Discard(), validator)

			if test.expAttached {
				g.Expect(gws[gwNsName].Policies).To(HaveLen(1))
				g.Expect(policy.InvalidForGateways).To(BeEmpty())
				return
			}

			g.Expect(gws[gwNsName].Policies).To(BeEmpty())
			g.Expect(policy.InvalidForGateways).To(HaveKey(gwNsName))
			g.Expect(policy.Ancestors).To(HaveLen(1))
			g.Expect(policy.Ancestors[0].Conditions).To(ContainElement(
				conditions.NewPolicyInvalid(conditions.PolicyMessageProxyCacheZoneMissing),
			))
		})
	}
}
