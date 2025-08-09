package cel

import (
	"testing"

	. "github.com/onsi/gomega"
	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestClientSettingsPoliciesTargetRefKind(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	k8sClient, err := getKubernetesClient(t)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		policySpec ngfAPIv1alpha1.ClientSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate TargetRef of kind Gateway is allowed",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name: "Validate TargetRef of kind HTTPRoute is allowed",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  httpRouteKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name: "Validate TargetRef of kind GRPCRoute is allowed",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  grpcRouteKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name:       "Validate Invalid TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefKindError},
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  invalidKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name:       "Validate TCPRoute TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefKindError},
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  tcpRouteKind,
					Group: gatewayGroup,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			policySpec := tt.policySpec
			policySpec.TargetRef.Name = gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName))
			clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testPolicyName),
					Namespace: defaultNamespace,
				},
				Spec: policySpec,
			}
			validateCrd(t, tt.wantErrors, g, clientSettingsPolicy, k8sClient)
		})
	}
}

func TestClientSettingsPoliciesTargetRefGroup(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	k8sClient, err := getKubernetesClient(t)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		policySpec ngfAPIv1alpha1.ClientSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate gateway.networking.k8s.io TargetRef Group is allowed",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: gatewayGroup,
				},
			},
		},
		{
			name:       "Validate invalid.networking.k8s.io TargetRef Group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: invalidGroup,
				},
			},
		},
		{
			name:       "Validate discovery.k8s.io/v1 TargetRef Group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: discoveryGroup,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			policySpec := tt.policySpec
			policySpec.TargetRef.Name = gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName))
			clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testPolicyName),
					Namespace: defaultNamespace,
				},
				Spec: policySpec,
			}
			validateCrd(t, tt.wantErrors, g, clientSettingsPolicy, k8sClient)
		})
	}
}

func TestClientSettingsPoliciesKeepAliveTimeout(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	k8sClient, err := getKubernetesClient(t)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		policySpec ngfAPIv1alpha1.ClientSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate KeepAliveTimeout is not set",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  gatewayKind,
					Group: gatewayGroup,
				},
				KeepAlive: nil,
			},
		},
		{
			name: "Validate KeepAlive is set",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
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
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
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
			policySpec := tt.policySpec
			policySpec.TargetRef.Name = gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName))
			clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testPolicyName),
					Namespace: defaultNamespace,
				},
				Spec: policySpec,
			}
			validateCrd(t, tt.wantErrors, g, clientSettingsPolicy, k8sClient)
		})
	}
}
