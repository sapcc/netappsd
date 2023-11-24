package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/spf13/cobra"
)

var (
	debug          bool
	log            *slog.Logger
	logLvl         *slog.LevelVar
	httpListenAddr string
	netboxURL      string
	netboxToken    string
)

var cmd = &cobra.Command{
	Use:   "netappsd",
	Short: "Discover filers",
	Long:  `Discover filers`,
	PreRun: func(cmd *cobra.Command, _ []string) {
		if !cmd.Flags().Changed("netbox-token") {
			if envVal := os.Getenv("NETBOX_TOKEN"); envVal != "" {
				netboxToken = envVal
			}
		}
		if !cmd.Flags().Changed("netbox-url") {
			if envVal := os.Getenv("NETBOX_URL"); envVal != "" {
				netboxURL = envVal
			}
		}
		if debug {
			logLvl.Set(slog.LevelDebug)
			log.Debug("debug logging enabled")
		}
	},
	Run: func(cmd *cobra.Command, _ []string) {
		log.Info("starting netappsd master", "netbox-url", netboxURL)
		ctx := httpext.ContextWithSIGINT(context.Background(), 0)

		netappsd := NewNetAppSD()
		go netappsd.Discover(ctx.Done())

		mux := http.NewServeMux()
		mux.Handle("/", httpapi.Compose(netappsd))
		must.Succeed(httpext.ListenAndServeContext(ctx, httpListenAddr, mux))
	},
}

func main() {
	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	cmd.Flags().StringVarP(&httpListenAddr, "listen-addr", "l", ":8080", "The address to listen on")
	cmd.Flags().StringVarP(&netboxToken, "netbox-token", "t", "", "The token to authenticate against netbox")
	cmd.Flags().StringVarP(&netboxURL, "netbox-url", "n", "netbox.staging.cloud.sap", "The url of the netbox instance")

	if err := cmd.Execute(); err != nil {
		fmt.Println(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	logLvl = new(slog.LevelVar)
	logh := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{AddSource: false, Level: logLvl})
	log = slog.New(logh)
}
