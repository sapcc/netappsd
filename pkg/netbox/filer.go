package netbox

import (
	"fmt"
	"strings"

	"github.com/netbox-community/go-netbox/netbox/client/dcim"
	"github.com/netbox-community/go-netbox/netbox/models"
)

type Filer struct {
	Name             string `json:"name" yaml:"name"`
	Host             string `json:"host" yaml:"host"`
	AvailabilityZone string `json:"availability_zone" yaml:"availability_zone"`
	IP               string `json:"ip,omitempty" yaml:"ip,omitempty"`
}

type Filers []Filer

func (nb Client) GetFilers(region, query string) (filers Filers, err error) {
	switch query {
	case "md", "manila":
		filers, err = getFilersByTag(nb, region, "manila")
	case "bb", "cinder":
		filers, err = getFilersByTag(nb, region, "cinder")
	case "bm", "baremetal":
		filers, err = getFilersByTag(nb, region, "baremetal")
	case "apod", "cp", "control-plane", "control_plane":
		filers, err = getFilersByTag(nb, region, "apod")
	default:
		err = fmt.Errorf("%s is not valide filer type", query)
	}
	if err != nil {
		return nil, err
	}
	return filers, nil
}

func getFilersByTag(nb Client, region, tag string) (Filers, error) {
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
	return makeFilers(nb, region, devices), nil
}

func makeFilers(nb Client, region string, devices []*models.DeviceWithConfigContext) Filers {
	// IP address is not maintained in netbox for the filer cluster, therefore
	// filer name is used to determin the host name.
	//
	// TODO: Use the ip address of the first node as the host ip. To do that,
	// one should read the IP of the installed device on the first node
	// bay.
	filers := make(Filers, 0)
	for _, d := range devices {
		// Ignore filer cluster with no nodes
		if deviceBays, err := nb.GetDeviceBaysByDeviceID(d.ID); err == nil {
			hasChildDevice := false
			ip := ""
			for _, node := range deviceBays {
				if node.InstalledDevice != nil {
					hasChildDevice = true
					if ip == "" {
						d, err := nb.GetDeviceByID(node.InstalledDevice.ID)
						if err != nil {
							continue
						}
						if d.PrimaryIp4 != nil {
							s := strings.Split(*d.PrimaryIp4.Address, "/")
							ip = s[0]
						}
					}
				}
			}
			if hasChildDevice {
				filers = append(filers, Filer{
					Name:             *d.Name,
					Host:             *d.Name + ".cc." + region + ".cloud.sap",
					AvailabilityZone: strings.ToLower(*d.Site.Name),
					IP:               ip,
				})
			}
		}
	}
	return filers
}
