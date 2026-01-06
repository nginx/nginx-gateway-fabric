package ratelimit

import (
	"fmt"
	"text/template"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var (
	tmplHTTP     = template.Must(template.New("rate limit policy http").Parse(rateLimitHTTPTemplate))
	tmplLocation = template.Must(template.New("rate limit policy location").Parse(rateLimitLocationTemplate))
)

//nolint:lll
const rateLimitHTTPTemplate = `
{{ range $r := .Rule }}
limit_req_zone {{ .Key }} zone={{ .ZoneName }}:{{ .ZoneSize }} rate={{ .Rate }};
  {{- if not $.LimitZoneOnly }}
limit_req zone={{ .ZoneName }}{{ if .Burst }} burst={{ .Burst }}{{ end }}{{ if .NoDelay }} nodelay{{ end }}{{ if .Delay }} delay={{ .Delay }}{{ end }};
  {{- end }}
{{ end }}
{{- if not .LimitZoneOnly }}
  {{- if .LogLevel }}
limit_req_log_level {{ .LogLevel }};
  {{- end }}
  {{- if .RejectCode }}
limit_req_status {{ .RejectCode }};
  {{- end }}
  {{- if .DryRun }}
limit_req_dry_run on;
  {{- end }}
{{- end }}
`

//nolint:lll
const rateLimitLocationTemplate = `
{{ range $r := .Rule }}
limit_req zone={{ .ZoneName }}{{ if .Burst }} burst={{ .Burst }}{{ end }}{{ if .NoDelay }} nodelay{{ end }}{{ if .Delay }} delay={{ .Delay }}{{ end }};
{{ end }}
{{- if .LogLevel }}
limit_req_log_level {{ .LogLevel }};
{{- end }}
{{- if .RejectCode }}
limit_req_status {{ .RejectCode }};
{{- end }}
{{- if .DryRun }}
limit_req_dry_run on;
{{- end }}
`

const (
	defaultZoneSize = "10m"
	defaultRate     = "100r/s"
	defaultKey      = "$binary_remote_addr"
)

type rateLimitSettings struct {
	LogLevel      string
	Rule          []rateLimitRule
	RejectCode    int
	DryRun        bool
	LimitZoneOnly bool
}

type rateLimitRule struct {
	ZoneName string
	ZoneSize string
	Rate     string
	Key      string
	Delay    int
	Burst    int
	NoDelay  bool
}

func getRateLimitSettings(rlp ngfAPI.RateLimitPolicy) rateLimitSettings {
	settings := rateLimitSettings{}

	if rlp.Spec.RateLimit != nil {
		if rlp.Spec.RateLimit.DryRun != nil {
			settings.DryRun = *rlp.Spec.RateLimit.DryRun
		}

		if rlp.Spec.RateLimit.LogLevel != nil {
			settings.LogLevel = string(*rlp.Spec.RateLimit.LogLevel)
		}

		if rlp.Spec.RateLimit.RejectCode != nil {
			settings.RejectCode = int(*rlp.Spec.RateLimit.RejectCode)
		}

		if rlp.Spec.RateLimit.Local != nil {
			for i, rule := range rlp.Spec.RateLimit.Local.Rules {
				rlRule := rateLimitRule{}

				rlRule.ZoneSize = defaultZoneSize
				if rule.ZoneSize != nil {
					rlRule.ZoneSize = string(*rule.ZoneSize)
				}

				if rule.Delay != nil {
					rlRule.Delay = int(*rule.Delay)
				}

				if rule.Burst != nil {
					rlRule.Burst = int(*rule.Burst)
				}

				if rule.NoDelay != nil {
					rlRule.NoDelay = *rule.NoDelay
				}

				rlRule.Rate = defaultRate
				if rule.Rate != "" {
					rlRule.Rate = string(rule.Rate)
				}

				rlRule.Key = defaultKey
				if rule.Key != "" {
					rlRule.Key = rule.Key
				}

				rlRule.ZoneName = fmt.Sprintf("%s_rl_%s_rule%d", rlp.Namespace, rlp.Name, i)

				settings.Rule = append(settings.Rule, rlRule)
			}
		}
	}

	return settings
}

// Generator generates nginx configuration based on a rate limit policy.
type Generator struct {
	policies.UnimplementedGenerator
}

// NewGenerator returns a new instance of Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateForHTTP generates policy configuration for the http block.
func (g Generator) GenerateForHTTP(pols []policies.Policy) policies.GenerateResultFiles {
	return generate(pols, tmplHTTP)
}

// GenerateForLocation generates policy configuration for a normal location block.
func (g Generator) GenerateForLocation(pols []policies.Policy, _ http.Location) policies.GenerateResultFiles {
	return generate(pols, tmplLocation)
}

func generate(pols []policies.Policy, tmpl *template.Template) policies.GenerateResultFiles {
	files := make(policies.GenerateResultFiles, 0, len(pols))

	for _, pol := range pols {
		rlp, ok := pol.(*ngfAPI.RateLimitPolicy)
		if !ok {
			continue
		}

		settings := getRateLimitSettings(*rlp)

		// Check if this is a fake HTTP context only policy
		isHTTPContextOnly := false
		if rlp.Annotations != nil {
			if val, exists := rlp.Annotations["nginx.org/internal-annotation-http-context-only"]; exists && val == "true" {
				isHTTPContextOnly = true
			}
		}

		// Set the flag in settings
		settings.LimitZoneOnly = isHTTPContextOnly

		name := fmt.Sprintf("RateLimitPolicy_%s_%s.conf", rlp.Namespace, rlp.Name)

		if isHTTPContextOnly {
			name = fmt.Sprintf("RateLimitPolicy_%s_%s_internal_http.conf", rlp.Namespace, rlp.Name)
		}

		files = append(files, policies.File{
			Name:    name,
			Content: helpers.MustExecuteTemplate(tmpl, settings),
		})
	}

	return files
}
