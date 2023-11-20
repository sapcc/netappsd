package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gorilla/mux"
	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/sapcc/netappsd/internal/netappsd"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
	"github.com/spf13/cobra"
)

var (
	httpListenAddr string
	netboxURL      string
	netboxToken    string
)

var cmdMaster = &cobra.Command{
	Use:   "master",
	Short: "Discover filers",
	Long:  `Discover filers`,
	Run: func(_ *cobra.Command, _ []string) {
		log = log.WithName("netappsd-master")
		log.Info("Starting netappsd")

		netappsd := NewNetAppSD()
		go netappsd.Discover(nil)

		handler := httpapi.Compose(netappsd)
		mux := http.NewServeMux()
		mux.Handle("/", handler)

		ctx := httpext.ContextWithSIGINT(context.Background(), 10*time.Second)
		must.Succeed(httpext.ListenAndServeContext(ctx, httpListenAddr, mux))
	},
}

func init() {
	cmdMaster.Flags().StringVarP(&httpListenAddr, "listen-addr", "l", ":8080", "The address to listen on")
	cmdMaster.Flags().StringVarP(&netboxURL, "netbox-url", "n", "https://netbox.staging.cloud.sap", "The url of the netbox instance")
	cmdMaster.Flags().StringVarP(&netboxToken, "netbox-token", "t", "", "The token to authenticate against netbox")
}

type NetAppSD struct {
	*netappsd.NetAppSD
}

func NewNetAppSD() *NetAppSD {
	netboxClient, err := netbox.NewClient(netboxURL, netboxToken)
	if err != nil {
		panic(err.Error())
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return &NetAppSD{
		&netappsd.NetAppSD{
			NetboxClient:        &netboxClient,
			KubernetesClientset: clientset,
			Region:              "qa-de-1",
			ServiceType:         "manila",
			Namespace:           "netapp-exporters",
			AppLabel:            "app=netapp-harvest",
		},
	}
}

// AddTo implements the go-bits/httpapi.API interface. It registers the handler
// to the given router.
func (n *NetAppSD) AddTo(r *mux.Router) {
	r.HandleFunc("/next/filer.json", n.handelRequest)
}

func (n *NetAppSD) handelRequest(w http.ResponseWriter, r *http.Request) {
	filer, err := n.Next()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(filer)
}
