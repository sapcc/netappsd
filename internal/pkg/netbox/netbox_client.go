package netbox

import (
	"context"
	"fmt"
	"strings"

	"github.com/netbox-community/go-netbox/v4"
)

type Client struct {
	*netbox.APIClient
}

func NewClient(host, token string) (Client, error) {
	c := netbox.NewAPIClientFor(host, token)
	return Client{c}, nil
}

func (c Client) getFilers(ctx context.Context, region, tag string) ([]Filer, error) {
	filers, err := c.getFilerRacks(ctx, region, tag)
	if err != nil {
		return nil, err
	}
	if tag == "manila" {
		clusters, err := c.getManilaFilerClusters(ctx, region)
		if err != nil {
			return nil, err
		}
		filers = append(filers, clusters...)
	}
	return filers, nil
}

// getFilers returns a list of NetApp Filers in a region with a specific tag.
// The filers are modeld as Dcim Devices with "filer" role. The Filer does not
// has interfaces and ip addresses set, so the function extracts the ip address
// from its first node.
//
// EG https://netbox.global.cloud.sap/dcim/devices/?region_id=19&role_id=13&manufacturer_id=11&tenant_id=1&interfaces=False
func (c Client) getFilerRacks(ctx context.Context, region, tag string) ([]Filer, error) {
	netappFilers := make([]Filer, 0)
	devices := make([]netbox.DeviceWithConfigContext, 0)

	var limit int32 = 100
	var offset int32 = 0
	for {
		d, _, err := c.DcimAPI.
			DcimDevicesList(ctx).
			Role([]string{"filer"}).
			Manufacturer([]string{"netapp"}).
			Region([]string{region}).
			Tag([]string{tag}).
			Interfaces(false).
			Limit(limit).
			Offset(offset).
			Execute()
		if err != nil {
			return nil, err
		}
		devices = append(devices, d.Results...)
		if d.GetNext() != "" {
			offset += limit
		} else {
			break
		}
	}

	for _, device := range devices {
		deviceName := *device.Name.Get()
		deviceAZ := ""
		deviceIp := ""
		deviceStatus := ""

		if az, ok := device.Site.GetNameOk(); ok {
			deviceAZ = strings.ToLower(*az)
		}
		if device.Status != nil {
			if val, ok := device.Status.GetValueOk(); ok {
				deviceStatus = string(*val)
			}
		}

		// Primary ip address is not set on the filer, but on the first node
		bays, _, err := c.DcimAPI.DcimDeviceBaysList(ctx).
			DeviceId([]int32{device.Id}).
			Execute()
		if err != nil {
			return nil, err
		}
		for _, deviceBay := range bays.Results {
			if deviceBay.InstalledDevice.IsSet() {
				installedDevice, _, err := c.DcimAPI.DcimDevicesRetrieve(ctx, deviceBay.InstalledDevice.Get().Id).Execute()
				if err != nil {
					return nil, err
				}
				if installedDevice.PrimaryIp4.IsSet() {
					if addr, ok := installedDevice.PrimaryIp4.Get().GetAddressOk(); ok {
						deviceIp = *addr
						break
					}
				}
			}
		}

		netappFilers = append(netappFilers, Filer{
			Name:             deviceName,
			Host:             fmt.Sprintf("%s.cc.%s.cloud.sap", deviceName, region),
			Ip:               deviceIp,
			Status:           deviceStatus,
			AvailabilityZone: deviceAZ,
		})
	}

	return netappFilers, nil
}

// getManilaFlexGroupClusters returns a list of Manila FlexGroup clusters. It's
// modeled as Virtualization Cluster with "NetApp Storage Cluster" type in Netbox.
//
// EG https://netbox.global.cloud.sap/virtualization/clusters/?tag=manila&type_id=25
func (c Client) getManilaFilerClusters(ctx context.Context, region string) ([]Filer, error) {
	filers := make([]Filer, 0)

	clusters, _, err := c.VirtualizationAPI.
		VirtualizationClustersList(ctx).
		Region([]string{region}).
		// TypeN([]string{"NetApp Storage Cluster"}).
		Tag([]string{"manila"}).
		Execute()
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters.Results {
		clusterId := cluster.Id
		clusterName := cluster.Name
		clusterIpAddr := ""
		clusterStatus := ""
		clusterAZ := ""

		if cluster.Status != nil {
			if val, ok := cluster.Status.GetValueOk(); ok {
				clusterStatus = string(*val)
			}
		}

		res, _, err := c.
			DcimAPI.
			DcimDevicesList(ctx).
			ClusterId([]*int32{&clusterId}).
			Role([]string{"filer"}).
			Execute()
		if err != nil {
			return nil, err
		}
		for i := range res.Results {
			// get device site
			if res.Results[i].PrimaryIp4.IsSet() {
				if addr, ok := res.Results[0].PrimaryIp4.Get().GetAddressOk(); ok {
					clusterIpAddr = *addr
					break
				}
			}
		}
		for i := range res.Results {
			if site, ok := res.Results[i].Site.GetNameOk(); ok {
				clusterAZ = strings.ToLower(*site)
				break
			}
		}

		filers = append(filers, Filer{
			Name:             clusterName,
			Host:             fmt.Sprintf("%s.cc.%s.cloud.sap", clusterName, region),
			Ip:               clusterIpAddr,
			Status:           clusterStatus,
			AvailabilityZone: clusterAZ,
		})
	}
	return filers, nil
}
