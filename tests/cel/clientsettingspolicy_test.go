package cel

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	ExpectedTargetRefKindError  = `TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute`
	ExpectedTargetRefGroupError = `TargetRef Group must be gateway.networking.k8s.io.`
)

const (
	PolicyNameFormat = "test-policy-%d"
	TargetRefFormat  = "targetRef-name-%d"
)

func TestClientSettingsPoliciesTargetRefKind(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			validateClientSettingsPolicy(t, tt)
		})
	}
}

func TestClientSettingsPoliciesTargetRefGroup(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			validateClientSettingsPolicy(t, tt)
		})
	}
}

func validateClientSettingsPolicy(t *testing.T, tt struct {
	policySpec ngfAPIv1alpha1.ClientSettingsPolicySpec
	name       string
	wantErrors []string
},
) {
	t.Helper()
	g := NewWithT(t)

	// Get Kubernetes client from test framework
	// This should be set up by your test framework to connect to a real cluster
	k8sClient := getKubernetesClient(t)

	policySpec := tt.policySpec
	policySpec.TargetRef.Name = gatewayv1alpha2.ObjectName(fmt.Sprintf(TargetRefFormat, time.Now().UnixNano()))
	policyName := fmt.Sprintf(PolicyNameFormat, time.Now().UnixNano())

	clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      policyName,
			Namespace: "default",
		},
		Spec: policySpec,
	}

	err := k8sClient.Create(context.Background(), clientSettingsPolicy)

	// Clean up after test
	defer func() {
		_ = k8sClient.Delete(context.Background(), clientSettingsPolicy)
	}()

	// Check if we expected errors
	if len(tt.wantErrors) == 0 {
		if err != nil {
			g.Expect(err).ToNot(HaveOccurred())
		}
		return
	}

	// We expected errors - validation should have failed
	if err == nil {
		g.Expect(err).To(HaveOccurred())
		return
	}

	// Check that we got the expected error messages
	for _, wantError := range tt.wantErrors {
		g.Expect(err.Error()).To(ContainSubstring(wantError), "Expected error '%s' not found in: %s", wantError, err.Error())
	}
}

// getKubernetesClient returns a client connected to a real Kubernetes cluster.
func getKubernetesClient(t *testing.T) client.Client {
	t.Helper()
	// Use controller-runtime to get cluster connection
	k8sConfig, err := controllerruntime.GetConfig()
	if err != nil {
		t.Skipf("Cannot connect to Kubernetes cluster: %v", err)
	}

	// Set up scheme with NGF types
	scheme := runtime.NewScheme()
	if err := ngfAPIv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add NGF schema: %v", err)
	}

	// Create client
	k8sClient, err := client.New(k8sConfig, client.Options{Scheme: scheme})
	if err != nil {
		t.Skipf("Cannot create k8s client: %v", err)
	}

	return k8sClient
}
