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

{{/* Sanitized model name (DNS compliant) */}}
{{- define "llm-d-modelservice.sanitizedModelName" -}}
  {{- $name := .Release.Name | lower | trim -}}
  {{- $name = regexReplaceAll "[^a-z0-9_.-]" $name "-" -}}
  {{- $name = regexReplaceAll "^[\\-._]+" $name "" -}}
  {{- $name = regexReplaceAll "[\\-._]+$" $name "" -}}
  {{- $name = regexReplaceAll "\\." $name "-" -}}

  {{- if gt (len $name) 63 -}}
    {{- $name = substr 0 63 $name -}}
  {{- end -}}

{{- $name -}}
{{- end }}

{{/* Common P/D labels */}}
{{- define "llm-d-modelservice.pdlabels" -}}
llm-d.ai/inferenceServing: "true"
llm-d.ai/model: {{ (include "llm-d-modelservice.sanitizedModelName" .) -}}
{{- end }}

{{/* prefill labels */}}
{{- define "llm-d-modelservice.prefilllabels" -}}
{{ include "llm-d-modelservice.pdlabels" . }}
llm-d.ai/role: prefill
{{- end }}

{{/* decode labels */}}
{{- define "llm-d-modelservice.decodelabels" -}}
{{ include "llm-d-modelservice.pdlabels" . }}
llm-d.ai/role: decode
{{- end }}

{{/* affinity from acceleratorTypes */}}
{{- define "llm-d-modelservice.acceleratorTypes" -}}
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
          - key: {{ .labelKey }}
            operator: In
            {{- with .labelValues }}
            values:
            {{- toYaml . | nindent 14 }}
            {{- end }}
{{- end }}

{{/* Routing proxy -- sidecar for decode pods */}}
{{- define "llm-d-modelservice.routingProxy" -}}
initContainers:
  - name: routing-proxy
    args:
      - --port={{ default 8080 .servicePort }}
      - --vllm-port={{ default 8200 .proxy.targetPort }}
      - --connector=nixlv2
      - -v={{ default 5 .proxy.debugLevel }}
    image: {{ .image }}
    imagePullPolicy: Always
    ports:
      - containerPort: {{ default 8080 .servicePort }}
    protocol: TCP
    resources: {}
    restartPolicy: Always
    securityContext:
    allowPrivilegeEscalation: false
    runAsNonRoot: true
{{- end }}

{{- define "llm-d-modelservice.parallelism" -}}
{{- $parallelism := dict "tensor" 1 "data" 1 -}}
{{- if and . .tensor }}
{{- $parallelism = mergeOverwrite $parallelism (dict "tensor" .tensor) -}}
{{- end }}
{{- if and . .data }}
{{- $parallelism = mergeOverwrite $parallelism (dict "data" .data) -}}
{{- end }}
{{- $parallelism | toYaml | nindent 0 }}
{{- end }}

{{/* P/D service account name */}}
{{- define "llm-d-modelservice.pdServiceAccountName" -}}
{{ include "llm-d-modelservice.sanitizedModelName" . }}-sa
{{- end }}

{{/* EPP service account name */}}
{{- define "llm-d-modelservice.eppServiceAccountName" -}}
{{ include "llm-d-modelservice.sanitizedModelName" . }}-epp-sa
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
Volumes for PD containers based on model artifact prefix
*/}}
{{- define "llm-d-modelservice.mountModelVolumeVolumes" -}}
{{- if eq .Values.modelArtifacts.prefix "hf" }}
- name: model-storage
  emptyDir: 
    sizeLimit: {{ default "0" .Values.modelArtifacts.size }}
{{- else if eq .Values.modelArtifacts.prefix "pvc" }}
- name: model-storage
  persistentVolumeClaim:
    claimName: {{ .Values.modelArtifacts.artifact }}
    readOnly: true
{{- else if eq .Values.modelArtifacts.prefix "oci" }}
- name: model-storage
  image:
    reference: {{ .Values.modelArtifacts.artifact }}
    pullPolicy: {{ default "Always" .Values.modelArtifacts.imagePullPolicy }}
{{- end }}
{{- end }}

{{/*
VolumeMount for a PD container
Supplies model-storage mount if mountModelVolume: true for the container
*/}}
{{- define "llm-d-modelservice.mountModelVolumeVolumeMounts" -}}
{{- if or .volumeMounts .mountModelVolume }}
volumeMounts:
{{- end }}
{{- /* user supplied volume mount in values */}}
{{- with .volumeMounts }}
  {{- toYaml . | nindent 8 }}
{{- end }}
{{- /* what we add if mounModelVolume is true */}}
{{- if .mountModelVolume }}
  - name: model-storage
    mountPath: /model-cache
{{- end }}
{{- end }}
