{{/* Expand the name of the chart. */}}
{{- define "e5renew.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Create a default fully qualified app name. */}}
{{- define "e5renew.fullname" -}}
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

{{/* Common labels */}}
{{- define "e5renew.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{ include "e5renew.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/* Selector labels */}}
{{- define "e5renew.selectorLabels" -}}
app.kubernetes.io/name: {{ include "e5renew.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/* Create the name of the service account to use */}}
{{- define "e5renew.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "e5renew.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Secret holding database-dsn, azuread-*, usertoken-* and encryption-key:
.Values.existingSecret when set (the chart then renders no Secret), else the chart's own.
*/}}
{{- define "e5renew.secretName" -}}
{{- .Values.existingSecret | default (printf "%s-secret" (include "e5renew.fullname" .)) }}
{{- end }}
