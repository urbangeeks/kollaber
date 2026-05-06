{{/*
Expand the name of the chart.
*/}}
{{- define "kollaber.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kollaber.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kollaber.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/name: {{ include "kollaber.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Database URL — prefer externalDatabaseUrl, otherwise build from secret
*/}}
{{- define "kollaber.databaseUrl" -}}
{{- if .Values.externalDatabaseUrl -}}
{{ .Values.externalDatabaseUrl }}
{{- else -}}
postgres://kollaber:$(DB_PASSWORD)@{{ include "kollaber.fullname" . }}-postgres:5432/kollaber
{{- end -}}
{{- end }}
