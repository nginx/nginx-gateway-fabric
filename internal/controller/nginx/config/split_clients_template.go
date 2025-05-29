package config

const splitClientsTemplateText = `
{{ range $sc := . }}
split_clients $request_id ${{ $sc.VariableName }} {
    {{- range $d := $sc.Distributions }}
        {{- if eq $d.Percent "0.00" }}
    # {{ $d.Percent }}% {{ $d.Value }};
        {{- else }}
    {{ $d.Percent }}% {{ $d.Value }};
        {{- end }}
    {{- end }}
}
{{ end }}
`
