package main

import (
	"os"

	"github.com/chuan137/go-netbox/netbox/client/dcim"
	"github.com/chuan137/go-netbox/netbox/models"
	klog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"netappsd"
)

var (
	logger klog.Logger
)

func init() {
	logger = klog.NewLogfmtLogger(klog.NewSyncWriter(os.Stdout))
}

func main() {
	cm, err := netappsd.NewConfigMapOutofCluster("test-cm", "netapp2", logger)
	logError(err)

	filers := getFilers()
	err = cm.Write("netapp-filers.conf", filers.YamlString())
	logError(err)

	netbox := newNetboxClient()
	params := dcim.NewDcimDevicesListParams()
	role := "filer"
	manufacturer := "netapp"
	region := "qa-de-1"
	params.WithRole(&role)
	params.WithRegion(&region)
	params.WithManufacturer(&manufacturer)
	devices, err := netbox.ActiveDevicesByParams("bb093", params)
	var filerDevices []models.Device
	for _, d := range devices {
		if d.ParentDevice == nil {
			filerDevices = append(filerDevices, d)
		}
	}
	logError(err)
	logger.Log("filers#", len(filerDevices))
}

func logError(err error) {
	if err != nil {
		level.Error(logger).Log("msg", err)
	}
}

func getFilers() netappsd.Filers {
	f := make(netappsd.Filers, 0)
	f = append(f, &netappsd.Filer{Name: "bb98", Host: "netapp-bb98.cloud.sap"})
	f = append(f, &netappsd.Filer{Name: "bb99", Host: "netapp-bb99.cloud.sap"})
	return f
}

func newNetboxClient() *netappsd.Netbox {
	c, err := netappsd.NewNetbox(`c1d40ae380689e55384c26f1e5303a36f618ca73`)
	logError(err)
	return c
}
