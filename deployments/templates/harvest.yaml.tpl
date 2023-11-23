Defaults:
  auth_style: basic_auth
  use_insecure_tls: true
  credential_files: /etc/harvest/secrets.yaml
  collectors:
    - Zapi
  exporters:
    - prom

Exporters:
  prom:
    exporter: Prometheus
    port_range: 13000-13999
    global_prefix: netapp_

Pollers:
  - name: {{ .Name }}
    addr: {{ .Host }}
    datacenter: {{ .AvailabilityZone }}
