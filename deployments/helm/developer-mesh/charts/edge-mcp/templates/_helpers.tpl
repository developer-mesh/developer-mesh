{{/*
Expand the name of the chart.
*/}}
{{- define "edge-mcp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "edge-mcp.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "edge-mcp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "edge-mcp.labels" -}}
helm.sh/chart: {{ include "edge-mcp.chart" . }}
{{ include "edge-mcp.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: developer-mesh
{{- end }}

{{/*
Selector labels
*/}}
{{- define "edge-mcp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "edge-mcp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: edge-mcp
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "edge-mcp.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "edge-mcp.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use
*/}}
{{- define "edge-mcp.secretName" -}}
{{- if .Values.secrets.name }}
{{- .Values.secrets.name }}
{{- else }}
{{- include "edge-mcp.fullname" . }}-secrets
{{- end }}
{{- end }}

{{/*
Image name
*/}}
{{- define "edge-mcp.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .Values.image.repository }}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}
