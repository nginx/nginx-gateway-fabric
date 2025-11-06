package waf

import (
	"fmt"
	"text/template"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const appProtectBundleFolder = "/etc/app_protect/bundles"

var tmpl = template.Must(template.New("waf policy").Parse(wafTemplate))

const wafTemplate = `
{{- if .BundlePath }}
app_protect_enable on;
app_protect_policy_file "{{ .BundlePath }}";
{{- end }}
{{- if .SecurityLogs }}
app_protect_security_log_enable on;
{{- range .SecurityLogs }}
{{- if .LogProfile }}
app_protect_security_log "{{ .LogProfile }}" {{ .Destination }};
{{- else if .LogProfileBundlePath }}
app_protect_security_log "{{ .LogProfileBundlePath }}" {{ .Destination }};
{{- end }}
{{- end }}
{{- end }}
`

// Generator generates nginx configuration based on a WAF policy.
type Generator struct {
	policies.UnimplementedGenerator
}

// NewGenerator returns a new instance of Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateForServer generates policy configuration for the server block.
func (g Generator) GenerateForServer(pols []policies.Policy, _ http.Server) policies.GenerateResultFiles {
	return generate(pols)
}

// GenerateForLocation generates policy configuration for a normal location block.
func (g Generator) GenerateForLocation(pols []policies.Policy, _ http.Location) policies.GenerateResultFiles {
	return generate(pols)
}

func generate(pols []policies.Policy) policies.GenerateResultFiles {
	files := make(policies.GenerateResultFiles, 0, len(pols))

	for _, pol := range pols {
		wp, ok := pol.(*ngfAPI.WAFPolicy)
		if !ok {
			continue
		}

		fields := map[string]any{}

		if wp.Spec.PolicySource != nil && wp.Spec.PolicySource.FileLocation != "" {
			fileLocation := wp.Spec.PolicySource.FileLocation
			bundleName := helpers.ToSafeFileName(fileLocation)
			bundlePath := fmt.Sprintf("%s/%s.tgz", appProtectBundleFolder, bundleName)
			fields["BundlePath"] = bundlePath
		}

		if len(wp.Spec.SecurityLogs) > 0 {
			securityLogs := make([]map[string]string, 0, len(wp.Spec.SecurityLogs))

			for _, secLog := range wp.Spec.SecurityLogs {
				logEntry := map[string]string{}

				if secLog.LogProfile != nil {
					logEntry["LogProfile"] = string(*secLog.LogProfile)
				}

				if secLog.LogProfileBundle != nil && secLog.LogProfileBundle.FileLocation != "" {
					bundleName := helpers.ToSafeFileName(secLog.LogProfileBundle.FileLocation)
					bundlePath := fmt.Sprintf("%s/%s.tgz", appProtectBundleFolder, bundleName)
					logEntry["LogProfileBundlePath"] = bundlePath
				}

				destination := formatSecurityLogDestination(secLog.Destination)
				logEntry["Destination"] = destination

				securityLogs = append(securityLogs, logEntry)
			}

			fields["SecurityLogs"] = securityLogs
		}

		files = append(files, policies.File{
			Name:    fmt.Sprintf("WafPolicy_%s_%s.conf", wp.Namespace, wp.Name),
			Content: helpers.MustExecuteTemplate(tmpl, fields),
		})
	}

	return files
}

func formatSecurityLogDestination(dest ngfAPI.SecurityLogDestination) string {
	switch dest.Type {
	case ngfAPI.SecurityLogDestinationTypeStderr:
		return "stderr"
	case ngfAPI.SecurityLogDestinationTypeFile:
		if dest.File != nil {
			return dest.File.Path
		}
		return "stderr"
	case ngfAPI.SecurityLogDestinationTypeSyslog:
		if dest.Syslog != nil {
			return fmt.Sprintf("syslog:server=%s", dest.Syslog.Server)
		}
		return "stderr"
	default:
		return "stderr"
	}
}
