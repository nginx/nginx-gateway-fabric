package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

func TestSnippetsFilterValidation_Snippets(t *testing.T) {
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
			name:       "Validate empty snippet value is not allowed",
			wantErrors: []string{"Invalid value: \"\""},
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
				Snippets: []ngfAPIv1alpha1.Snippet{
					{
						Context: ngfAPIv1alpha1.NginxContextHTTP,
						Value:   "",
					},
				},
			},
		},
		{
			name:       "Validate no snippets is not allowed",
			wantErrors: []string{"Invalid value: 0"},
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
				Snippets: []ngfAPIv1alpha1.Snippet{},
			},
		},
		{
			name: "Validate maximum number of snippets is allowed",
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
				Snippets: []ngfAPIv1alpha1.Snippet{
					{
						Context: ngfAPIv1alpha1.NginxContextMain,
						Value:   "worker_processes 4;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTP,
						Value:   "limit_req zone=one burst=5 nodelay;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTPServer,
						Value:   "server_name example.com;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTPServerLocation,
						Value:   "deny all;",
					},
				},
			},
		},
		{
			name:       "Validate more than maximum number of snippets is not allowed",
			wantErrors: []string{"Too many: 5: must have at most 4 items"},
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
				Snippets: []ngfAPIv1alpha1.Snippet{
					{
						Context: ngfAPIv1alpha1.NginxContextMain,
						Value:   "worker_processes 4;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTP,
						Value:   "limit_req zone=one burst=5 nodelay;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTPServer,
						Value:   "server_name example.com;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTPServerLocation,
						Value:   "deny all;",
					},
					{
						Context: ngfAPIv1alpha1.NginxContextHTTPServerLocation,
						Value:   "extra configuration",
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

func TestSnippetsFilterValidation_Context(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		wantErrors []string
		spec       ngfAPIv1alpha1.SnippetsFilterSpec
	}{
		{
			name:       "Validate empty context is not allowed",
			wantErrors: []string{"Unsupported value: \"\""},
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
				Snippets: []ngfAPIv1alpha1.Snippet{
					{
						Context: "",
						Value:   "empty context configuration",
					},
				},
			},
		},
		{
			name:       "Validate incorrect context is not allowed",
			wantErrors: []string{"Unsupported value: \"invalid.context\""},
			spec: ngfAPIv1alpha1.SnippetsFilterSpec{
				Snippets: []ngfAPIv1alpha1.Snippet{
					{
						Context: "invalid.context",
						Value:   "invalid context configuration",
					},
				},
			},
		},
		{
			name:       "Validate duplicate contexts are not allowed",
			wantErrors: []string{"Only one snippet allowed per context"},
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
