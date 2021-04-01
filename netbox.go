package main

import (
	"fmt"
	"strings"

	"github.com/netbox-community/go-netbox/netbox/client/dcim"
	"github.com/netbox-community/go-netbox/netbox/models"
)

var ()

func GetFilers(nb *Netbox, region, query string) (filers Filers, err error) {
	switch query {
	case "md", "manila":
		filers, err = getFilersByTag(nb, region, "manila")
	case "bb", "cinder":
		filers, err = getFilersByTag(nb, region, "cinder")
	case "bm", "baremetal":
		filers, err = getFilersByTag(nb, region, "baremetal")
	case "cp", "control-plane", "control_plane":
		filers, err = getFilersByTag(nb, region, "control_plane")
	default:
		err = fmt.Errorf("%s is not valide filer type", query)
	}
	if err != nil {
		return nil, err
	}
	return filers, nil
}

func getFilersByTag(nb *Netbox, region, tag string) (Filers, error) {
	var (
		roleFiler    = "filer"
		manufacturer = "netapp"
		statusActive = "active"
		interfaces   = "False"
	)
	devices, err := nb.FetchDevices(dcim.DcimDevicesListParams{
		Role:         &roleFiler,
		Status:       &statusActive,
		Manufacturer: &manufacturer,
		Interfaces:   &interfaces,
		Tag:          &tag,
		Region:       &region,
	})
	if err != nil {
		return nil, err
	}
	return makeFilers(nb, devices), nil
}

func makeFilers(nb *Netbox, devices []*models.DeviceWithConfigContext) Filers {
	// IP address is not maintained in netbox for the filer cluster, therefore
	// filer name is used to determin the host name.
	//
	// TODO: Use the ip address of the first node as the host ip. To do that,
	// one should read the IP of the installed device on the first node
	// bay.
	filers := make(Filers)
	for _, d := range devices {
		// Ignore filer cluster with no nodes
		if deviceBays, err := nb.GetDeviceBaysByDeviceID(d.ID); err == nil {
			nd := 0
			for _, dd := range deviceBays {
				if dd.InstalledDevice != nil {
					nd = nd + 1
				}
			}
			if nd > 0 {
				filers[*d.Name] = Filer{
					Name: *d.Name,
					Host: *d.Name + ".cc." + region + ".cloud.sap",
					AZ:   strings.ToLower(*d.Site.Name),
				}
			}
		}
	}
	return filers
}
