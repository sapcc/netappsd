package main

import (
	"os"

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
	err = cm.Write("test-field", `a: test-data
b: fun to do
`)
	logError(err)
}

func logError(err error) {
	if err != nil {
		level.Error(logger).Log("msg", err)
	}
}
