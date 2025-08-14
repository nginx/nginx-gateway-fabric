package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

// UpstreamSettingsPolicy validation errors.
const (
	expectedTargetRefKindServiceError = `TargetRefs Kind must be: Service`
	expectedTargetRefGroupCoreError   = `TargetRefs Group must be core`
	expectedTargetRefNameUniqueError  = `TargetRef Name must be unique`
)

const (
	serviceKind = "Service"
	coreGroup   = "core"
	emptyGroup  = ""
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
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name: "Validate multiple TargetRefs of kind Service are allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name:       "Validate TargetRef of kind Gateway is not allowed",
			wantErrors: []string{expectedTargetRefKindServiceError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name:       "Validate TargetRef of kind HTTPRoute is not allowed",
			wantErrors: []string{expectedTargetRefKindServiceError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name:       "Validate invalid TargetRef Kind is not allowed",
			wantErrors: []string{expectedTargetRefKindServiceError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  invalidKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name:       "Validate mixed TargetRef kinds - one valid, one invalid",
			wantErrors: []string{expectedTargetRefKindServiceError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
					{
						Kind:  gatewayKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name: "Validate TargetRef with empty group is allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: emptyGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name: "Validate multiple TargetRefs with valid groups are allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
					{
						Kind:  serviceKind,
						Group: emptyGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name:       "Validate TargetRef with gateway group is not allowed",
			wantErrors: []string{expectedTargetRefGroupCoreError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: gatewayGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name:       "Validate TargetRef with invalid group is not allowed",
			wantErrors: []string{expectedTargetRefGroupCoreError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: invalidGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name: "Validate mixed TargetRef groups with valid core group passes due to current CEL rule",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
					{
						Kind:  serviceKind,
						Group: gatewayGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name:       "Validate all TargetRef groups are invalid",
			wantErrors: []string{expectedTargetRefGroupCoreError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: gatewayGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
					{
						Kind:  serviceKind,
						Group: invalidGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name: "Validate multiple TargetRefs with unique names are allowed",
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
					{
						Kind:  serviceKind,
						Group: coreGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
					{
						Kind:  serviceKind,
						Group: emptyGroup,
						Name:  gatewayv1alpha2.ObjectName(uniqueResourceName(testTargetRefName)),
					},
				},
			},
		},
		{
			name:       "Validate duplicate TargetRef names are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
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
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
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
				TargetRefs: []gatewayv1alpha2.LocalPolicyTargetReference{
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
