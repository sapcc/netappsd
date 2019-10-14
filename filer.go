package main

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

func (f Filers) QueryNetbox(nb *netbox.Netbox, region, query string) (updated bool, err error) {
	nf, err := queryNetbox(nb, region, query)
	if err != nil {
		return false, err
	}

	updates := make(map[string]bool)

	// remove filers that exist no longer
	l := make([]string, 0)
	for _, d := range f {
		if _, ok := nf[d.Name]; !ok {
			l = append(l, d.Name)
			updates[d.Name] = true
		}
	}
	for _, n := range l {
		delete(f, n)
	}

	// add filer if it's not yet in the list
	for _, d := range nf {
		if _, ok := f[d.Name]; !ok {
			f[d.Name] = d
			updates[d.Name] = true
		}
	}

	// updated?
	if len(updates) > 0 {
		updated = true
	} else {
		updated = false
	}

	return updated, nil
}

func queryNetbox(nb *netbox.Netbox, region, query string) (Filers, error) {
	params := dcim.NewDcimDevicesListParams()
	role := "filer"
	manufacturer := "netapp"
	status := "1" // active
	interfaces := "False"
	params.WithQ(&query)
	params.WithRole(&role)
	params.WithRegion(&region)
	params.WithManufacturer(&manufacturer)
	params.WithStatus(&status)
	params.WithInterfaces(&interfaces)
	devices, err := nb.DevicesByParams(*params)
	if err != nil {
		return nil, err
	}

	// hostnames/ips are not maintained in netbox for the filers, we have to
	// rely on the name of filers to determin the hosts
	fl := make(map[string]Filer, 0)
	for _, d := range devices {
		fl[*d.Name] = Filer{
			Name: *d.Name,
			Host: *d.Name + ".cc." + region + ".cloud.sap",
		}
	}
	return fl, nil
}
