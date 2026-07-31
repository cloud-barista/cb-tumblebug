{{- define "cb.labels" -}}
app.kubernetes.io/part-of: cb-tumblebug
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
