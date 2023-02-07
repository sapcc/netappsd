module github.com/sapcc/netappsd

go 1.12

require (
	github.com/go-kit/kit v0.9.0
	github.com/go-openapi/runtime v0.19.21
	github.com/netbox-community/go-netbox v0.0.0-20200923200002-49832662a6fd
	github.com/sapcc/go-bits v0.0.0-20230203091932-bc999fbc3108
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/netbox-community/go-netbox => github.com/stefanhipfel/go-netbox v0.0.0-20200928114340-fcd4119414a4
