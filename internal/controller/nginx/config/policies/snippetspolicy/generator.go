package snippetspolicy

import (
	"fmt"

	"github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
)

const (
	mainTemplate = `
# SnippetsPolicy %s main context
%s
`
	httpTemplate = `
# SnippetsPolicy %s http context
%s
`
	serverTemplate = `
# SnippetsPolicy %s server context
%s
`
	locationTemplate = `
# SnippetsPolicy %s location context
%s
`
)

// Generator generates NGINX configuration snippets for SnippetsPolicies.
type Generator struct {
	policies.UnimplementedGenerator
}

// NewGenerator returns a new instance of Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateForMain generates policy configuration for the main block.
func (g *Generator) GenerateForMain(pols []policies.Policy) policies.GenerateResultFiles {
	return g.generate(pols, v1alpha1.NginxContextMain)
}

// GenerateForHTTP generates policy configuration for the http block.
func (g *Generator) GenerateForHTTP(pols []policies.Policy) policies.GenerateResultFiles {
	return g.generate(pols, v1alpha1.NginxContextHTTP)
}

// GenerateForServer generates policy configuration for the server block.
func (g *Generator) GenerateForServer(pols []policies.Policy, _ http.Server) policies.GenerateResultFiles {
	return g.generate(pols, v1alpha1.NginxContextHTTPServer)
}

// GenerateForLocation generates policy configuration for the location block.
func (g *Generator) GenerateForLocation(pols []policies.Policy, _ http.Location) policies.GenerateResultFiles {
	return g.generate(pols, v1alpha1.NginxContextHTTPServerLocation)
}

// GenerateForInternalLocation generates policy configuration for an internal location block.
func (g *Generator) GenerateForInternalLocation(pols []policies.Policy) policies.GenerateResultFiles {
	return g.generate(pols, v1alpha1.NginxContextHTTPServerLocation)
}

func (g *Generator) generate(
	pols []policies.Policy,
	context v1alpha1.NginxContext,
) policies.GenerateResultFiles {
	var files policies.GenerateResultFiles

	for _, policy := range pols {
		sp, ok := policy.(*v1alpha1.SnippetsPolicy)
		if !ok {
			continue
		}

		for _, snippet := range sp.Spec.Snippets {
			if snippet.Context != context {
				continue
			}

			policyNsName := fmt.Sprintf("%s/%s", sp.GetNamespace(), sp.GetName())
			policyFileID := fmt.Sprintf("%s-%s", sp.GetNamespace(), sp.GetName())

			// Build content and filename
			var content string
			var filename string

			switch context {
			case v1alpha1.NginxContextMain:
				content = fmt.Sprintf(mainTemplate, policyNsName, snippet.Value)
				filename = fmt.Sprintf("SnippetsPolicy_main_%s.conf", policyFileID)
			case v1alpha1.NginxContextHTTP:
				content = fmt.Sprintf(httpTemplate, policyNsName, snippet.Value)
				filename = fmt.Sprintf("SnippetsPolicy_http_%s.conf", policyFileID)
			case v1alpha1.NginxContextHTTPServer:
				content = fmt.Sprintf(serverTemplate, policyNsName, snippet.Value)
				filename = fmt.Sprintf("SnippetsPolicy_server_%s.conf", policyFileID)
			case v1alpha1.NginxContextHTTPServerLocation:
				content = fmt.Sprintf(locationTemplate, policyNsName, snippet.Value)
				filename = fmt.Sprintf("SnippetsPolicy_location_%s.conf", policyFileID)
			}

			files = append(files, policies.File{
				Name:    filename,
				Content: []byte(content),
			})
		}
	}
	return files
}
