package cel

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

func TestProxySettingsPolicyTargetRefsKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.ProxySettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate TargetRef of kind Gateway is allowed",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "Validate TargetRef of kind HTTPRoute is allowed",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "Validate TargetRef of kind GRPCRoute is allowed",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "Validate TargetRefs of kind GRPCRoute and HTTPRoute are allowed",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRefs of kind Gateway and HTTPRoute are not allowed",
			wantErrors: []string{"Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs"},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRefs of kind Gateway and GRPCRoute are not allowed",
			wantErrors: []string{"Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs"},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRefs with Gateway, HTTPRoute, and GRPCRoute are not allowed",
			wantErrors: []string{"Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs"},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
					},
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate invalid TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  invalidKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TCPRoute TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  tcpRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate valid and invalid TargetRefs Kinds is not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  invalidKind,
						Group: gatewayGroup,
					},
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate more than one invalid TargetRefs Kinds are not allowed",
			wantErrors: []string{expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  invalidKind,
						Group: gatewayGroup,
					},
					{
						Kind:  invalidKind,
						Group: gatewayGroup,
					},
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
			psp := &ngfAPIv1alpha1.ProxySettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, psp, k8sClient)
		})
	}
}

func TestProxySettingsPolicyTargetRefsGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.ProxySettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate gateway.networking.k8s.io TargetRef Group is allowed",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate invalid.networking.k8s.io TargetRef Group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: invalidGroup,
					},
				},
			},
		},
		{
			name:       "Validate valid and invalid TargetRef Group are not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: invalidGroup,
					},
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate more than one invalid TargetRef Group are not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: invalidGroup,
					},
					{
						Kind:  gatewayKind,
						Group: discoveryGroup,
					},
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
			psp := &ngfAPIv1alpha1.ProxySettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, psp, k8sClient)
		})
	}
}

func TestProxySettingsPolicyTargetRefsNameUniqueness(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.ProxySettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate single TargetRef with unique name is allowed",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "Validate multiple TargetRefs with unique names are allowed",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
					},
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "Validate multiple name duplicates for different TargetRefs Kind are allowed",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "duplicate-service-name",
					},
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
						Name:  "duplicate-service-name", // Same name as above
					},
				},
			},
		},
		{
			name:       "Validate duplicate TargetRef names are not allowed",
			wantErrors: []string{expectedTargetRefKindAndNameComboMustBeUnique},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "duplicate-service-name",
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "duplicate-service-name", // Same name as above
					},
				},
			},
		},
		{
			name:       "Validate three TargetRefs with one duplicate name are not allowed",
			wantErrors: []string{expectedTargetRefKindAndNameComboMustBeUnique},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
						Name:  "unique-service-1",
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "duplicate-service-name",
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "duplicate-service-name", // Same name as above
					},
				},
			},
		},
		{
			name:       "Validate multiple duplicates are not allowed",
			wantErrors: []string{expectedTargetRefKindAndNameComboMustBeUnique},
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "service-a",
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "service-a", // Duplicate of first
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "service-b",
					},
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
						Name:  "service-b", // Duplicate of third
					},
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
			psp := &ngfAPIv1alpha1.ProxySettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, psp, k8sClient)
		})
	}
}

func TestProxySettingsPolicy_ContentCaching(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.ProxySettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "useStale contains 'off' and another value",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				Caching: &ngfAPIv1alpha1.ProxyCache{
					UseStale: []ngfAPIv1alpha1.UseStale{
						"off",
						"timeout",
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
			wantErrors: []string{expectedUseStaleError},
		},
		{
			name: "useStale contains only 'off'",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				Caching: &ngfAPIv1alpha1.ProxyCache{
					UseStale: []ngfAPIv1alpha1.UseStale{
						"off",
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "useStale contains multiple values, none are 'off'",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				Caching: &ngfAPIv1alpha1.ProxyCache{
					UseStale: []ngfAPIv1alpha1.UseStale{
						"error",
						"timeout",
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "codes is set, time is also set, no error expected",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				Caching: &ngfAPIv1alpha1.ProxyCache{
					Valid: &ngfAPIv1alpha1.ProxyCacheValid{
						Codes: []intstr.IntOrString{
							intstr.FromInt(400),
							intstr.FromString("300"),
						},
						Time: func() *ngfAPIv1alpha1.Duration {
							value := ngfAPIv1alpha1.Duration("30s")
							return &value
						}(),
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "codes is set, time is not set, error expected",
			spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
				Caching: &ngfAPIv1alpha1.ProxyCache{
					Valid: &ngfAPIv1alpha1.ProxyCacheValid{
						Codes: []intstr.IntOrString{
							intstr.FromInt(400),
							intstr.FromString("300"),
						},
					},
				},
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
			wantErrors: []string{expectedTimeNotSetError},
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
			psp := &ngfAPIv1alpha1.ProxySettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, psp, k8sClient)
		})
	}
}
