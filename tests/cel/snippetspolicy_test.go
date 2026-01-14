package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

func TestSnippetsPolicyTargetRefsKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.SnippetsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate TargetRef of kind Gateway is allowed",
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRef of kind HTTPRoute is not allowed",
			wantErrors: []string{expectedTargetRefKindGatewayError},
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  httpRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRef of kind GRPCRoute is not allowed",
			wantErrors: []string{expectedTargetRefKindGatewayError},
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  grpcRouteKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate TargetRefs of kind Gateway + HTTPRoute are not allowed",
			wantErrors: []string{expectedTargetRefKindGatewayError},
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
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
			wantErrors: []string{expectedTargetRefKindGatewayError},
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for i := range tt.spec.TargetRefs {
				tt.spec.TargetRefs[i].Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
			}
			sp := &ngfAPIv1alpha1.SnippetsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, sp, k8sClient)
		})
	}
}

func TestSnippetsPolicyTargetRefsGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.SnippetsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate gateway.networking.k8s.io TargetRef Group is allowed",
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
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
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
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
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
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
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
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
			sp := &ngfAPIv1alpha1.SnippetsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, sp, k8sClient)
		})
	}
}

func TestSnippetsPolicyTargetRefsNameUniqueness(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha1.SnippetsPolicySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate single TargetRef with unique name is allowed",
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
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
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
					},
				},
			},
		},
		{
			name:       "Validate duplicate TargetRef names are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "duplicate-targetref-name",
					},
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "duplicate-targetref-name", // Same name as above
					},
				},
			},
		},
		{
			name:       "Validate three TargetRefs with one duplicate name are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "unique-targetref-name",
					},
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "duplicate-targetref-name-duplicate",
					},
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "duplicate-targetref-name-duplicate", // Same name as above
					},
				},
			},
		},
		{
			name:       "Validate multiple duplicates are not allowed",
			wantErrors: []string{expectedTargetRefNameUniqueError},
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "service-a",
					},
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "service-a", // Duplicate of first
					},
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "service-b",
					},
					{
						Kind:  gatewayKind,
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
			sp := &ngfAPIv1alpha1.SnippetsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, sp, k8sClient)
		})
	}
}

func TestSnippetsPolicyContextUniqueness(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		wantErrors []string
		spec       ngfAPIv1alpha1.SnippetsPolicySpec
	}{
		{
			name: "Validate single snippet with valid context",
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "targetref-name",
					},
				},
				Snippets: []ngfAPIv1alpha1.Snippet{
					{
						Context: ngfAPIv1alpha1.NginxContextHTTP,
						Value:   "limit_req zone=one burst=5 nodelay;",
					},
				},
			},
		},
		{
			name: "Validate multiple snippets with unique contexts",
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "targetref-name",
					},
				},
				Snippets: []ngfAPIv1alpha1.Snippet{
					{
						Context: ngfAPIv1alpha1.NginxContextMain,
						Value:   "worker_processes 4;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTPServer,
						Value:   "server_name example.com;",
					},
				},
			},
		},
		{
			name:       "Validate duplicate contexts are not allowed",
			wantErrors: []string{expectedSnippetsContextError},
			spec: ngfAPIv1alpha1.SnippetsPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReference{
					{
						Kind:  gatewayKind,
						Group: gatewayGroup,
						Name:  "targetref-name",
					},
				},
				Snippets: []ngfAPIv1alpha1.Snippet{
					{
						Context: ngfAPIv1alpha1.NginxContextHTTP,
						Value:   "limit_req zone=one burst=5 nodelay;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTP,
						Value:   "sendfile on;",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			snippetsPolicy := &ngfAPIv1alpha1.SnippetsPolicy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, snippetsPolicy, k8sClient)
		})
	}
}
