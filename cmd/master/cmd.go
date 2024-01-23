package master

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "master",
	Short: "Netappsd master: discover filers from netbox",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := httpext.ContextWithSIGINT(context.Background(), 0)
		netappsd := NewNetAppSD("qa-de-1", "manila", "netapp-exporters")

		slog.Info("starting netappsd master")

		if err := netappsd.Start(ctx); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}

		mux := http.NewServeMux()
		mux.Handle("/", httpapi.Compose(netappsd))
		must.Succeed(httpext.ListenAndServeContext(ctx, viper.GetString("listen_addr"), mux))
	},
}

func init() {
	Cmd.Flags().StringP("listen-addr", "l", ":8080", "The address to listen on")
	Cmd.Flags().StringP("netbox-token", "t", "", "The token to authenticate against netbox")
	Cmd.Flags().StringP("netbox-host", "n", "netbox.staging.cloud.sap", "The netbox host to query")

	viper.BindPFlag("listen_addr", Cmd.Flags().Lookup("listen-addr"))
	viper.BindPFlag("netbox_host", Cmd.Flags().Lookup("netbox-host"))
	viper.BindPFlag("netbox_token", Cmd.Flags().Lookup("netbox-token"))
}
