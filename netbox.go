package main

import (
	"github.com/hosting-de-labs/go-netbox/netbox/client/dcim"
	"github.com/hosting-de-labs/go-netbox/netbox/models"
	"github.com/sapcc/atlas/pkg/netbox"
)

var (
	role         = "filer"
	manufacturer = "netapp"
	status       = "1"
	interfaces   = "False"

	deviceParams = &dcim.DcimDevicesListParams{
		Role:         &role,
		Manufacturer: &manufacturer,
		Status:       &status,
		Interfaces:   &interfaces,
	}
)

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
