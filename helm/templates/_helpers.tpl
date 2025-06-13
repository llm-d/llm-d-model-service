{{/*
Expand the name of the chart.
*/}}
{{- define "llm-d-modelservice.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "llm-d-modelservice.fullname" -}}
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
{{- define "llm-d-modelservice.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "llm-d-modelservice.labels" -}}
helm.sh/chart: {{ include "llm-d-modelservice.chart" . }}
{{ include "llm-d-modelservice.eppSelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
EPP selector labels
*/}}
{{- define "llm-d-modelservice.eppSelectorLabels" -}}
app.kubernetes.io/name: {{ include "llm-d-modelservice.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
llm-d.ai/epp: {{ include "llm-d-modelservice.fullname" . }}-epp
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "llm-d-modelservice.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- (include "llm-d-modelservice.fullname" .) -}}-sa
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the EPP service account to use
*/}}
{{- define "llm-d-modelservice.eppServiceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- (include "llm-d-modelservice.fullname" .) -}}-epp-sa
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
