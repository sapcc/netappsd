package main

import (
	"encoding/json"
	"fmt"
	"github.com/hosting-de-labs/go-netbox/netbox/models"
	"log"

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

func (f Filers) Fetch(nb *netbox.Netbox, region, query string) (updated bool, err error) {
	var (
		devices []models.Device
	)
	switch query {
	case "md":
		devices, err = getManilaFilers(nb, region)
	case "bb":
		devices, err = getCinderFilers(nb, region)
	case "bm":
		devices, err = getBareMetalFilers(nb, region)
	case "cp":
		devices, err = getControlPlaneFilers(nb, region)
	default:
		return false, fmt.Errorf("%s is not valide filer type", query)
	}
	if err != nil {
		return false, err
	}

	// hostnames/ips are not maintained in netbox for the filers, we have to
	// rely on the name of filers to determin the hosts
	nf := make(map[string]Filer, 0)
	for _, d := range devices {
		nf[*d.Name] = Filer{
			Name: *d.Name,
			Host: *d.Name + ".cc." + region + ".cloud.sap",
		}
	}

	// remove filers that exist no longer
	l := make([]string, 0)
	for _, d := range f {
		if _, ok := nf[d.Name]; !ok {
			l = append(l, d.Name)
			updated = true
		}
	}
	for _, n := range l {
		delete(f, n)
	}

	// add filer if it's not yet in the list
	for _, d := range nf {
		if _, ok := f[d.Name]; !ok {
			f[d.Name] = d
			updated = true
		}
	}

	return updated, nil
}
