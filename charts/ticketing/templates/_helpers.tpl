{{/* Numele de bază al chart-ului. */}}
{{- define "ticketing.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Numele stabil al release-ului. */}}
{{- define "ticketing.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "ticketing.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "ticketing.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ticketing.commonLabels" -}}
helm.sh/chart: {{ include "ticketing.chart" . }}
app.kubernetes.io/part-of: ticketing
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "ticketing.apiSelectorLabels" -}}
app.kubernetes.io/name: {{ include "ticketing.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: api
{{- end -}}

{{- define "ticketing.postgresqlSelectorLabels" -}}
app.kubernetes.io/name: {{ include "ticketing.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: postgresql
{{- end -}}

{{- define "ticketing.apiName" -}}
{{- printf "%s-api" (include "ticketing.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ticketing.postgresqlName" -}}
{{- printf "%s-postgresql" (include "ticketing.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ticketing.configName" -}}
{{- printf "%s-config" (include "ticketing.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ticketing.secretName" -}}
{{- if .Values.postgresql.auth.existingSecret -}}
{{- .Values.postgresql.auth.existingSecret -}}
{{- else -}}
{{- printf "%s-db" (include "ticketing.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

