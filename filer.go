package netappsd

import (
	"encoding/json"
	"log"

	"github.com/hosting-de-labs/go-netbox/netbox/client/dcim"
	"github.com/sapcc/atlas/pkg/netbox"
	"gopkg.in/yaml.v2"
)

type Filer struct {
	Name string `json:"name" yaml:"name"`
	Host string `json:"host" yaml:"host"`
}

type Filers map[string]Filer

func NewFilers() Filers {
	return make(map[string]Filer)
}

func (f Filers) list() []Filer {
	lf := make([]Filer, 0)
	for _, ff := range f {
		lf = append(lf, ff)
	}
	return lf
}

func (f Filers) JsonString() string {
	s, err := json.Marshal(f.list())
	if err != nil {
		log.Fatal(err)
	}
	return string(s)
}

func (f Filers) YamlString() string {
	s, err := yaml.Marshal(f.list())
	if err != nil {
		log.Fatal(err)
	}
	return string(s)
}

func (f Filers) QueryNetbox(nb *netbox.Netbox, query, region string) (bool, error) {
	params := dcim.NewDcimDevicesListParams()
	role := "filer"
	manufacturer := "netapp"
	status := "1" // active
	params.WithQ(&query)
	params.WithRole(&role)
	params.WithRegion(&region)
	params.WithManufacturer(&manufacturer)
	params.WithStatus(&status)
	devices, err := nb.DevicesByParams(*params)
	if err != nil {
		return false, err
	}

	// hostnames/ips are not maintained in netbox for the filers, we have to
	// rely on the name of filers to determin the hosts
	for _, d := range devices {
		if d.ParentDevice == nil {
			f[*d.Name] = Filer{
				Name: *d.Name,
				Host: *d.Name + ".cc." + region + ".cloud.sap",
			}
		}
	}
	return true, nil
}
