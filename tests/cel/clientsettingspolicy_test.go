package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestClientSettingsPoliciesTargetRefKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.ClientSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate TargetRef of kind Gateway is allowed",
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name: "Validate TargetRef of kind HTTPRoute is allowed",
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  httpRouteKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name: "Validate TargetRef of kind GRPCRoute is allowed",
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  grpcRouteKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name:       "Validate Invalid TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefKindError},
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  invalidKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name:       "Validate TCPRoute TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefKindError},
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  tcpRouteKind,
					Group: gatewayGroup,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec
			spec.TargetRef.Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, clientSettingsPolicy, k8sClient)
		})
	}
}

func TestClientSettingsPoliciesTargetRefGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.ClientSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate gateway.networking.k8s.io TargetRef Group is allowed",
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name:       "Validate invalid.networking.k8s.io TargetRef Group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: invalidGroup,
				},
			},
		},
		{
			name:       "Validate discovery.k8s.io/v1 TargetRef Group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: discoveryGroup,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec
			spec.TargetRef.Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, clientSettingsPolicy, k8sClient)
		})
	}
}

func TestClientSettingsPoliciesKeepAliveTimeout(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.ClientSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate KeepAliveTimeout is not set",
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: gatewayGroup,
				},
				KeepAlive: nil,
			},
		},
		{
			name: "Validate KeepAlive is set",
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: gatewayGroup,
				},
				KeepAlive: &ngfAPIv1alpha1.ClientKeepAlive{
					Timeout: &ngfAPIv1alpha1.ClientKeepAliveTimeout{
						Server: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
						Header: helpers.GetPointer[ngfAPIv1alpha1.Duration]("2s"),
					},
				},
			},
		},
		{
			name:       "Validate Header cannot be set without Server",
			wantErrors: []string{expectedHeaderWithoutServerError},
			spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: gatewayGroup,
				},
				KeepAlive: &ngfAPIv1alpha1.ClientKeepAlive{
					Timeout: &ngfAPIv1alpha1.ClientKeepAliveTimeout{
						Header: helpers.GetPointer[ngfAPIv1alpha1.Duration]("2s"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec
			spec.TargetRef.Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, clientSettingsPolicy, k8sClient)
		})
	}
}
