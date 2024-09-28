{{- define "istio-upgrade-worker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "istio-upgrade-worker.podlabels" -}}
{{- with .Values.podLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}
