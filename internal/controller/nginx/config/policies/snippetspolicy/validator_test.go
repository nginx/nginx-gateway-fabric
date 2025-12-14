package snippetspolicy_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/snippetspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

type policyModFunc func(policy *ngfAPI.SnippetsPolicy) *ngfAPI.SnippetsPolicy

func createValidPolicy() *ngfAPI.SnippetsPolicy {
	return &ngfAPI.SnippetsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-policy",
		},
		Spec: ngfAPI.SnippetsPolicySpec{
			TargetRef: ngfAPI.SnippetsPolicyTargetRef{
				LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
					Group: gatewayv1.GroupName,
					Kind:  kinds.Gateway,
					Name:  "test-gateway",
				},
			},
			Snippets: []ngfAPI.Snippet{
				{
					Context: ngfAPI.NginxContextMain,
					Value:   "worker_processes 1;",
				},
				{
					Context: ngfAPI.NginxContextHTTP,
					Value:   "log_format custom '...';",
				},
			},
		},
		Status: gatewayv1.PolicyStatus{},
	}
}

func createModifiedPolicy(mod policyModFunc) *ngfAPI.SnippetsPolicy {
	return mod(createValidPolicy())
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		policy        *ngfAPI.SnippetsPolicy
		expConditions []conditions.Condition
	}{
		{
			name:          "valid policy",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
		{
			name: "invalid target ref - unsupported group",
			policy: createModifiedPolicy(func(p *ngfAPI.SnippetsPolicy) *ngfAPI.SnippetsPolicy {
				p.Spec.TargetRef.Group = "unsupported.group"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRef.group: Unsupported value: \"unsupported.group\": supported values: \"gateway.networking.k8s.io\""),
			},
		},
		{
			name: "invalid target ref - unsupported kind",
			policy: createModifiedPolicy(func(p *ngfAPI.SnippetsPolicy) *ngfAPI.SnippetsPolicy {
				p.Spec.TargetRef.Kind = "UnsupportedKind"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRef.kind: Unsupported value: \"UnsupportedKind\": supported values: \"Gateway\""),
			},
		},
		{
			name: "duplicate context",
			policy: createModifiedPolicy(func(p *ngfAPI.SnippetsPolicy) *ngfAPI.SnippetsPolicy {
				p.Spec.Snippets = append(p.Spec.Snippets, ngfAPI.Snippet{
					Context: ngfAPI.NginxContextMain,
					Value:   "another snippet;",
				})
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("duplicate context \"main\""),
			},
		},
		{
			name: "unsupported context",
			policy: createModifiedPolicy(func(p *ngfAPI.SnippetsPolicy) *ngfAPI.SnippetsPolicy {
				p.Spec.Snippets = []ngfAPI.Snippet{
					{
						Context: ngfAPI.NginxContextHTTPServerLocation,
						Value:   "location snippet",
					},
				}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("context \"http.server.location\" is not supported in SnippetsPolicy"),
			},
		},
		{
			name: "snippet too large",
			policy: createModifiedPolicy(func(p *ngfAPI.SnippetsPolicy) *ngfAPI.SnippetsPolicy {
				p.Spec.Snippets = []ngfAPI.Snippet{
					{
						Context: ngfAPI.NginxContextMain,
						Value:   string(make([]byte, 2049)),
					},
				}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("snippet value for context \"main\" exceeds maximum size of 2048 bytes"),
			},
		},
	}

	v := snippetspolicy.NewValidator()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := v.Validate(test.policy)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	v := snippetspolicy.NewValidator()

	g.Expect(v.ValidateGlobalSettings(nil, nil)).To(BeNil())
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	v := snippetspolicy.NewValidator()

	g.Expect(v.Conflicts(nil, nil)).To(BeFalse())
}
