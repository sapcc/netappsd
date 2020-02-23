package main

import (
	"fmt"
	"strings"

	"github.com/hosting-de-labs/go-netbox/netbox/client/dcim"
	"github.com/hosting-de-labs/go-netbox/netbox/models"
	"github.com/sapcc/atlas/pkg/netbox"
)

var (
	role         = "filer"
	manufacturer = "netapp"
	status       = "active"
	interfaces   = "False"

	deviceParams = &dcim.DcimDevicesListParams{
		Role:         &role,
		Manufacturer: &manufacturer,
		Status:       &status,
		Interfaces:   &interfaces,
	}
)

func GetFilers(nb *netbox.Netbox, region, query string) (filers Filers, err error) {
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
		return nil, fmt.Errorf("%s is not valide filer type", query)
	}
	if err != nil {
		return nil, err
	}

	// hostnames/ips are not maintained in netbox for the filers, we have to
	// rely on the name of filers to determin the hosts
	filers = Filers{}
	for _, d := range devices {
		filers[*d.Name] = Filer{
			Name: *d.Name,
			Host: *d.Name + ".cc." + region + ".cloud.sap",
			AZ:   strings.ToLower(*d.Site.Name),
		}
	}
	return filers, nil
}

func getManilaFilers(nb *netbox.Netbox, region string) ([]models.Device, error) {
	query := "md"
	params := *deviceParams
	params.WithQ(&query)
	params.WithRegion(&region)
	devices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}

	// some control plane filers are used as manila filers as well
	query = "cp"
	tag := "manila"
	params.WithTag(&tag)
	moreDevices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	devices = append(devices, moreDevices...)
	return devices, nil
}

func getCinderFilers(nb *netbox.Netbox, region string) ([]models.Device, error) {
	query := "bb"
	params := *deviceParams
	params.WithQ(&query)
	params.WithRegion(&region)
	devices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	return devices, nil
}

func getBareMetalFilers(nb *netbox.Netbox, region string) ([]models.Device, error) {
	query := "bm"
	params := *deviceParams
	params.WithQ(&query)
	params.WithRegion(&region)
	devices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	return devices, nil
}

func getControlPlaneFilers(nb *netbox.Netbox, region string) ([]models.Device, error) {
	query := "cp"
	params := *deviceParams
	params.WithQ(&query)
	params.WithRegion(&region)
	devices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	return devices, nil
}
