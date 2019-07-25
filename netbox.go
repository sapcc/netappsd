package netappsd

import (
	"fmt"

	netboxclient "github.com/chuan137/go-netbox/netbox/client"
	"github.com/chuan137/go-netbox/netbox/client/dcim"
	"github.com/chuan137/go-netbox/netbox/models"
	runtimeclient "github.com/go-openapi/runtime/client"
)

const netboxDefaultHost = "netbox.global.cloud.sap"

type Netbox struct {
	client *netboxclient.NetBox
}

func NewNetbox(token string) (*Netbox, error) {
	return newNetbox(netboxDefaultHost, token)
}

func newNetbox(host, token string) (*Netbox, error) {
	client, err := newClient(host, token)
	if err != nil {
		return nil, err
	}
	return &Netbox{client: client}, nil
}

func newClient(host, token string) (*netboxclient.NetBox, error) {
	tlsClient, err := runtimeclient.TLSClient(runtimeclient.TLSClientOptions{InsecureSkipVerify: true})
	if err != nil {
		return nil, err
	}

	transport := runtimeclient.NewWithClient(host, netboxclient.DefaultBasePath, []string{"https"}, tlsClient)
	transport.DefaultAuthentication = runtimeclient.APIKeyAuth("Authorization", "header", fmt.Sprintf("Token %v", token))
	c := netboxclient.New(transport, nil)
	return c, nil
}

// AcitveDevicesByParams retrievs all active devices with custom parameters
func (nb *Netbox) ActiveDevicesByParams(query string, params *dcim.DcimDevicesListParams) ([]models.Device, error) {
	res := make([]models.Device, 0)
	activeStatus := "1"
	limit := int64(100)
	params.WithQ(&query)
	params.WithStatus(&activeStatus)
	params.WithLimit(&limit)
	for {
		offset := int64(0)
		if params.Offset != nil {
			offset = *params.Offset + limit
		}
		params.Offset = &offset
		list, err := nb.client.Dcim.DcimDevicesList(params, nil)
		if err != nil {
			return res, err
		}
		for _, device := range list.Payload.Results {
			res = append(res, *device)
		}
		if list.Payload.Next == nil {
			break
		}
	}
	return res, nil
}

func (nb *Netbox) QueryNetappFilers(query, region string) (Filers, error) {
	params := dcim.NewDcimDevicesListParams()
	role := "filer"
	manufacturer := "netapp"
	params.WithRole(&role)
	params.WithRegion(&region)
	params.WithManufacturer(&manufacturer)
	devices, err := nb.ActiveDevicesByParams(query, params)
	if err != nil {
		return nil, err
	}

	// hostnames/ips are not maintained in netbox for the filers, we have to
	// rely on the name of filers to determin the hosts
	filers := make(Filers, 0)
	for _, d := range devices {
		if d.ParentDevice == nil {
			filers = append(filers, Filer{
				Name: *d.Name,
				Host: *d.Name + ".cc." + region + ".cloud.sap",
			})
		}
	}
	return filers, nil
}
