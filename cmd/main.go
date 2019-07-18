package main

import (
	"os"

	klog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sapcc/atlas/pkg/netbox"

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
	devices, err := netbox.DevicesByRegion("bm091", "netapp", "qa-de-1", "1")
	logError(err)
	logger.Log("devices#", len(devices))
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

func newNetboxClient() *netbox.Netbox {
	c, err := netbox.NewDefaultHost(` c1d40ae380689e55384c26f1e5303a36f618ca73`)
	logError(err)
	return c
}
