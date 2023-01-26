module github.com/sapcc/netappsd

go 1.12

require (
	github.com/go-kit/kit v0.9.0
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-openapi/runtime v0.19.21
	github.com/netbox-community/go-netbox v0.0.0-20200923200002-49832662a6fd
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/netbox-community/go-netbox => github.com/stefanhipfel/go-netbox v0.0.0-20200928114340-fcd4119414a4
