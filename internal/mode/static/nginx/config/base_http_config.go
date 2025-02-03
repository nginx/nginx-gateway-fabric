package config

import (
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var baseHTTPTemplate = gotemplate.Must(gotemplate.New("baseHttp").Parse(baseHTTPTemplateText))

type httpConfig struct {
	Includes []shared.Include
	HTTP2    bool
}

func executeBaseHTTPConfig(conf dataplane.Configuration) []executeResult {
	includes := createIncludesFromSnippets(conf.BaseHTTPConfig.Snippets)

	hc := httpConfig{
		HTTP2:    conf.BaseHTTPConfig.HTTP2,
		Includes: includes,
	}

	results := make([]executeResult, 0, len(includes)+1)
	results = append(results, executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(baseHTTPTemplate, hc),
	})
	results = append(results, createIncludeExecuteResults(includes)...)

	return results
}
