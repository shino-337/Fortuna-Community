{{/*
Expand the name of the chart.
*/}}
{{- define "fortuna.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "fortuna.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Namespace (prefer Release.Namespace; override for manifest metadata).
*/}}
{{- define "fortuna.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Core image
*/}}
{{- define "fortuna.coreImage" -}}
{{- $reg := default .Values.image.registry "" }}
{{- $repo := .Values.core.image.repository }}
{{- $tag := default .Values.image.tag .Values.core.image.tag }}
{{- if $reg }}{{ $reg }}/{{ $repo }}:{{ $tag }}{{- else }}{{ $repo }}:{{ $tag }}{{- end }}
{{- end }}

{{/*
Agent image
*/}}
{{- define "fortuna.agentImage" -}}
{{- $reg := default .Values.image.registry "" }}
{{- $repo := .Values.agent.image.repository }}
{{- $tag := default .Values.image.tag .Values.agent.image.tag }}
{{- if $reg }}{{ $reg }}/{{ $repo }}:{{ $tag }}{{- else }}{{ $repo }}:{{ $tag }}{{- end }}
{{- end }}

{{/*
Dashboard image
*/}}
{{- define "fortuna.dashboardImage" -}}
{{- $reg := default .Values.image.registry "" }}
{{- $repo := .Values.dashboard.image.repository }}
{{- $tag := default .Values.image.tag .Values.dashboard.image.tag }}
{{- if $reg }}{{ $reg }}/{{ $repo }}:{{ $tag }}{{- else }}{{ $repo }}:{{ $tag }}{{- end }}
{{- end }}

{{/*
Core service host (for Agent and Dashboard proxy)
*/}}
{{- define "fortuna.coreServiceHost" -}}
fortuna-core.{{ include "fortuna.namespace" . }}.svc.cluster.local
{{- end }}
