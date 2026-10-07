{{/* Image tag: component override, then image.tag, then the chart appVersion. */}}
{{- define "fortuna.tag" -}}
{{- .tag | default .root.Values.image.tag | default .root.Chart.AppVersion -}}
{{- end }}

{{/* Full image reference for a Fortuna component: (dict "root" $ "image" .Values.core.image) */}}
{{- define "fortuna.image" -}}
{{- $registry := trimSuffix "/" .root.Values.image.registry -}}
{{- $tag := include "fortuna.tag" (dict "root" .root "tag" .image.tag) -}}
{{- if $registry -}}
{{ printf "%s/%s:%s" $registry .image.repository $tag }}
{{- else -}}
{{ printf "%s:%s" .image.repository $tag }}
{{- end -}}
{{- end }}

{{- define "fortuna.imagePullSecrets" -}}
{{- with .Values.image.pullSecrets }}
imagePullSecrets:
{{- range . }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}

{{/* In-cluster DNS names. */}}
{{- define "fortuna.coreHost" -}}
fortuna-core.{{ .Release.Namespace }}.svc.cluster.local
{{- end }}

{{- define "fortuna.coreGrpcEndpoint" -}}
{{- .Values.agent.coreGrpcEndpoint | default (printf "%s:9090" (include "fortuna.coreHost" .)) -}}
{{- end }}

{{- define "fortuna.coreHttpEndpoint" -}}
{{- .Values.agent.coreHttpEndpoint | default (printf "http://%s:8080" (include "fortuna.coreHost" .)) -}}
{{- end }}

{{/*
Generated secret values, computed once per render and reused on upgrade.
Returns YAML; read it with (include "fortuna.generatedSecrets" . | fromYaml).
*/}}
{{- define "fortuna.generatedSecrets" -}}
{{- if not (hasKey .Values "__generatedSecrets") -}}
{{- $existing := (lookup "v1" "Secret" .Release.Namespace "fortuna-secrets") | default dict -}}
{{- $data := $existing.data | default dict -}}
{{- $pg := (lookup "v1" "Secret" .Release.Namespace "postgres-credentials") | default dict -}}
{{- $pgData := $pg.data | default dict -}}
{{- $postgresPassword := .Values.secrets.postgresPassword | default (get $pgData "POSTGRES_PASSWORD" | b64dec) | default (randAlphaNum 24) -}}
{{- $databaseUrl := .Values.secrets.databaseUrl | default (printf "postgres://%s:%s@postgres.%s.svc.cluster.local:5432/%s?sslmode=disable" .Values.postgresql.user $postgresPassword .Release.Namespace .Values.postgresql.database) -}}
{{- $out := dict
  "postgresPassword" $postgresPassword
  "databaseUrl" $databaseUrl
  "adminPassword" (.Values.secrets.adminPassword | default (get $data "admin-password" | b64dec) | default (printf "Fa1-%s" (randAlphaNum 20)))
  "riskEvaluatorPassword" (get $data "risk-evaluator-password" | b64dec | default (printf "Fr1-%s" (randAlphaNum 32)))
  "jwtSecret" (.Values.secrets.jwtSecret | default (get $data "jwt-secret" | b64dec) | default (randAlphaNum 48))
  "ingestToken" (.Values.secrets.ingestToken | default (get $data "ingest-token" | b64dec) | default (randAlphaNum 48))
  "podDetailEncryptionKey" (.Values.secrets.podDetailEncryptionKey | default (get $data "pod-detail-encryption-key" | b64dec) | default (randAlphaNum 32 | b64enc))
  "podDetailEncryptionKeyPrevious" (get $data "pod-detail-encryption-key-previous" | b64dec)
-}}
{{- $_ := set .Values "__generatedSecrets" $out -}}
{{- end -}}
{{- toYaml (get .Values "__generatedSecrets") -}}
{{- end }}

{{/*
Generated mTLS material, computed once per render and reused on upgrade.
The CA key is used only during rendering and never stored.
*/}}
{{- define "fortuna.generatedTLS" -}}
{{- if not (hasKey .Values "__generatedTLS") -}}
{{- $ns := .Release.Namespace -}}
{{- $ca := (lookup "v1" "Secret" $ns "fortuna-ca-cert") | default dict -}}
{{- $core := (lookup "v1" "Secret" $ns "fortuna-core-tls") | default dict -}}
{{- $agent := (lookup "v1" "Secret" $ns "fortuna-agent-tls") | default dict -}}
{{- $webhook := (lookup "v1" "Secret" $ns "fortuna-webhook-tls") | default dict -}}
{{- $out := dict -}}
{{- if and $ca.data $core.data $agent.data $webhook.data -}}
{{- $_ := set $out "caCrt" (get $ca.data "ca.crt" | b64dec) -}}
{{- $_ := set $out "coreCrt" (get $core.data "tls.crt" | b64dec) -}}
{{- $_ := set $out "coreKey" (get $core.data "tls.key" | b64dec) -}}
{{- $_ := set $out "agentCrt" (get $agent.data "tls.crt" | b64dec) -}}
{{- $_ := set $out "agentKey" (get $agent.data "tls.key" | b64dec) -}}
{{- $_ := set $out "webhookCrt" (get $webhook.data "tls.crt" | b64dec) -}}
{{- $_ := set $out "webhookKey" (get $webhook.data "tls.key" | b64dec) -}}
{{- else -}}
{{- $days := int .Values.tls.validityDays -}}
{{- $caCert := genCA "Fortuna CA" (int .Values.tls.caValidityDays) -}}
{{- $serverNames := list "fortuna-core" (printf "fortuna-core.%s" $ns) (printf "fortuna-core.%s.svc" $ns) (printf "fortuna-core.%s.svc.cluster.local" $ns) (printf "fortuna-webhook.%s.svc" $ns) (printf "fortuna-webhook.%s.svc.cluster.local" $ns) -}}
{{- $server := genSignedCert (printf "fortuna-core.%s.svc.cluster.local" $ns) (list "127.0.0.1") $serverNames $days $caCert -}}
{{- $client := genSignedCert "fortuna-agent" nil (list "fortuna-agent") $days $caCert -}}
{{- $_ := set $out "caCrt" $caCert.Cert -}}
{{- $_ := set $out "coreCrt" $server.Cert -}}
{{- $_ := set $out "coreKey" $server.Key -}}
{{- $_ := set $out "agentCrt" $client.Cert -}}
{{- $_ := set $out "agentKey" $client.Key -}}
{{- $_ := set $out "webhookCrt" $server.Cert -}}
{{- $_ := set $out "webhookKey" $server.Key -}}
{{- end -}}
{{- $_ := set .Values "__generatedTLS" $out -}}
{{- end -}}
{{- toYaml (get .Values "__generatedTLS") -}}
{{- end }}
