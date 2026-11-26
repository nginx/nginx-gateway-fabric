package proxysettings

import (
	"fmt"
	"text/template"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const defaultCacheKey = "$scheme$host$request_uri"

var tmpl = template.Must(template.New("proxy settings policy").Parse(proxySettingsTemplate))

const proxySettingsTemplate = `
{{- if .ProxyBufferingDirective }}
proxy_buffering {{ .ProxyBufferingDirective }};
{{- end }}
{{- if .ProxyBufferSize }}
proxy_buffer_size {{ .ProxyBufferSize }};
{{- end }}
{{- if .ProxyBuffers }}
proxy_buffers {{ .ProxyBuffers }};
{{- end }}
{{- if .ProxyBusyBuffersSize }}
proxy_busy_buffers_size {{ .ProxyBusyBuffersSize }};
{{- end }}
{{- if .ProxyConnectTimeout }}
proxy_connect_timeout {{ .ProxyConnectTimeout }};
{{- end }}
{{- if .ProxyReadTimeout }}
proxy_read_timeout {{ .ProxyReadTimeout }};
{{- end }}
{{- if .ProxySendTimeout }}
proxy_send_timeout {{ .ProxySendTimeout }};
{{- end }}
{{- with .ProxyCache }}
{{- if .Enable }}
proxy_cache {{ .Enable }};
{{- end }}
{{- with .Valid }}
{{- if .Time }}
proxy_cache_valid{{ range .Codes }} {{ . }}{{ end }} {{ .Time }};
{{- end }}
{{- end }}
{{- if .Key }}
proxy_cache_key "{{ .Key }}";
{{- end }}
{{- if .UseStale }}
proxy_cache_use_stale{{ range .UseStale }} {{ . }}{{ end }};
{{- end }}
{{- if .Revalidate }}
proxy_cache_revalidate {{ .Revalidate }};
{{- end }}
{{- if .BackgroundUpdate }}
proxy_cache_background_update {{ .BackgroundUpdate }};
{{- end }}
{{- if .MinUses }}
proxy_cache_min_uses {{ .MinUses }};
{{- end }}
{{- if .Methods }}
proxy_cache_methods{{ range .Methods }} {{ . }}{{ end }};
{{- end }}
{{- with .Lock }}
{{- if .Enable }}
proxy_cache_lock {{ .Enable }};
{{- end }}
{{- if .Age }}
proxy_cache_lock_age {{ .Age }};
{{- end }}
{{- if .Timeout }}
proxy_cache_lock_timeout {{ .Timeout }};
{{- end }}
{{- end }}
{{- with .Conditions }}
{{- if .NoCache }}
proxy_no_cache{{ range .NoCache }} "{{ . }}"{{ end }};
{{- end }}
{{- if .Bypass }}
proxy_cache_bypass{{ range .Bypass }} "{{ . }}"{{ end }};
{{- end }}
{{- end }}
{{- if .IgnoreHeaders }}
proxy_ignore_headers{{ range .IgnoreHeaders }} {{ . }}{{ end }};
{{- end }}
{{- if and $.Plus .CachePurgeVar }}
proxy_cache_purge {{ .CachePurgeVar }};
{{- end }}
{{- end }}
`

var geoBlockTmpl = template.Must(template.New("cache purge geo block").Parse(geoBlockTemplateText))

const geoBlockTemplateText = `
geo {{ .Variable }} {
    default 0;
{{- range .Entries }}
    {{ . }} 1;
{{- end }}
}
`

// cachePurgeGeoBlock holds the data for rendering the cache purge geo block template.
type cachePurgeGeoBlock struct {
	Variable string
	Entries  []string
}

type proxySettings struct {
	ProxyCache              *http.ProxyCache
	ProxyBufferingDirective string
	ProxyBufferSize         string
	ProxyBuffers            string
	ProxyBusyBuffersSize    string
	ProxyConnectTimeout     string
	ProxyReadTimeout        string
	ProxySendTimeout        string
	Plus                    bool
}

// generationContext identifies whether proxy settings target an external or internal location.
// It only affects the generated include filename; caching is generated in both contexts because
// internal match/inference locations perform the real proxy_pass to the backend.
type generationContext int

const (
	// Generate settings for an external location block.
	externalContext generationContext = iota
	// Generate settings for an application-routing internal location block.
	internalContext
)

func getProxySettings(psp *ngfAPI.ProxySettingsPolicy, plus bool) proxySettings {
	spec := psp.Spec
	settings := proxySettings{Plus: plus}

	if spec.Buffering != nil {
		settings = getBufferingSettings(spec.Buffering, settings)
	}

	if spec.Timeout != nil {
		settings = getTimeoutSettings(spec.Timeout, settings)
	}

	if spec.Caching != nil {
		settings = getCachingSettings(psp, spec.Caching, settings)
	}

	return settings
}

func getBufferingSettings(buffering *ngfAPI.ProxyBuffering, settings proxySettings) proxySettings {
	if buffering.Disable != nil {
		if *buffering.Disable {
			settings.ProxyBufferingDirective = "off"
		} else {
			settings.ProxyBufferingDirective = "on"
		}
	}

	if buffering.BufferSize != nil {
		settings.ProxyBufferSize = string(*buffering.BufferSize)
	}

	if buffering.Buffers != nil {
		settings.ProxyBuffers = fmt.Sprintf("%d %s", buffering.Buffers.Number, buffering.Buffers.Size)
	}

	if buffering.BusyBuffersSize != nil {
		settings.ProxyBusyBuffersSize = string(*buffering.BusyBuffersSize)
	}

	return settings
}

func getTimeoutSettings(timeout *ngfAPI.ProxyTimeout, settings proxySettings) proxySettings {
	if timeout.Connect != nil {
		settings.ProxyConnectTimeout = string(*timeout.Connect)
	}

	if timeout.Read != nil {
		settings.ProxyReadTimeout = string(*timeout.Read)
	}

	if timeout.Send != nil {
		settings.ProxySendTimeout = string(*timeout.Send)
	}

	return settings
}

func getCachingSettings(
	psp *ngfAPI.ProxySettingsPolicy,
	caching *ngfAPI.ProxyCache,
	settings proxySettings,
) proxySettings {
	settings.ProxyCache = &http.ProxyCache{}
	settings.ProxyCache.ZoneName = shared.ProxyCacheZoneName

	if caching.Enable != nil {
		settings.ProxyCache.Enable = cacheEnableDirective(*caching.Enable, settings.ProxyCache.ZoneName)
	}

	if caching.Valid != nil {
		settings = getCacheValidSettings(caching.Valid, settings)
	}

	settings.ProxyCache.Key = setCacheKey(caching)

	if caching.UseStale != nil {
		settings.ProxyCache.UseStale = make([]string, 0, len(caching.UseStale))
		for _, value := range caching.UseStale {
			settings.ProxyCache.UseStale = append(settings.ProxyCache.UseStale, string(value))
		}
	}

	if caching.Revalidate != nil {
		settings.ProxyCache.Revalidate = boolDirective(caching.Revalidate)
	}

	if caching.BackgroundUpdate != nil {
		settings.ProxyCache.BackgroundUpdate = boolDirective(caching.BackgroundUpdate)
	}

	if caching.MinUses != nil {
		settings.ProxyCache.MinUses = caching.MinUses
	}

	if caching.Methods != nil {
		settings.ProxyCache.Methods = caching.Methods
	}

	if caching.Lock != nil {
		settings = getCacheLockSettings(caching.Lock, settings)
	}

	if caching.Conditions != nil {
		settings.ProxyCache.Conditions = &http.ProxyCacheConditions{
			NoCache: caching.Conditions.NoCache,
			Bypass:  caching.Conditions.Bypass,
		}
	}

	if caching.IgnoreHeaders != nil {
		settings.ProxyCache.IgnoreHeaders = make([]string, len(caching.IgnoreHeaders))
		for i, header := range caching.IgnoreHeaders {
			settings.ProxyCache.IgnoreHeaders[i] = string(header)
		}
	}

	if len(caching.CachePurgeAllow) > 0 {
		settings.ProxyCache.CachePurgeVar = cachePurgeVarName(psp)
	}

	return settings
}

func cacheEnableDirective(enable bool, zoneName string) string {
	if enable {
		return zoneName
	}
	return "off"
}

func getCacheValidSettings(valid *ngfAPI.ProxyCacheValid, settings proxySettings) proxySettings {
	settings.ProxyCache.Valid = &http.ProxyCacheValid{}
	if valid.Codes != nil {
		settings.ProxyCache.Valid.Codes = valid.Codes
	}
	if valid.Time != nil {
		settings.ProxyCache.Valid.Time = (*string)(valid.Time)
	}

	return settings
}

func setCacheKey(caching *ngfAPI.ProxyCache) *string {
	switch {
	case caching.Key != nil:
		return caching.Key
	case caching.Enable != nil && *caching.Enable:
		// Need to set a default different from the NGINX default to prevent cross-hostname cache collision
		return helpers.GetPointer(defaultCacheKey)
	}
	return nil
}

func getCacheLockSettings(lock *ngfAPI.ProxyCacheLock, settings proxySettings) proxySettings {
	settings.ProxyCache.Lock = &http.ProxyCacheLock{}
	if lock.Enable != nil {
		settings.ProxyCache.Lock.Enable = boolDirective(lock.Enable)
	}
	if lock.Timeout != nil {
		settings.ProxyCache.Lock.Timeout = (*string)(lock.Timeout)
	}
	if lock.Age != nil {
		settings.ProxyCache.Lock.Age = (*string)(lock.Age)
	}

	return settings
}

func boolDirective(value *bool) string {
	if value == nil {
		return ""
	}
	if *value {
		return "on"
	}
	return "off"
}

// Generator generates nginx configuration based on a ProxySettingsPolicy.
type Generator struct {
	policies.UnimplementedGenerator
	plus bool
}

// NewGenerator returns a new instance of Generator.
func NewGenerator(plus bool) *Generator {
	return &Generator{plus: plus}
}

// GenerateForHTTP generates http context configuration for ProxySettingsPolicies.
func (g Generator) GenerateForHTTP(pols []policies.Policy) policies.GenerateResultFiles {
	var files policies.GenerateResultFiles

	for _, pol := range pols {
		psp, ok := pol.(*ngfAPI.ProxySettingsPolicy)
		if !ok {
			continue
		}

		if g.plus && psp.Spec.Caching != nil && len(psp.Spec.Caching.CachePurgeAllow) > 0 {
			files = append(files, geoBlockFile(psp))
		}
	}

	files = append(files, generate(pols, g.plus, externalContext)...)

	return files
}

// GenerateForLocation generates policy configuration for a normal location block.
func (g Generator) GenerateForLocation(pols []policies.Policy, _ http.Location) policies.GenerateResultFiles {
	return generate(pols, g.plus, externalContext)
}

// GenerateForInternalLocation generates policy configuration for an internal location block.
func (g Generator) GenerateForInternalLocation(pols []policies.Policy) policies.GenerateResultFiles {
	return generate(pols, g.plus, internalContext)
}

func generate(pols []policies.Policy, plus bool, context generationContext) policies.GenerateResultFiles {
	files := make(policies.GenerateResultFiles, 0, len(pols))

	for _, pol := range pols {
		psp, ok := pol.(*ngfAPI.ProxySettingsPolicy)
		if !ok {
			continue
		}

		settings := getProxySettings(psp, plus)
		filename := fmt.Sprintf("ProxySettingsPolicy_%s_%s.conf", psp.Namespace, psp.Name)
		if context == internalContext {
			filename = fmt.Sprintf("ProxySettingsPolicy_%s_%s_internal.conf", psp.Namespace, psp.Name)
		}

		files = append(files, policies.File{
			Name:    filename,
			Content: helpers.MustExecuteTemplate(tmpl, settings),
		})
	}

	return files
}

// geoBlockFile builds the geo block include file that maps the client's address to the
// policy's cache purge allowlist variable.
func geoBlockFile(psp *ngfAPI.ProxySettingsPolicy) policies.File {
	block := cachePurgeGeoBlock{
		Variable: cachePurgeVarName(psp),
		Entries:  psp.Spec.Caching.CachePurgeAllow,
	}

	return policies.File{
		Name:    fmt.Sprintf("ProxySettingsPolicy_%s_%s_cache_purge_geo.conf", psp.Namespace, psp.Name),
		Content: helpers.MustExecuteTemplate(geoBlockTmpl, block),
	}
}

// cachePurgeVarName returns the NGINX variable name for the policy's cache purge allowlist.
func cachePurgeVarName(psp *ngfAPI.ProxySettingsPolicy) string {
	return fmt.Sprintf("$nginx_gateway_cache_purge_%s_%s",
		helpers.SanitizeNginxVar(psp.Namespace),
		helpers.SanitizeNginxVar(psp.Name),
	)
}
