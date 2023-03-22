## Netappsd


Netappsd discovers NetApp filers from Netbox, and provides them as targets to harvest instances.

![Discovery and Observing](images/netappsd-arch.jpg?raw=true "Netappsd Arch")


## Usage

```
Usage of bin/netappsd:
  -address string
        server address (default "0.0.0.0:8000")
  -config-dir string
        Directory where config and template files are located (default "./")
  -discover-interval duration
        time interval between dicovering filers from netbox (default 5m0s)
  -log-level string
        log level (default "info")
  -netbox-api-token string
        netbox token
  -netbox-host string
        netbox host (default "netbox.global.cloud.sap")
  -query string
        query
  -region string
        region
  -update-interval duration
        time interval between state updates from prometheus (default 1m0s)
```

And following env variables must be provided.

```
- NETAPP_USERNAME
- NETAPP_PASSWORD
- NETAPPSD_PROMETHEUS_OBSERVE_URL
- NETAPPSD_PROMETHEUS_OBSERVE_QUERY
- NETAPPSD_PROMETHEUS_OBSERVE_LABEL

```
