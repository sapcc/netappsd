package master

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/sapcc/netappsd/internal/netappsd"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "master",
	Short: "Netappsd master: discover filers from netbox",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := httpext.ContextWithSIGINT(context.Background(), 0)

		netappsdMaster := new(NetappsdMaster)
		netappsdMaster.NetAppSD = &netappsd.NetAppSD{
			NetboxHost:  viper.GetString("netbox_host"),
			NetboxToken: viper.GetString("netbox_token"),
			Namespace:   viper.GetString("namespace"),
			Region:      viper.GetString("region"),
			ServiceType: viper.GetString("tag"),
			WorkerName:  viper.GetString("worker"),
		}

		slog.Info("starting netappsd master")
		slog.Info("netappsd master config", "region", netappsdMaster.NetAppSD.Region, "tag", netappsdMaster.NetAppSD.ServiceType, "worker", netappsdMaster.NetAppSD.WorkerName)

		if err := netappsdMaster.Start(ctx); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}

		mux := http.NewServeMux()
		mux.Handle("/", httpapi.Compose(netappsdMaster))
		must.Succeed(httpext.ListenAndServeContext(ctx, viper.GetString("listen_addr"), mux))
	},
}

func init() {
	Cmd.Flags().StringP("listen-addr", "l", ":8080", "The address to listen on")
	Cmd.Flags().StringP("namespace", "n", "netapp-exporters", "The namespace to use for netappsd")
	Cmd.Flags().StringP("netbox-host", "", "netbox.staging.cloud.sap", "The netbox host to query")
	Cmd.Flags().StringP("netbox-token", "", "", "The token to authenticate against netbox")
	Cmd.Flags().StringP("region", "r", "", "The region to filter netbox devices")
	Cmd.Flags().StringP("tag", "t", "", "The tag to filter netbox devices")
	Cmd.Flags().StringP("worker", "w", "", "The deployemnt label of workers")

	viper.BindPFlag("listen_addr", Cmd.Flags().Lookup("listen-addr"))
	viper.BindPFlag("netbox_host", Cmd.Flags().Lookup("netbox-host"))
	viper.BindPFlag("netbox_token", Cmd.Flags().Lookup("netbox-token"))
	viper.BindPFlag("tag", Cmd.Flags().Lookup("tag"))
	viper.BindPFlag("region", Cmd.Flags().Lookup("region"))
	viper.BindPFlag("worker", Cmd.Flags().Lookup("worker"))

	viper.BindEnv("namespace", "POD_NAMESPACE")
}
