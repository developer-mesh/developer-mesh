{{/*
Expand the name of the chart.
*/}}
{{- define "rest-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "rest-api.fullname" -}}
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
{{- define "rest-api.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "rest-api.labels" -}}
helm.sh/chart: {{ include "rest-api.chart" . }}
{{ include "rest-api.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: developer-mesh
app.kubernetes.io/component: rest-api
{{- end }}

{{/*
Selector labels
*/}}
{{- define "rest-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "rest-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: rest-api
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "rest-api.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "rest-api.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use
*/}}
{{- define "rest-api.secretName" -}}
{{- if .Values.secrets.name }}
{{- .Values.secrets.name }}
{{- else }}
{{- include "rest-api.fullname" . }}-secrets
{{- end }}
{{- end }}

{{/*
Image name
*/}}
{{- define "rest-api.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .Values.image.repository }}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}
