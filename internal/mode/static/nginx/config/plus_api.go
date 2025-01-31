package config

import (
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var plusAPITemplate = gotemplate.Must(gotemplate.New("plusAPI").Parse(plusAPITemplateText))

func executePlusAPI(conf dataplane.Configuration) []executeResult {
	var result executeResult
	// if AllowedAddresses is empty, it means that we are not running on nginx plus, and we don't want this generated
	if conf.NginxPlus.AllowedAddresses != nil {
		result = executeResult{
			dest: nginxPlusConfigFile,
			data: helpers.MustExecuteTemplate(plusAPITemplate, conf.NginxPlus),
		}
	} else {
		return nil
	}

	return []executeResult{result}
}
