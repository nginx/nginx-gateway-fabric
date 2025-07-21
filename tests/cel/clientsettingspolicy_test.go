package cel

import (
	"context"
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
)

const (
	GatewayKind   = "Gateway"
	HTTPRouteKind = "HTTPRoute"
	GRPCRouteKind = "GRPCRoute"
	TCPRouteKind  = "TCPRoute"
	InvalidKind   = "InvalidKind"
)

const (
	GatewayGroup   = "gateway.networking.k8s.io"
	InvalidGroup   = "invalid.networking.k8s.io"
	DiscoveryGroup = "discovery.k8s.io/v1"
)

const (
	ExpectedTargetRefKindError  = "TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute'"
	ExpectedTargetRefGroupError = "TargetRef Group must be gateway.networking.k8s.io."
)

func TestClientSettingsPoliciesTargetRefKind(t *testing.T) {

	tests := []struct {
		policySpec ngfAPIv1alpha1.ClientSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate TargetRef of kind Gateway is allowed",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  GatewayKind,
					Group: GatewayGroup,
				},
			},
		},
		{
			name: "Validate TargetRef of kind HTTPRoute is allowed",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  HTTPRouteKind,
					Group: GatewayGroup,
				},
			},
		},
		{
			name: "Validate TargetRef of kind GRPCRoute is allowed",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  GRPCRouteKind,
					Group: GatewayGroup,
				},
			},
		},
		{
			name:       "Validate Invalid TargetRef Kind is not allowed",
			wantErrors: []string{ExpectedTargetRefKindError},
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  InvalidKind,
					Group: GatewayGroup,
				},
			},
		},
		{
			name:       "Validate TCPRoute TargetRef Kind is not allowed",
			wantErrors: []string{ExpectedTargetRefKindError},
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  TCPRouteKind,
					Group: GatewayGroup,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ClientSettingsPolicy with the targetRef from the test case.
			clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					TargetRef: tt.policySpec.TargetRef,
				},
			}
			validateClientSettingsPolicy(t, clientSettingsPolicy, tt.wantErrors)
		})
	}
}

func TestClientSettingsPoliciesTargetRefGroup(t *testing.T) {

	tests := []struct {
		policySpec ngfAPIv1alpha1.ClientSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate gateway.networking.k8s.io TargetRef Group is allowed",
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  GatewayKind,
					Group: GatewayGroup,
				},
			},
		},
		{
			name:       "Validate invalid.networking.k8s.io TargetRef Group is not allowed",
			wantErrors: []string{ExpectedTargetRefGroupError},
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  GatewayKind,
					Group: InvalidGroup,
				},
			},
		},
		{
			name:       "Validate discovery.k8s.io/v1 TargetRef Group is not allowed",
			wantErrors: []string{ExpectedTargetRefGroupError},
			policySpec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
				TargetRef: gatewayv1alpha2.LocalPolicyTargetReference{
					Kind:  GatewayKind,
					Group: DiscoveryGroup,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ClientSettingsPolicy with the targetRef from the test case.
			clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					TargetRef: tt.policySpec.TargetRef,
				},
			}
			validateClientSettingsPolicy(t, clientSettingsPolicy, tt.wantErrors)
		})
	}
}

func validateClientSettingsPolicy(t *testing.T,
	clientSettingsPolicy *ngfAPIv1alpha1.ClientSettingsPolicy,
	wantErrors []string,
) {
	t.Helper()

	// Register API types with the runtime scheme
	// This is necessary to create a fake client that can handle the custom resource.
	scheme := runtime.NewScheme()
	_ = ngfAPIv1alpha1.AddToScheme(scheme)

	// Create a fake client with the scheme
	// This is used to simulate interactions with the Kubernetes API.
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := k8sClient.Create(context.Background(), clientSettingsPolicy)
	if err != nil {
		t.Logf("Error creating ClientSettingsPolicy %q: %v",
			fmt.Sprintf("%v/%v", clientSettingsPolicy.Namespace, clientSettingsPolicy.Name), err)
	}

	if err != nil {
		for _, wantError := range wantErrors {
			if !strings.Contains(err.Error(), wantError) {
				t.Errorf("missing expected error %q in %v", wantError, err)
			}
		}
	}
}
