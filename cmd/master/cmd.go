package master

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/sapcc/netappsd/internal/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	log    *slog.Logger
	logLvl *slog.LevelVar = new(slog.LevelVar)
)

var Cmd = &cobra.Command{
	Use:   "master",
	Short: "Netappsd master: discover filers from netbox",
	PreRun: func(cmd *cobra.Command, _ []string) {
		if viper.GetBool("debug") {
			logLvl.Set(slog.LevelDebug)
			log.Debug("debug logging enabled")
		}
	},
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := httpext.ContextWithSIGINT(context.Background(), 0)

		log.Info("starting netappsd master", "netbox-url", viper.GetString("netbox-url"))
		netappsd := NewNetAppSD("qa-de-1", "manila", "netapp-exporters", "app=netappsd-worker")

		go netappsd.Start(ctx)

		mux := http.NewServeMux()
		mux.Handle("/", httpapi.Compose(netappsd))
		must.Succeed(httpext.ListenAndServeContext(ctx, viper.GetString("listen-addr"), mux))
	},
}

func init() {
	log = utils.NewLogger(logLvl)

	Cmd.Flags().StringP("listen-addr", "l", ":8080", "The address to listen on")
	Cmd.Flags().StringP("netbox-token", "t", "", "The token to authenticate against netbox")
	Cmd.Flags().StringP("netbox-url", "n", "netbox.staging.cloud.sap", "The url of the netbox instance")

	viper.BindPFlag("listen-addr", Cmd.Flags().Lookup("listen-addr"))
	viper.BindPFlag("netbox-token", Cmd.Flags().Lookup("netbox-token"))
	viper.BindPFlag("netbox-url", Cmd.Flags().Lookup("netbox-url"))

	viper.BindEnv("netbox-token", "NETBOX_TOKEN")
	viper.BindEnv("netbox-url", "NETBOX_URL")
}
