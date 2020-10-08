module netappsd

go 1.12

require (
	github.com/go-kit/kit v0.8.0
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/netbox-community/go-netbox v0.0.0-20200923200002-49832662a6fd
	github.com/sapcc/atlas v0.0.0-20201008114448-b4b0ed639620
	github.com/sirupsen/logrus v1.4.2
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/netbox-community/go-netbox => github.com/stefanhipfel/go-netbox v0.0.0-20200928114340-fcd4119414a4
