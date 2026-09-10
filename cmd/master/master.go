package master

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sapcc/go-bits/respondwith"
	"github.com/sapcc/netappsd/internal/netappsd"
)

type NetappsdMaster struct {
	*netappsd.NetAppSD
}

// AddTo implements the go-bits/httpapi.API interface. It registers the handler
// for the /filer/{name} endpoint, which returns the requested filer's details.
// It also registers the /healthz endpoint, which is used by the Kubernetes
// readiness/liveness probe.
func (n *NetappsdMaster) AddTo(r *mux.Router) {
	// filer details endpoint
	r.Methods("GET").
		Path("/filer/{name}").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := mux.Vars(r)["name"]
			if filer, found := n.GetFiler(name); found {
				respondwith.JSON(w, http.StatusOK, filer)
			} else {
				respondwith.JSON(w, http.StatusNotFound, "filer not found")
			}
		})

	// health check endpoint
	r.Methods("GET").
		Path("/healthz").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !n.IsReady() {
				respondwith.JSON(w, http.StatusServiceUnavailable, "NOT READY")
			} else {
				respondwith.JSON(w, http.StatusOK, "OK")
			}
		})
}
