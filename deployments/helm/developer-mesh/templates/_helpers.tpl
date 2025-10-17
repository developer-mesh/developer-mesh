{{/*
Expand the name of the chart.
*/}}
{{- define "developer-mesh.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "developer-mesh.fullname" -}}
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
{{- define "developer-mesh.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "developer-mesh.labels" -}}
helm.sh/chart: {{ include "developer-mesh.chart" . }}
{{ include "developer-mesh.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: developer-mesh
{{- with .Values.global.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "developer-mesh.selectorLabels" -}}
app.kubernetes.io/name: {{ include "developer-mesh.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the namespace to use
*/}}
{{- define "developer-mesh.namespace" -}}
{{- if .Values.global.namespace.create }}
{{- .Values.global.namespace.name }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Database host
*/}}
{{- define "developer-mesh.database.host" -}}
{{- if .Values.global.database.embedded }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else }}
{{- .Values.global.database.host }}
{{- end }}
{{- end }}

{{/*
Database port
*/}}
{{- define "developer-mesh.database.port" -}}
{{- .Values.global.database.port }}
{{- end }}

{{/*
Database name
*/}}
{{- define "developer-mesh.database.name" -}}
{{- .Values.global.database.name }}
{{- end }}

{{/*
Database username
*/}}
{{- define "developer-mesh.database.username" -}}
{{- .Values.global.database.username }}
{{- end }}

{{/*
Database password secret name
*/}}
{{- define "developer-mesh.database.secretName" -}}
{{- if .Values.global.database.existingSecret }}
{{- .Values.global.database.existingSecret }}
{{- else if .Values.global.database.embedded }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else }}
{{- printf "%s-database-secret" (include "developer-mesh.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Database password secret key
*/}}
{{- define "developer-mesh.database.secretKey" -}}
{{- if .Values.global.database.existingSecret }}
{{- .Values.global.database.existingSecretPasswordKey }}
{{- else }}
{{- "password" }}
{{- end }}
{{- end }}

{{/*
Database DSN (connection string)
*/}}
{{- define "developer-mesh.database.dsn" -}}
{{- $host := include "developer-mesh.database.host" . -}}
{{- $port := include "developer-mesh.database.port" . -}}
{{- $name := include "developer-mesh.database.name" . -}}
{{- $username := include "developer-mesh.database.username" . -}}
{{- $sslMode := .Values.global.database.sslMode -}}
{{- $searchPath := .Values.global.database.searchPath -}}
{{- printf "postgresql://%s:$(DATABASE_PASSWORD)@%s:%s/%s?sslmode=%s&search_path=%s" $username $host $port $name $sslMode $searchPath }}
{{- end }}

{{/*
Redis host
*/}}
{{- define "developer-mesh.redis.host" -}}
{{- if .Values.global.redis.embedded }}
{{- printf "%s-redis-master" .Release.Name }}
{{- else }}
{{- .Values.global.redis.host }}
{{- end }}
{{- end }}

{{/*
Redis port
*/}}
{{- define "developer-mesh.redis.port" -}}
{{- .Values.global.redis.port }}
{{- end }}

{{/*
Redis address
*/}}
{{- define "developer-mesh.redis.address" -}}
{{- $host := include "developer-mesh.redis.host" . -}}
{{- $port := include "developer-mesh.redis.port" . -}}
{{- printf "%s:%s" $host $port }}
{{- end }}

{{/*
Redis password secret name
*/}}
{{- define "developer-mesh.redis.secretName" -}}
{{- if .Values.global.redis.existingSecret }}
{{- .Values.global.redis.existingSecret }}
{{- else if .Values.global.redis.embedded }}
{{- printf "%s-redis" .Release.Name }}
{{- else }}
{{- printf "%s-redis-secret" (include "developer-mesh.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Redis password secret key
*/}}
{{- define "developer-mesh.redis.secretKey" -}}
{{- if .Values.global.redis.existingSecret }}
{{- .Values.global.redis.existingSecretPasswordKey }}
{{- else }}
{{- "redis-password" }}
{{- end }}
{{- end }}

{{/*
AWS credentials secret name
*/}}
{{- define "developer-mesh.aws.secretName" -}}
{{- if .Values.global.aws.existingSecret }}
{{- .Values.global.aws.existingSecret }}
{{- else }}
{{- printf "%s-aws-secret" (include "developer-mesh.fullname" .) }}
{{- end }}
{{- end }}

{{/*
JWT secret name
*/}}
{{- define "developer-mesh.jwt.secretName" -}}
{{- if .Values.global.security.jwt.existingSecret }}
{{- .Values.global.security.jwt.existingSecret }}
{{- else }}
{{- printf "%s-jwt-secret" (include "developer-mesh.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Encryption master key secret name
*/}}
{{- define "developer-mesh.encryption.secretName" -}}
{{- if .Values.global.security.existingSecret }}
{{- .Values.global.security.existingSecret }}
{{- else }}
{{- printf "%s-encryption-secret" (include "developer-mesh.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Image registry
*/}}
{{- define "developer-mesh.imageRegistry" -}}
{{- if .Values.global.imageRegistry }}
{{- printf "%s/" .Values.global.imageRegistry }}
{{- end }}
{{- end }}

{{/*
Common environment variables for database connection
*/}}
{{- define "developer-mesh.database.env" -}}
- name: DATABASE_HOST
  value: {{ include "developer-mesh.database.host" . | quote }}
- name: DB_HOST
  value: {{ include "developer-mesh.database.host" . | quote }}
- name: DATABASE_PORT
  value: {{ include "developer-mesh.database.port" . | quote }}
- name: DB_PORT
  value: {{ include "developer-mesh.database.port" . | quote }}
- name: DATABASE_NAME
  value: {{ include "developer-mesh.database.name" . | quote }}
- name: DB_NAME
  value: {{ include "developer-mesh.database.name" . | quote }}
- name: DATABASE_USER
  value: {{ include "developer-mesh.database.username" . | quote }}
- name: DB_USER
  value: {{ include "developer-mesh.database.username" . | quote }}
- name: DATABASE_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "developer-mesh.database.secretName" . }}
      key: {{ include "developer-mesh.database.secretKey" . }}
- name: DB_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "developer-mesh.database.secretName" . }}
      key: {{ include "developer-mesh.database.secretKey" . }}
- name: DATABASE_SSL_MODE
  value: {{ .Values.global.database.sslMode | quote }}
- name: DB_SSLMODE
  value: {{ .Values.global.database.sslMode | quote }}
- name: DATABASE_SEARCH_PATH
  value: {{ .Values.global.database.searchPath | quote }}
- name: DATABASE_DSN
  value: {{ include "developer-mesh.database.dsn" . | quote }}
{{- end }}

{{/*
Common environment variables for Redis connection
*/}}
{{- define "developer-mesh.redis.env" -}}
- name: REDIS_HOST
  value: {{ include "developer-mesh.redis.host" . | quote }}
- name: REDIS_PORT
  value: {{ include "developer-mesh.redis.port" . | quote }}
- name: REDIS_ADDR
  value: {{ include "developer-mesh.redis.address" . | quote }}
- name: REDIS_ADDRESS
  value: {{ include "developer-mesh.redis.address" . | quote }}
{{- if or .Values.global.redis.password .Values.global.redis.existingSecret .Values.global.redis.embedded }}
- name: REDIS_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "developer-mesh.redis.secretName" . }}
      key: {{ include "developer-mesh.redis.secretKey" . }}
{{- end }}
- name: REDIS_DATABASE
  value: {{ .Values.global.redis.database | quote }}
{{- end }}

{{/*
Common environment variables for AWS
*/}}
{{- define "developer-mesh.aws.env" -}}
- name: AWS_REGION
  value: {{ .Values.global.aws.region | quote }}
{{- if not .Values.global.aws.useIRSA }}
{{- if or .Values.global.aws.accessKeyId .Values.global.aws.existingSecret }}
- name: AWS_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: {{ include "developer-mesh.aws.secretName" . }}
      key: {{ .Values.global.aws.existingSecretAccessKeyIdKey }}
- name: AWS_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "developer-mesh.aws.secretName" . }}
      key: {{ .Values.global.aws.existingSecretSecretAccessKeyKey }}
{{- end }}
{{- end }}
{{- if .Values.global.aws.s3.bucket }}
- name: S3_BUCKET
  value: {{ .Values.global.aws.s3.bucket | quote }}
{{- end }}
{{- if .Values.global.aws.s3.endpoint }}
- name: S3_ENDPOINT
  value: {{ .Values.global.aws.s3.endpoint | quote }}
{{- end }}
{{- if .Values.global.aws.s3.usePathStyle }}
- name: S3_USE_PATH_STYLE
  value: "true"
{{- end }}
{{- end }}

{{/*
Common security context for pods
*/}}
{{- define "developer-mesh.podSecurityContext" -}}
runAsNonRoot: {{ .Values.global.security.podSecurityContext.runAsNonRoot }}
runAsUser: {{ .Values.global.security.podSecurityContext.runAsUser }}
fsGroup: {{ .Values.global.security.podSecurityContext.fsGroup }}
seccompProfile:
  type: {{ .Values.global.security.podSecurityContext.seccompProfile.type }}
{{- end }}

{{/*
Common security context for containers
*/}}
{{- define "developer-mesh.containerSecurityContext" -}}
allowPrivilegeEscalation: {{ .Values.global.security.containerSecurityContext.allowPrivilegeEscalation }}
capabilities:
  drop:
  {{- range .Values.global.security.containerSecurityContext.capabilities.drop }}
  - {{ . }}
  {{- end }}
readOnlyRootFilesystem: {{ .Values.global.security.containerSecurityContext.readOnlyRootFilesystem }}
{{- end }}

{{/*
Common volume mounts for read-only root filesystem
*/}}
{{- define "developer-mesh.commonVolumeMounts" -}}
- name: tmp
  mountPath: /tmp
- name: cache
  mountPath: /var/cache
{{- end }}

{{/*
Common volumes for read-only root filesystem
*/}}
{{- define "developer-mesh.commonVolumes" -}}
- name: tmp
  emptyDir: {}
- name: cache
  emptyDir: {}
{{- end }}

{{/*
Service account annotations for IRSA
*/}}
{{- define "developer-mesh.serviceAccountAnnotations" -}}
{{- if .Values.global.aws.useIRSA }}
eks.amazonaws.com/role-arn: {{ .Values.global.aws.roleArn }}
{{- end }}
{{- end }}
