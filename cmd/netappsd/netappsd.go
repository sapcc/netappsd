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
	netboxURL := ""
	netboxToken := ""
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
