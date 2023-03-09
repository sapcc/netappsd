module github.com/sapcc/netappsd

go 1.12

require (
	github.com/go-openapi/runtime v0.19.21
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/netbox-community/go-netbox v0.0.0-20200923200002-49832662a6fd
	github.com/prometheus/client_golang v1.14.0
	github.com/prometheus/common v0.39.0
	github.com/rs/zerolog v1.29.0
	github.com/sapcc/go-bits v0.0.0-20230203091932-bc999fbc3108
	github.com/urfave/negroni v1.0.0
)

replace github.com/netbox-community/go-netbox => github.com/stefanhipfel/go-netbox v0.0.0-20200928114340-fcd4119414a4
