package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sapcc/netappsd/cmd/master"
	"github.com/sapcc/netappsd/cmd/worker"
	"github.com/sapcc/netappsd/internal/pkg/utils"
)

var rootCmd = &cobra.Command{
	Use:   "netappsd",
	Short: "NetApp filer discovery and exporter initiator",
	Long: `
Netappsd runs in master and worker mode. The master mode is used to discover
NetApp filers from netbox and monitor the workers. The workers request a
filer from the master and start the exporter for it.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug logging")
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))

	rootCmd.AddCommand(master.Cmd)
	rootCmd.AddCommand(worker.Cmd)
}

func initConfig() {
	viper.AutomaticEnv()

	logLvl := new(slog.LevelVar)
	if viper.GetBool("debug") {
		logLvl.Set(slog.LevelDebug)
	}
	logger := utils.NewLogger(logLvl, true /*addSource*/)
	slog.SetDefault(logger)
}
