Defaults:
  auth_style: basic_auth
  use_insecure_tls: true
  username: {{ .Username }}
  password: {{ .Password }}
  exporters:
    - prom

Exporters:
  prom:
    exporter: Prometheus
    global_prefix: netapp_
    port: 13000

Pollers:
  {{ .Name }}:
    addr: {{ .Host }}
    datacenter: {{ .AvailabilityZone }}
    labels:
      - availability_zone: {{ .AvailabilityZone }}
      - filer: {{ .Name }}
    collectors:
      - Rest:
        - limited.yaml
      - RestPerf:
        - limited.yaml