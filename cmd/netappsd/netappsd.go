package main

import (
	"net/http"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gorilla/mux"
	"github.com/sapcc/go-bits/respondwith"
	"github.com/sapcc/netappsd/internal/netappsd"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
)

type NetAppSD struct {
	*netappsd.NetAppSD
}

func NewNetAppSD(region, service, namespace, appLabel string) *NetAppSD {
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
			Region:              region,
			ServiceType:         service,
			Namespace:           namespace,
			AppLabel:            appLabel,
		},
	}
}

// AddTo implements the go-bits/httpapi.API interface. It registers the handler
// for the /next/filer.json endpoint, which returns the next filer to be
// harvested. It also registers the /healthz endpoint, which is used by the
// Kubernetes readiness/liveness probe.
func (n *NetAppSD) AddTo(r *mux.Router) {
	r.Methods("GET").
		Path("/next/filer").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			podname := r.URL.Query().Get("pod")
			if podname == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("missing pod parameter"))
				return
			}
			if !n.IsValidPodName(podname) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("invalid pod name"))
				return
			}
			filer, err := n.NextFiler(podname)
			if err != nil {
				respondwith.ErrorText(w, err)
				return
			}
			respondwith.JSON(w, 200, filer)
		})
	r.Methods("GET").
		Path("/healthz").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if n.IsReady() {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("NOT READY"))
			}
		})
}

func (n *NetAppSD) IsValidPodName(podname string) bool {
	return strings.Contains(podname, "-")
}
