package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestAccessPolicyTargetRefsKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.AccessPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Gateway kind is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "HTTPRoute kind is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "GRPCRoute kind is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: grpcRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "multiple Route kinds are allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
					{Kind: grpcRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "invalid kind is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: invalidKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "TCPRoute kind is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: tcpRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "mix of valid and invalid kinds is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
					{Kind: invalidKind, Group: gatewayGroup},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			}
			ap := &ngfAPIv1alpha1.AccessPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, ap, k8sClient)
		})
	}
}

func TestAccessPolicyTargetRefsGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.AccessPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "gateway.networking.k8s.io group is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "invalid group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: invalidGroup},
				},
			},
		},
		{
			name:       "mix of valid and invalid groups is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
					{Kind: httpRouteKind, Group: invalidGroup},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			}
			ap := &ngfAPIv1alpha1.AccessPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, ap, k8sClient)
		})
	}
}

func TestAccessPolicyTargetRefsNameUniqueness(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.AccessPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "single TargetRef is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "multiple TargetRefs with unique kind+name combinations are allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
					{Kind: grpcRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "same name for different kinds is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "shared-name"},
					{Kind: grpcRouteKind, Group: gatewayGroup, Name: "shared-name"},
				},
			},
		},
		{
			name:       "duplicate kind+name combination is not allowed",
			wantErrors: []string{expectedTargetRefKindAndNameComboMustBeUnique},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "duplicate-route"},
					{Kind: httpRouteKind, Group: gatewayGroup, Name: "duplicate-route"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				if tt.spec.TargetRefs[i].Name == "" {
					tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
				}
			}
			ap := &ngfAPIv1alpha1.AccessPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, ap, k8sClient)
		})
	}
}

func TestAccessPolicyTargetRefsMixedKinds(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.AccessPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Gateway alone is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "HTTPRoute and GRPCRoute together are allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
					{Kind: grpcRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "Gateway and HTTPRoute cannot be mixed",
			wantErrors: []string{"Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs"},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
					{Kind: httpRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "Gateway and GRPCRoute cannot be mixed",
			wantErrors: []string{"Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs"},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
					{Kind: grpcRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "Gateway, HTTPRoute, and GRPCRoute cannot be mixed",
			wantErrors: []string{"Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs"},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules:  minimalRules(),
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
					{Kind: httpRouteKind, Group: gatewayGroup},
					{Kind: grpcRouteKind, Group: gatewayGroup},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			}
			ap := &ngfAPIv1alpha1.AccessPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, ap, k8sClient)
		})
	}
}

func TestAccessPolicyRuleNameUniqueness(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.AccessPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "single rule is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules: []ngfAPIv1alpha1.AccessRule{
					{Name: "rule-one"},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "multiple rules with unique names are allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules: []ngfAPIv1alpha1.AccessRule{
					{Name: "rule-one"},
					{Name: "rule-two"},
					{Name: "rule-three"},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "one duplicate among several unique rule names is not allowed",
			wantErrors: []string{expectedAccessRuleNamesUniqueError},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules: []ngfAPIv1alpha1.AccessRule{
					{Name: "unique-one"},
					{Name: "shared"},
					{Name: "shared"},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			}
			ap := &ngfAPIv1alpha1.AccessPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, ap, k8sClient)
		})
	}
}

func TestAccessPolicyRuleSourceIPAddress(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.AccessPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "IPAddress type with ipAddress set is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules: []ngfAPIv1alpha1.AccessRule{
					{
						Name: "rule-one",
						Source: &ngfAPIv1alpha1.AccessRuleSource{
							Type:      ngfAPIv1alpha1.AccessRuleSourceTypeIP,
							IPAddress: &ngfAPIv1alpha1.AccessRuleSourceIPAddress{Address: "10.0.0.0/8"},
						},
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "rule with no source (match-all) is allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules: []ngfAPIv1alpha1.AccessRule{
					{Name: "catch-all"},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
				},
			},
		},
		{
			name:       "IPAddress type without ipAddress is not allowed",
			wantErrors: []string{expectedAccessRuleSourceIPAddressSetError},
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules: []ngfAPIv1alpha1.AccessRule{
					{
						Name: "rule-one",
						Source: &ngfAPIv1alpha1.AccessRuleSource{
							Type: ngfAPIv1alpha1.AccessRuleSourceTypeIP,
						},
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "multiple rules with valid IPAddress sources are allowed",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionAllow,
				Rules: []ngfAPIv1alpha1.AccessRule{
					{
						Name: "ipv4-cidr",
						Source: &ngfAPIv1alpha1.AccessRuleSource{
							Type:      ngfAPIv1alpha1.AccessRuleSourceTypeIP,
							IPAddress: &ngfAPIv1alpha1.AccessRuleSourceIPAddress{Address: "192.168.0.0/24"},
						},
					},
					{
						Name: "ipv6-cidr",
						Source: &ngfAPIv1alpha1.AccessRuleSource{
							Type:      ngfAPIv1alpha1.AccessRuleSourceTypeIP,
							IPAddress: &ngfAPIv1alpha1.AccessRuleSourceIPAddress{Address: "2001:db8::/32"},
						},
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: gatewayKind, Group: gatewayGroup},
				},
			},
		},
		{
			name: "ipAddress field may be set with IPv4 host address",
			spec: ngfAPIv1alpha1.AccessPolicySpec{
				Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
				Rules: []ngfAPIv1alpha1.AccessRule{
					{
						Name: "host",
						Source: &ngfAPIv1alpha1.AccessRuleSource{
							Type:      ngfAPIv1alpha1.AccessRuleSourceTypeIP,
							IPAddress: &ngfAPIv1alpha1.AccessRuleSourceIPAddress{Address: "203.0.113.50"},
						},
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{Kind: httpRouteKind, Group: gatewayGroup},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := range tt.spec.TargetRefs {
				tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			}
			ap := &ngfAPIv1alpha1.AccessPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, ap, k8sClient)
		})
	}
}

// minimalRules returns a single no-source rule usable as a valid Rules value in most test specs.
func minimalRules() []ngfAPIv1alpha1.AccessRule {
	return []ngfAPIv1alpha1.AccessRule{{Name: "default"}}
}

// Ensure the helpers import is used when building with unused-import checks.
var _ = helpers.GetPointer[int32]
