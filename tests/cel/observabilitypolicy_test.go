package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
)

func TestObservabilityPoliciesTargetRefKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.ObservabilityPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate TargetRef of kind HTTPRoute is allowed",
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "Validate TargetRef of kind GRPCRoute is allowed",
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate Invalid TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefMustBeHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  invalidKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TCPRoute TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefMustBeHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  tcpRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRef of kind Gateway is allowed",
			wantErrors: []string{expectedTargetRefMustBeHTTPRouteOrGrpcRouteError},
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name: "Validate ObservabilityPolicy is applied when one TargetRef is valid and another is invalid",
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec

			for i := range spec.TargetRefs {
				spec.TargetRefs[i].Name = gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName))
			}

			observabilityPolicy := &ngfAPIv1alpha2.ObservabilityPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, observabilityPolicy, k8sClient)
		})
	}
}

func TestObservabilityPoliciesTargetRefGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.ObservabilityPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate gateway.networking.k8s.io TargetRef Group is allowed",
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate invalid.networking.k8s.io TargetRef Group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: invalidGroup,
					},
				},
			},
		},
		{
			name:       "Validate discovery.k8s.io/v1 TargetRef Group is not allowed",
			wantErrors: []string{expectedTargetRefGroupError},
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: discoveryGroup,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec

			for i := range spec.TargetRefs {
				spec.TargetRefs[i].Name = gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName))
			}

			observabilityPolicy := &ngfAPIv1alpha2.ObservabilityPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, observabilityPolicy, k8sClient)
		})
	}
}

func TestObservabilityPoliciesTargetRefKindAndNameCombo(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.ObservabilityPolicySpec
		name       string
		wantErrors []string
	}{
		{
			// This test is not throwing an error like we expect it to...
			name:       "Validate TargetRef Kind and Name combination is unique",
			wantErrors: []string{expectedTargetRefKindAndNameComboMustBeUnique},
			spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Name:  gatewayv1alpha2.ObjectName(testTargetRefName),
						Group: gatewayGroup,
					},
					{
						Kind:  httpRouteKind,
						Name:  gatewayv1alpha2.ObjectName(testTargetRefName),
						Group: gatewayGroup,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec

			observabilityPolicy := &ngfAPIv1alpha2.ObservabilityPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, observabilityPolicy, k8sClient)
		})
	}
}
