## NetAppSD

NetAppSD discovers NetApp filers from Netbox and provides them as scraping targets for NetApp harvest exporters running in a Kubernetes cluster.

NetAppSD operates in either master or worker modes. The master node is responsible for discovering NetApp filers and monitoring the worker pods. The worker node runs as a sidecar in the NetApp harvest pod, fetching the filer to scrape from the master's endpoint "/next/filer" for the harvest exporter.

The process for assigning a filer to a worker:

1. A worker requests the master to assign a filer to work on.
2. The master checks which filers are already being worked on by examining the "filer" label of the worker pods.
3. The master selects a filer that is not currently being scraped and sets the "filer" label on the requesting worker.
4. The master returns the filer to the worker.
5. The worker creates the configuration file for the harvest exporter.

## Usage

### Master

Usage:
  netappsd master [flags]

Flags:
  -h, --help                  help for master
  -l, --listen-addr string    The address to listen on (default ":8080")
      --netbox-host string    The netbox host to query (default "netbox.staging.cloud.sap")
      --netbox-token string   The token to authenticate against netbox
  -r, --region string         The region to filter netbox devices
  -t, --tag string            The tag to filter netbox devices
  -w, --worker string         The deployment name of workers
      --worker-label string   The label of worker pods

Global Flags:
  -d, --debug   Enable debug logging

### Worker

Usage:
  netappsd worker [flags]

Flags:
  -h, --help                   help for worker
  -l, --listen-addr string     The address to listen on (default ":8082")
  -m, --master-url string      The url of the netappsd-master (default "http://localhost:8080")
  -o, --output-file string     The path to the output file (default "harvest.yaml")
  -t, --template-file string   The path to the template file (default "harvest.yaml.tpl")

Global Flags:
  -d, --debug   Enable debug logging

