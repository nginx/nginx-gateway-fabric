package config

import (
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var otelTemplate = gotemplate.Must(gotemplate.New("otel").Parse(otelTemplateText))

func executeTelemetry(conf dataplane.Configuration) []executeResult {
	if conf.Telemetry.Endpoint != "" {
		result := executeResult{
			dest: httpConfigFile,
			data: helpers.MustExecuteTemplate(otelTemplate, conf.Telemetry),
		}

		return []executeResult{result}
	}

	return nil
}
