module github.com/sapcc/netappsd

go 1.12

require (
	github.com/go-logr/logr v1.2.3
	github.com/go-openapi/runtime v0.19.21
	github.com/gorilla/mux v1.8.0
	github.com/netbox-community/go-netbox v0.0.0-20200923200002-49832662a6fd
	github.com/prometheus/client_golang v1.14.0
	github.com/prometheus/common v0.39.0
	github.com/rs/zerolog v1.29.0
	github.com/sapcc/go-bits v0.0.0-20230203091932-bc999fbc3108
)

replace github.com/netbox-community/go-netbox => github.com/stefanhipfel/go-netbox v0.0.0-20200928114340-fcd4119414a4
