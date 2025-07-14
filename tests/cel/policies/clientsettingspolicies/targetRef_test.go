package clientsettingspolicies

import (
	"testing"

	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func TestClientSettingsPoliciesTargetRef(t *testing.T) {
	allowedKinds := map[string]bool{
		"Gateway":   true,
		"HTTPRoute": true,
		"GRPCRoute": true,
	}

	testInvalidTargetRefs(t, allowedKinds)
	testValidTargetRefs(t, allowedKinds)
}

func testInvalidTargetRefs(t *testing.T, allowedKinds map[string]bool) {
	t.Helper()

	tests := []struct {
		name       string
		wantErrors string
		targetRef  gatewayv1alpha2.LocalPolicyTargetReference
	}{
		{
			name:       "Validate TargetRef is of an allowed kind",
			wantErrors: "TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute'",
			targetRef: gatewayv1alpha2.LocalPolicyTargetReference{
				Kind:  "InvalidKind",
				Group: "gateway.networking.k8s.io",
			},
		},
		{
			name:       "Validate TargetRef is of an allowed kind",
			wantErrors: "TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute'",
			targetRef: gatewayv1alpha2.LocalPolicyTargetReference{
				Kind:  "TCPRoute",
				Group: "gateway.networking.k8s.io",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := allowedKinds[string(tt.targetRef.Kind)]; !ok {
				gotError := "TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute'"

				if tt.wantErrors != gotError {
					t.Errorf("Test %s failed: got error %q, want %q", tt.name, gotError, tt.wantErrors)
				}
			}
		})
	}
}

func testValidTargetRefs(t *testing.T, allowedKinds map[string]bool) {
	t.Helper()

	tests := []struct {
		name       string
		wantErrors string
		targetRef  gatewayv1alpha2.LocalPolicyTargetReference
	}{
		{
			name:       "Validate TargetRef is of an allowed kind",
			wantErrors: "TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute'",
			targetRef: gatewayv1alpha2.LocalPolicyTargetReference{
				Kind:  "Gateway",
				Group: "gateway.networking.k8s.io",
			},
		},
		{
			name:       "Validate TargetRef is of an allowed kind",
			wantErrors: "TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute'",
			targetRef: gatewayv1alpha2.LocalPolicyTargetReference{
				Kind:  "HTTPRoute",
				Group: "gateway.networking.k8s.io",
			},
		},
		{
			name:       "Validate TargetRef is of an allowed kind",
			wantErrors: "TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute'",
			targetRef: gatewayv1alpha2.LocalPolicyTargetReference{
				Kind:  "GRPCRoute",
				Group: "gateway.networking.k8s.io",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := allowedKinds[string(tt.targetRef.Kind)]; !ok {
				gotError := "TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute'"

				if tt.wantErrors == gotError {
					t.Errorf("Test %s failed: got error %q, want %q", tt.name, gotError, tt.wantErrors)
				}
			}
		})
	}
}
