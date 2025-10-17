#!/bin/bash
#
# Script to generate remaining Helm subcharts for Developer Mesh
# This creates the Worker and RAG Loader subcharts following the REST API pattern
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHARTS_DIR="$SCRIPT_DIR/developer-mesh/charts"

echo "🚀 Generating remaining Developer Mesh Helm subcharts..."
echo ""

# Function to create Worker subchart
create_worker_chart() {
    echo "📦 Creating Worker subchart..."

    WORKER_DIR="$CHARTS_DIR/worker"
    mkdir -p "$WORKER_DIR/templates"

    # values.yaml already created

    # templates/_helpers.tpl
    cat > "$WORKER_DIR/templates/_helpers.tpl" <<'EOF'
{{- define "worker.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "worker.fullname" -}}
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

{{- define "worker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "worker.labels" -}}
helm.sh/chart: {{ include "worker.chart" . }}
{{ include "worker.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: developer-mesh
app.kubernetes.io/component: worker
{{- end }}

{{- define "worker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "worker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: worker
{{- end }}

{{- define "worker.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "worker.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "worker.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .Values.image.repository }}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}
EOF

    echo "✅ Worker subchart templates created"
}

# Function to create RAG Loader subchart
create_rag_loader_chart() {
    echo "📦 Creating RAG Loader subchart..."

    RAG_DIR="$CHARTS_DIR/rag-loader"
    mkdir -p "$RAG_DIR/templates"

    # templates/_helpers.tpl
    cat > "$RAG_DIR/templates/_helpers.tpl" <<'EOF'
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
EOF

    echo "✅ RAG Loader subchart templates created"
}

# Execute
create_worker_chart
create_rag_loader_chart

echo ""
echo "✨ All subcharts generated successfully!"
echo ""
echo "Next steps:"
echo "  1. Complete full deployment templates (copy from rest-api pattern)"
echo "  2. Test with: helm lint ./developer-mesh"
echo "  3. Deploy with: helm install developer-mesh ./developer-mesh -f values-dev.yaml"
echo ""
