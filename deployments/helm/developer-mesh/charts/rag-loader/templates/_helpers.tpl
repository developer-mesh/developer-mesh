{{- define "rag-loader.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "rag-loader.fullname" -}}
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

{{- define "rag-loader.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "rag-loader.labels" -}}
helm.sh/chart: {{ include "rag-loader.chart" . }}
{{ include "rag-loader.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: developer-mesh
app.kubernetes.io/component: rag-loader
{{- end }}

{{- define "rag-loader.selectorLabels" -}}
app.kubernetes.io/name: {{ include "rag-loader.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: rag-loader
{{- end }}

{{- define "rag-loader.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "rag-loader.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "rag-loader.secretName" -}}
{{- if .Values.secrets.name }}
{{- .Values.secrets.name }}
{{- else }}
{{- include "rag-loader.fullname" . }}-secrets
{{- end }}
{{- end }}

{{- define "rag-loader.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .Values.image.repository }}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}
