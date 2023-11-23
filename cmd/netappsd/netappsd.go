package main

import (
	"encoding/json"
	"net/http"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gorilla/mux"
	"github.com/sapcc/netappsd/internal/netappsd"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
)

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
// for the /next/filer.json endpoint, which returns the next filer to be
// harvested.
func (n *NetAppSD) AddTo(r *mux.Router) {
	r.Methods("GET").Path("/{podname}/next/filer.json").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			podname := mux.Vars(r)["podname"]
			filer, err := n.Next(podname)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(filer)
		})
}
