package cel

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
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
			validateClientSettingsPolicy(t, tt, g, k8sClient)
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
			validateClientSettingsPolicy(t, tt, g, k8sClient)
		})
	}
}

func validateClientSettingsPolicy(t *testing.T, tt struct {
	policySpec ngfAPIv1alpha1.ClientSettingsPolicySpec
	name       string
	wantErrors []string
}, g *WithT, k8sClient client.Client,
) {
	t.Helper()

	policySpec := tt.policySpec
	policySpec.TargetRef.Name = gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName))
	policyName := uniqueResourceName(testPolicyName)

	clientSettingsPolicy := &ngfAPIv1alpha1.ClientSettingsPolicy{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      policyName,
			Namespace: defaultNamespace,
		},
		Spec: policySpec,
	}
	timeoutConfig := framework.DefaultTimeoutConfig()
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.KubernetesClientTimeout)
	err := k8sClient.Create(ctx, clientSettingsPolicy)
	defer cancel()

	// Clean up after test
	defer func() {
		_ = k8sClient.Delete(context.Background(), clientSettingsPolicy)
	}()

	if len(tt.wantErrors) == 0 {
		g.Expect(err).ToNot(HaveOccurred())
	} else {
		g.Expect(err).To(HaveOccurred())
		for _, wantError := range tt.wantErrors {
			g.Expect(err.Error()).To(ContainSubstring(wantError), "Expected error '%s' not found in: %s", wantError, err.Error())
		}
	}
}
