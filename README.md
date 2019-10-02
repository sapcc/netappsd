## Netappsd
Netappsd queries netbox for netapp filers and write the name and host of the filers into configmap 
```
- name: __name_of_filer__
  host: __host_of_filer__
```

## Usage

```
Usage of bin/netappsd:
  -alsologtostderr
        log to standard error as well as files
  -configmap string
        configmap name (default "netapp-perf-etc")
  -key string
        configmap key (default "netapp-filers.yaml")
  -local
        run program out of cluster
  -log_backtrace_at value
        when logging hits line file:N, emit a stack trace
  -log_dir string
        If non-empty, write log files in this directory
  -logtostderr
        log to standard error instead of files
  -namespace string
        namespace
  -netbox-api-token string
        netbox token
  -netbox-host string
        netbox host
  -query string
        query
  -region string
        region
  -stderrthreshold value
        logs at or above this threshold go to stderr
  -v value
        log level for V logs
  -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging
```

