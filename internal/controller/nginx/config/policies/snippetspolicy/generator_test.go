package snippetspolicy_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/snippetspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestGenerator(t *testing.T) {
	g := &snippetspolicy.Generator{}

	policy := &v1alpha1.SnippetsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy-1",
			Namespace: "default",
		},
		Spec: v1alpha1.SnippetsPolicySpec{
			TargetRef: v1alpha1.SnippetsPolicyTargetRef{
				LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
					Group: gatewayv1.GroupName,
					Kind:  kinds.Gateway,
					Name:  "gateway-1",
				},
			},
			Snippets: []v1alpha1.Snippet{
				{
					Context: v1alpha1.NginxContextMain,
					Value:   "worker_processes 1;",
				},
				{
					Context: v1alpha1.NginxContextHTTP,
					Value:   "log_format custom '...';",
				},
				{
					Context: v1alpha1.NginxContextHTTPServer,
					Value:   "client_max_body_size 10m;",
				},
			},
		},
	}

	pols := []policies.Policy{policy}

	t.Run("GenerateForMain", func(t *testing.T) {
		gWithT := NewWithT(t)
		files := g.GenerateForMain(pols)
		gWithT.Expect(files).To(HaveLen(1))
		gWithT.Expect(files[0].Name).To(Equal("includes/policy/default-gateway-1/SnippetsPolicy_main_policy-1.conf"))
		gWithT.Expect(string(files[0].Content)).To(ContainSubstring("worker_processes 1;"))
	})

	t.Run("GenerateForHTTP", func(t *testing.T) {
		gWithT := NewWithT(t)
		files := g.GenerateForHTTP(pols)
		gWithT.Expect(files).To(HaveLen(1))
		gWithT.Expect(files[0].Name).To(Equal("includes/policy/default-gateway-1/SnippetsPolicy_http_policy-1.conf"))
		gWithT.Expect(string(files[0].Content)).To(ContainSubstring("log_format custom '...';"))
	})

	t.Run("GenerateForServer", func(t *testing.T) {
		gWithT := NewWithT(t)
		server := http.Server{
			Listen: "80",
		}
		files := g.GenerateForServer(pols, server)
		gWithT.Expect(files).To(HaveLen(1))
		gWithT.Expect(files[0].Name).To(Equal("includes/policy/default-gateway-1/80/SnippetsPolicy_server_policy-1.conf"))
		gWithT.Expect(string(files[0].Content)).To(ContainSubstring("client_max_body_size 10m;"))
	})
}
