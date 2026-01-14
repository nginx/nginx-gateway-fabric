package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

func TestSnippetsFilterValidation(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		wantErrors []string
		spec       ngfAPIv1alpha1.SnippetsFilterSpec
	}{
		{
			name: "Validate single snippet with valid context",
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
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
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
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
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
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
			snippetsFilter := &ngfAPIv1alpha1.SnippetsFilter{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      uniqueResourceName(testResourceName),
					Namespace: defaultNamespace,
				},
				Spec: tt.spec,
			}
			validateCrd(t, tt.wantErrors, snippetsFilter, k8sClient)
		})
	}
}
