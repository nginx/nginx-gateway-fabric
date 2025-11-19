package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestUpstreamSettingsPolicyTargetRefKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.UpstreamSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate TargetRef of kind Service is allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
				},
			},
		},
		{
			name: "Validate multiple TargetRefs of kind Service are allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRef of kind Gateway is not allowed",
			wantErrors: []string{expectedTargetRefKindServiceError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: coreGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRef of kind HTTPRoute is not allowed",
			wantErrors: []string{expectedTargetRefKindServiceError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: coreGroup,
					},
				},
			},
		},
		{
			name:       "Validate invalid TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefKindServiceError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  invalidKind,
						Group: coreGroup,
					},
				},
			},
		},
		{
			name:       "Validate mixed TargetRef kinds - one valid, one invalid",
			wantErrors: []string{expectedTargetRefKindServiceError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
					{
						Kind:  grpcRouteKind,
						Group: coreGroup,
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

			upstreamSettingsPolicy := &ngfAPIv1alpha1.UpstreamSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, upstreamSettingsPolicy, k8sClient)
		})
	}
}

func TestUpstreamSettingsPolicyTargetRefGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.UpstreamSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate TargetRef with core group is allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
				},
			},
		},
		{
			name: "Validate TargetRef with empty group is allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: emptyGroup,
					},
				},
			},
		},
		{
			name: "Validate multiple TargetRefs with valid groups are allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
					{
						Kind:  serviceKind,
						Group: emptyGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRef with gateway group is not allowed",
			wantErrors: []string{expectedTargetRefGroupCoreError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRef with invalid group is not allowed",
			wantErrors: []string{expectedTargetRefGroupCoreError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: invalidGroup,
					},
				},
			},
		},
		{
			name: "Validate mixed TargetRef groups with valid core group passes due to current CEL rule",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
					{
						Kind:  serviceKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate all TargetRef groups are invalid",
			wantErrors: []string{expectedTargetRefGroupCoreError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: gatewayGroup,
					},
					{
						Kind:  serviceKind,
						Group: invalidGroup,
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

			upstreamSettingsPolicy := &ngfAPIv1alpha1.UpstreamSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, upstreamSettingsPolicy, k8sClient)
		})
	}
}

func TestUpstreamSettingsPolicyTargetRefNameUniqueness(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.UpstreamSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate single TargetRef with unique name is allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
				},
			},
		},
		{
			name: "Validate multiple TargetRefs with unique names are allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
					{
						Kind:  serviceKind,
						Group: emptyGroup,
					},
				},
			},
		},
		{
			name:       "Validate duplicate TargetRef names are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  "duplicate-service-name",
					},
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  "duplicate-service-name", // Same name as above
					},
				},
			},
		},
		{
			name:       "Validate three TargetRefs with one duplicate name are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  "unique-service-1",
					},
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  "duplicate-service-name",
					},
					{
						Kind:  serviceKind,
						Group: emptyGroup,
						Name:  "duplicate-service-name", // Same name as above
					},
				},
			},
		},
		{
			name:       "Validate multiple duplicates are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  "service-a",
					},
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  "service-a", // Duplicate of first
					},
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  "service-b",
					},
					{
						Kind:  serviceKind,
						Group: emptyGroup,
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

			upstreamSettingsPolicy := &ngfAPIv1alpha1.UpstreamSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, upstreamSettingsPolicy, k8sClient)
		})
	}
}

func TestUpstreamSettingsPolicy_LoadBalancing(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.UpstreamSettingsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "when load balancing method is hash, hash key is required, error expected",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
				},
				LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeHash),
			},
			wantErrors: []string{expectedHashKeyLoadBalancingTypeError},
		},
		{
			name: "when load balancing method is hash consistent, hash key is required, error expected",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
				},
				LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeHashConsistent),
			},
			wantErrors: []string{expectedHashKeyLoadBalancingTypeError},
		},
		{
			name: "specify load balancing method as hash and set the hash key, no error expected",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
				},
				LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeHash),
				HashKey:             helpers.GetPointer(ngfAPIv1alpha1.HashMethodKey("$upstream_connect_time")),
			},
		},
		{
			name: "specify load balancing method as hash consistent and set the hash key, no error expected",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
					},
				},
				LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeHashConsistent),
				HashKey:             helpers.GetPointer(ngfAPIv1alpha1.HashMethodKey("$upstream_bytes_sent")),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for i := range tt.spec.TargetRefs {
				tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			}

			upstreamSettingsPolicy := &ngfAPIv1alpha1.UpstreamSettingsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, upstreamSettingsPolicy, k8sClient)
		})
	}
}
