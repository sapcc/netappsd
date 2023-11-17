package netbox

import (
	"context"
	"fmt"
	"strconv"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/netbox-community/go-netbox/v3/netbox/client"
	"github.com/netbox-community/go-netbox/v3/netbox/client/dcim"
	"github.com/netbox-community/go-netbox/v3/netbox/models"
)

type Client struct {
	client *client.NetBoxAPI
}

func NewClient(host, token string) (Client, error) {
	tlsClient, err := httptransport.TLSClient(httptransport.TLSClientOptions{InsecureSkipVerify: true})
	if err != nil {
		return Client{}, err
	}
	transport := httptransport.NewWithClient(host, client.DefaultBasePath, []string{"https"}, tlsClient)
	if token != "" {
		transport.DefaultAuthentication = httptransport.APIKeyAuth("Authorization", "header", fmt.Sprintf("Token %v", token))
	}
	c := client.New(transport, nil)
	return Client{c}, nil
}

func (nb Client) FetchDevices(params dcim.DcimDevicesListParams) ([]*models.DeviceWithConfigContext, error) {
	limit := int64(100)
	offset := int64(0)
	params.WithLimit(&limit)
	params.WithOffset(&offset)
	params.WithTimeout(30 * time.Second)
	params.WithContext(context.Background())
	res := make([]*models.DeviceWithConfigContext, 0)
	for {
		list, err := nb.client.Dcim.DcimDevicesList(&params, nil)
		if err != nil {
			return res, err
		}
		res = append(res, list.Payload.Results...)
		if list.Payload.Next != nil {
			offset = *params.Offset + limit
			params.Offset = &offset
		} else {
			break
		}
	}
	return res, nil
}

func (nb Client) GetDeviceBaysByDeviceID(deviceID int64) ([]*models.DeviceBay, error) {
	id := strconv.FormatInt(deviceID, 10)
	params := dcim.NewDcimDeviceBaysListParamsWithTimeout(30 * time.Second)
	params.WithDeviceID(&id)
	res, err := nb.client.Dcim.DcimDeviceBaysList(params, nil)
	if err != nil {
		return nil, err
	}
	return res.Payload.Results, nil
}

func (nb Client) GetDeviceByID(id int64) (*models.DeviceWithConfigContext, error) {
	params := dcim.DcimDevicesReadParams{ID: id}
	params.WithTimeout(30 * time.Second)
	params.WithContext(context.Background())
	r, err := nb.client.Dcim.DcimDevicesRead(&params, nil)
	if err != nil {
		return nil, err
	}
	return r.Payload, nil
}
