package main

import (
	stdlog "log"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/spf13/cobra"
)

var (
	log logr.Logger
)

var cmd = &cobra.Command{
	Use:   "netappsd",
	Short: "A simple network application service daemon",
	Long:  `A simple network application service daemon`,
}

func main() {
	cmd.AddCommand(cmdMaster)
	cmd.AddCommand(cmdWorker)
	if err := cmd.Execute(); err != nil {
		log.Error(err, "Failed to execute command")
	}
}

func init() {
	log = stdr.NewWithOptions(stdlog.New(os.Stderr, "", stdlog.LstdFlags), stdr.Options{LogCaller: stdr.All})
}
