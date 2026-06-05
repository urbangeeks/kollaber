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
Database connection string. The chart does not bundle a database, so
externalDatabaseUrl is required — point it at any reachable Postgres (a managed
instance, or one you deploy in-cluster such as a Bitnami postgresql release).
*/}}
{{- define "kollaber.databaseUrl" -}}
{{ required "externalDatabaseUrl is required: set --set externalDatabaseUrl=postgres://user:pass@host:5432/kollaber — the chart does not bundle Postgres (see https://kollaber.io/docs)" .Values.externalDatabaseUrl }}
{{- end }}
