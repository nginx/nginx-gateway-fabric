package config

import (
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var plusAPITemplate = gotemplate.Must(gotemplate.New("plusAPI").Parse(plusAPITemplateText))

func executePlusAPI(conf dataplane.Configuration) []executeResult {
	result := executeResult{
		dest: nginxPlusConfigFile,
		data: helpers.MustExecuteTemplate(plusAPITemplate, conf.NginxPlus),
	}

	return []executeResult{result}
}
