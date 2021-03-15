package main

import (
	"fmt"
	"strings"

	"github.com/netbox-community/go-netbox/netbox/client/dcim"
	"github.com/netbox-community/go-netbox/netbox/models"
	"github.com/sapcc/atlas/pkg/netbox"
)

var (
	roleFiler    = "filer"
	manufacturer = "netapp"
	statusActive = "active"
	interfaces   = "False"

	deviceParams = &dcim.DcimDevicesListParams{
		Role:         &roleFiler,
		Manufacturer: &manufacturer,
		Status:       &statusActive,
		Interfaces:   &interfaces,
	}
)

func GetFilers(nb *netbox.Netbox, region, query string) (filers Filers, err error) {
	var (
		devices []models.DeviceWithConfigContext
	)
	switch query {
	case "md":
		devices, err = getManilaFilers(nb, region)
	case "bb":
		devices, err = getCinderFilers(nb, region)
		if err != nil {
			break
		}
		st, err := getActiveFilersByTag(nb, region, "cinder")
		if err != nil {
			break
		}
		devices = append(devices, st...)
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

func getManilaFilers(nb *netbox.Netbox, region string) ([]models.DeviceWithConfigContext, error) {
	query := "md"
	params := *deviceParams
	params.WithQ(&query)
	params.WithRegion(&region)
	devices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	devices = filterDeviceName(devices, query)

	// some control plane filers are used as manila filers as well
	query = "cp"
	tag := "manila"
	params.WithTag(&tag)
	moreDevices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	moreDevices = filterDeviceName(moreDevices, query)
	devices = append(devices, moreDevices...)
	return devices, nil
}

func getCinderFilers(nb *netbox.Netbox, region string) ([]models.DeviceWithConfigContext, error) {
	query := "bb"
	params := *deviceParams
	params.WithQ(&query)
	params.WithRegion(&region)
	devices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	return filterDeviceName(devices, query), nil
}

func getBareMetalFilers(nb *netbox.Netbox, region string) ([]models.DeviceWithConfigContext, error) {
	query := "bm"
	params := *deviceParams
	params.WithQ(&query)
	params.WithRegion(&region)
	devices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	return filterDeviceName(devices, query), nil
}

func getControlPlaneFilers(nb *netbox.Netbox, region string) ([]models.DeviceWithConfigContext, error) {
	query := "cp"
	params := *deviceParams
	params.WithQ(&query)
	params.WithRegion(&region)
	devices, err := nb.DevicesByParams(params)
	if err != nil {
		return nil, err
	}
	devices = filterDeviceName(devices, query)

	// filter manila tag
	return filterDeviceWithTag(devices, "manila"), nil
}

func filterDeviceName(devices []models.DeviceWithConfigContext, s string) []models.DeviceWithConfigContext {
	results := []models.DeviceWithConfigContext{}
	for _, device := range devices {
		if strings.Contains(*device.Name, query) {
			results = append(results, device)
		}
	}
	return results
}

// remove devices with tag
func filterDeviceWithTag(devices []models.DeviceWithConfigContext, t string) []models.DeviceWithConfigContext {
	results := []models.DeviceWithConfigContext{}

device:
	for _, device := range devices {
		tags := device.Tags
		for _, tag := range tags {
			if *tag.Name == t {
				continue device
			}
		}
		results = append(results, device)
	}

	return results
}

func getActiveFilersByTag(nb *netbox.Netbox, region, tag string) ([]models.DeviceWithConfigContext, error) {
	return nb.DevicesByParams(dcim.DcimDevicesListParams{
		Role:   &roleFiler,
		Status: &statusActive,
		Tag:    &tag,
		Region: &region,
	})
}
