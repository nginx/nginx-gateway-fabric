{{/*
Expand the name of the chart.
*/}}
{{- define "nginx-gateway.name" -}}
{{- if .Values.nginxGateway.name }}
{{- .Values.nginxGateway.name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "nginx-gateway.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default (include "nginx-gateway.name" .) }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create control plane config name.
*/}}
{{- define "nginx-gateway.config-name" -}}
{{- $name := .Values.nginxGateway.name | default .Values.nameOverride | default .Release.Name }}
{{- printf "%s-config" $name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create data plane config name.
*/}}
{{- define "nginx-gateway.proxy-config-name" -}}
{{- $name := .Values.nginxGateway.name | default .Values.nameOverride | default .Release.Name }}
{{- printf "%s-proxy-config" $name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create security context constraints name.
*/}}
{{- define "nginx-gateway.scc-name" -}}
{{- $name := .Values.nginxGateway.name | default .Values.nameOverride | default .Release.Name }}
{{- printf "%s-scc" $name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "nginx-gateway.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "nginx-gateway.labels" -}}
helm.sh/chart: {{ include "nginx-gateway.chart" . }}
{{ include "nginx-gateway.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "nginx-gateway.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nginx-gateway.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the ServiceAccount to use
*/}}
{{- define "nginx-gateway.serviceAccountName" -}}
{{- default (include "nginx-gateway.fullname" .) .Values.nginxGateway.serviceAccount.name }}
{{- end }}

{{/*
Expand leader election lock name.
*/}}
{{- define "nginx-gateway.leaderElectionName" -}}
{{- if .Values.nginxGateway.leaderElection.lockName -}}
{{ .Values.nginxGateway.leaderElection.lockName }}
{{- else -}}
{{- printf "%s-%s" (include "nginx-gateway.fullname" .) "leader-election" -}}
{{- end -}}
{{- end -}}

{{/*
Filters out empty fields from a struct.
*/}}
{{- define "filterEmptyFields" -}}
{{- $result := dict }}
{{- range $key, $value := . }}
  {{- if and (not (empty $value)) (not (and (kindIs "slice" $value) (eq (len $value) 0))) }}
    {{- $result = merge $result (dict $key $value) }}
  {{- end }}
{{- end }}
{{- if $result -}}
{{- $result | toYaml -}}
{{- end -}}
{{- end }}

{{/*
Create namespaced RBAC rules.
*/}}
{{- define "rbac.namespacedRules" -}}
- apiGroups:
  - ""
  resources:
  - secrets
  - configmaps
  - serviceaccounts
  - services
  verbs:
  - create
  - update
  - delete
  - list
  - get
  - watch
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - events.k8s.io
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
- apiGroups:
  - apps
  resources:
  - deployments
  - daemonsets
  verbs:
  - create
  - update
  - delete
  - list
  - get
  - watch
- apiGroups:
  - apps
  resources:
  - replicasets
  verbs:
  - get
  - list
- apiGroups:
  - autoscaling
  resources:
  - horizontalpodautoscalers
  verbs:
  - create
  - update
  - delete
  - list
  - get
  - watch
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - list
  - watch
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gateways
  - httproutes
  - referencegrants
  - grpcroutes
  - backendtlspolicies
  {{- if .Values.nginxGateway.gwAPIExperimentalFeatures.enable }}
  - tlsroutes
  - tcproutes
  - udproutes
  {{- end }}
  verbs:
  - list
  - watch
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - httproutes/status
  - gateways/status
  - gatewayclasses/status
  - grpcroutes/status
  - backendtlspolicies/status
  {{- if .Values.nginxGateway.gwAPIExperimentalFeatures.enable }}
  - tlsroutes/status
  - tcproutes/status
  - udproutes/status
  {{- end }}
  verbs:
  - update
- apiGroups:
  - gateway.nginx.org
  resources:
  - nginxgateways
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - gateway.nginx.org
  resources:
  - nginxproxies
  - clientsettingspolicies
  - observabilitypolicies
  - upstreamsettingspolicies
  - authenticationfilters
  - proxysettingspolicies
  - ratelimitpolicies
  {{- if or .Values.nginxGateway.snippetsFilters.enable .Values.nginxGateway.snippets.enable }}
  - snippetsfilters
  {{- end }}
  {{- if .Values.nginxGateway.snippets.enable }}
  - snippetspolicies
  {{- end }}
  verbs:
  - list
  - watch
- apiGroups:
  - gateway.nginx.org
  resources:
  - nginxgateways/status
  - clientsettingspolicies/status
  - observabilitypolicies/status
  - upstreamsettingspolicies/status
  - authenticationfilters/status
  - proxysettingspolicies/status
  - ratelimitpolicies/status
  {{- if or .Values.nginxGateway.snippetsFilters.enable .Values.nginxGateway.snippets.enable }}
  - snippetsfilters/status
  {{- end }}
  {{- if .Values.nginxGateway.snippets.enable }}
  - snippetspolicies/status
  {{- end }}
  verbs:
  - update
  {{- if .Values.nginxGateway.gwAPIInferenceExtension.enable }}
- apiGroups:
  - inference.networking.k8s.io
  resources:
  - inferencepools
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - inference.networking.k8s.io
  resources:
  - inferencepools/status
  verbs:
  - update
  {{- end }}
  {{- if .Values.nginxGateway.leaderElection.enable }}
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
  - get
  - update
  {{- end }}
  {{- if .Capabilities.APIVersions.Has "security.openshift.io/v1/SecurityContextConstraints" }}
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - roles
  - rolebindings
  verbs:
  - create
  - update
  - delete
  - list
  - get
  - watch
  {{- end}}
{{- end }}

{{/*
Create cluster RBAC rules.
*/}}
{{- define "rbac.clusterRules" -}}
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - get
  - list
  - watch
  {{- if or .Values.nginxGateway.productTelemetry.enable .Values.nginx.plus }}
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - list
  {{- end }}
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs:
  - list
  - watch
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gatewayclasses
  verbs:
  - list
  - watch
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gatewayclasses/status
  verbs:
  - update
  {{- if .Capabilities.APIVersions.Has "security.openshift.io/v1/SecurityContextConstraints" }}
- apiGroups:
  - security.openshift.io
  resources:
  - securitycontextconstraints
  resourceNames:
  - {{ include "nginx-gateway.scc-name" . }}
  - {{ include "nginx-gateway.scc-name" . }}-nginx
  verbs:
  - use
  {{- end }}
{{- end }}
