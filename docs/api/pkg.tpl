{{ define "packages" }}
---
title: API reference
description: "API reference for NGINX Gateway Fabric custom resources under gateway.nginx.org."
weight: 100
f5-content-type: reference
f5-product: F5 NGINX Gateway Fabric
f5-docs: DOCS-1855
f5-summary: >
  Browse generated API documentation for NGINX Gateway Fabric custom resource definitions.
  Packages include gateway.nginx.org/v1alpha1 and gateway.nginx.org/v1alpha2.
f5-keywords: "NGINX Gateway Fabric, API reference, CRD, gateway.nginx.org, v1alpha1, v1alpha2"
f5-audience: any
---

## Overview

NGINX Gateway API Reference

{{ with .packages}}
<p>Packages:</p>
<ul>
    {{ range . }}
    <li>
        <a href="#{{- packageAnchorID . -}}">{{ packageDisplayName . }}</a>
    </li>
    {{ end }}
</ul>
{{ end}}

{{ range .packages }}
    <h2 id="{{- packageAnchorID . -}}">
        {{- packageDisplayName . -}}
    </h2>

    {{ with (index .GoPackages 0 )}}
        {{ with .DocComments }}
        <p>
            {{ safe (renderComments .) }}
        </p>
        {{ end }}
    {{ end }}

    Resource Types:
    <ul>
    {{- range (visibleTypes (sortedTypes .Types)) -}}
        {{ if isExportedType . -}}
        <li>
            <a href="{{ linkForType . }}">{{ typeDisplayName . }}</a>
        </li>
        {{- end }}
    {{- end -}}
    </ul>

    {{ range (visibleTypes (sortedTypes .Types))}}
        {{ template "type" .  }}
    {{ end }}
    <hr/>
{{ end }}

<p><em>
    Generated with <code>gen-crd-api-reference-docs</code>
</em></p>

{{ end }}
