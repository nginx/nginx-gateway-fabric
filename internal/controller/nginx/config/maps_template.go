package config

const mapsTemplateText = `
{{ range $m := . }}
map {{ $m.Source }} {{ $m.Variable }} {
	{{- if $m.UseHostnames }}
	hostnames;
	{{ end }}
	{{ range $p := $m.Parameters }}
	{{ $p.Value }} {{ $p.Result }};
	{{ end }}
}
{{- end }}
`
