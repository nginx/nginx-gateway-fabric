package ratelimit

import (
	"fmt"
	"text/template"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

// rateLimitHTTPTemplate generates only the limit_req_zone directive at the http context.
const rateLimitHTTPTemplate = `
{{ range $r := .Rule }}
limit_req_zone {{ .Key }} zone={{ .ZoneName }}:{{ .ZoneSize }} rate={{ .Rate }};
{{ end }}
`

//nolint:lll
const rateLimitReqTemplate = `
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

var (
	tmplHTTP     = template.Must(template.New("rate limit policy http").Parse(rateLimitHTTPTemplate))
	tmplServer   = template.Must(template.New("rate limit policy server").Parse(rateLimitReqTemplate))
	tmplLocation = template.Must(template.New("rate limit policy location").Parse(rateLimitReqTemplate))
)

const (
	// fileNamePrefix is the prefix for all generated rate limit policy config file names.
	fileNamePrefix = "RateLimitPolicy"

	fileNameSuffixGateway  = "gateway"
	fileNameSuffixHTTP     = "internal_http"
	fileNameSuffixServer   = "gateway_server"
	fileNameSuffixLocation = "route"

	// defaultZoneSize is the default size of the shared memory zone in the limit_req_zone NGINX directive.
	defaultZoneSize = "10m"
	// defaultRate is the default request rate in the limit_req_zone NGINX directive.
	defaultRate = "100r/s"
	// defaultKey is the default key in the limit_req_zone NGINX directive.
	defaultKey = "$binary_remote_addr"
)

// rateLimitSettings represents the settings for a rate limit policy.
type rateLimitSettings struct {
	// LogLevel is the log level for cases when the server refuses to process requests due to
	// rate exceeding, or delays request processing.
	LogLevel string
	// Rule is the list of rate limit rules.
	Rule []rateLimitRule
	// RejectCode is the status code to return in response to rejected requests.
	RejectCode int
	// DryRun enables the dry run mode, where the rate limit is not actually applied, but the number
	// of excessive requests is accounted as usual in the shared memory zone.
	DryRun bool
}

// rateLimitRule represents a single rate limit rule.
type rateLimitRule struct {
	// ZoneName is the name of the shared memory zone.
	ZoneName string
	// ZoneSize is the size of the shared memory zone.
	ZoneSize string
	// Rate is the request rate.
	Rate string
	// Key is the key to use for rate limiting.
	Key string
	// Delay is the delay for excessive requests.
	Delay int
	// Burst is the maximum number of excessive requests that can be delayed.
	Burst int
	// NoDelay indicates whether excessive requests are processed without delay.
	NoDelay bool
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

// GenerateForServer generates policy configuration for the server block.
func (g Generator) GenerateForServer(pols []policies.Policy, _ http.Server) policies.GenerateResultFiles {
	return generate(pols, tmplServer)
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

		isHTTPContextOnly := isShadowPolicy(rlp)

		// shadow policies (internally created for Route-attached policies) only contribute
		// limit_req_zone at the http context.
		if tmpl == tmplServer && isHTTPContextOnly {
			continue
		}

		settings := getRateLimitSettings(*rlp)

		var suffix string
		switch tmpl {
		case tmplHTTP:
			if isHTTPContextOnly {
				suffix = fileNameSuffixHTTP
			} else {
				suffix = fileNameSuffixGateway
			}
		case tmplServer:
			suffix = fileNameSuffixServer
		case tmplLocation:
			suffix = fileNameSuffixLocation
		}

		name := fmt.Sprintf("%s_%s_%s_%s.conf", fileNamePrefix, rlp.Namespace, rlp.Name, suffix)

		files = append(files, policies.File{
			Name:    name,
			Content: helpers.MustExecuteTemplate(tmpl, settings),
		})
	}

	return files
}

// isShadowPolicy checks if a RateLimitPolicy is intended to
// generate configuration only for the http context by looking for a specific annotation.
func isShadowPolicy(rlp *ngfAPI.RateLimitPolicy) bool {
	if rlp.Annotations == nil {
		return false
	}
	val, exists := rlp.Annotations[dataplane.InternalRLPAnnotationKey]
	return exists && val == dataplane.InternalRLPAnnotationValue
}
