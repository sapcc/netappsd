## NetAppSD

NetAppSD is an automatic scaler for NetApp's Harvest exporters running in a
Kubernetes cluster. It discovers NetApp filers from Netbox and starts an
exporter instance for each of them.

NetAppSD operates in two modes: master and worker. The master node discovers
NetApp filers from Netbox and reconciles one Deployment per filer, named after
the filer, applying a Deployment template it reads from a mounted ConfigMap.
The worker node runs as a sidecar in the NetApp Harvest pod. It knows its own
filer name (injected by the master as the `FILER_NAME` env var), fetches the
filer details from the master's "/filer/{name}" endpoint, and renders the
configuration file for the Harvest exporter.

### Discovery and reconciliation

The master runs two independent loops:

- **Discovery** (every 5 minutes): queries Netbox for the filers matching the
  configured region and tag, and probes each one (an HTTP call to the filer's
  NetApp API) to confirm it is reachable. Each successful probe refreshes the
  filer's last-probe timestamp. Filers that disappear from the Netbox result or
  become non-active are pruned.
- **Reconcile** (every 30 seconds, and immediately after each discovery):
  compares the desired set of filers against the Deployments currently running,
  then creates, updates, or deletes Deployments to converge.

A Deployment is created for every active filer that has been probed
successfully within the last 15 minutes (three discovery cycles — a filer is
only considered stale after several consecutive probe failures, so a transient
blip does not tear down its exporter). A Deployment is deleted when its filer
disappears from Netbox, becomes non-active, or goes unreachable past that
window.

### Handling template changes

Each managed Deployment carries a `netappsd/spec-hash` annotation holding a
hash of its rendered spec. On every reconcile the master reloads the template
from the mounted ConfigMap and recomputes the hash; if it differs from the live
Deployment, the master updates it (triggering a rolling restart so the worker
picks up the change). To avoid restarting every worker at once, at most 10
Deployments are updated per reconcile cycle; the rest follow on subsequent
cycles.

### Labels and cleanup

Managed Deployments are labeled `app.kubernetes.io/managed-by=netappsd`, carry a
`netappsd/filer=<name>` label identifying their filer, and a
`netappsd/service=<tag>` label scoping them to the master's service. The service
label ensures masters for different services (e.g. cinder, manila, apod) do not
manage or delete each other's Deployments.

Because the master creates these Deployments at runtime, Helm does not track
them. A `pre-delete` hook Job deletes all Deployments labeled
`app.kubernetes.io/managed-by=netappsd` on `helm uninstall` so they are not
orphaned.

## Usage

### Master
```
Usage:
  netappsd master [flags]

Flags:
  -h, --help                          help for master
  -l, --listen-addr string            The address to listen on (default ":8080")
      --netbox-host string            The netbox host to query (default "netbox.staging.cloud.sap")
      --netbox-token string           The token to authenticate against netbox
  -r, --region string                 The region to filter netbox devices
  -t, --tag string                    The tag to filter netbox devices
      --deployment-template string    The path to the per-filer deployment template (default "/etc/netappsd/deployment.yaml.tpl")
      --managed-label string          The label used to identify deployments managed by netappsd (default "app.kubernetes.io/managed-by=netappsd")

Global Flags:
  -d, --debug   Enable debug logging
```

### Worker
```
Usage:
  netappsd worker [flags]

Flags:
  -h, --help                   help for worker
  -l, --listen-addr string     The address to listen on (default ":8082")
  -m, --master-url string      The url of the netappsd-master (default "http://localhost:8080")
  -o, --output-file string     The path to the output file (default "harvest.yaml")
  -t, --template-file string   The path to the template file (default "harvest.yaml.tpl")
  -f, --filer-name string      The name of the filer to export (defaults to FILER_NAME env)

Global Flags:
  -d, --debug   Enable debug logging
```
