package master

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sapcc/go-bits/respondwith"
	"github.com/sapcc/netappsd/internal/netappsd"
)

type NetappsdMaster struct {
	*netappsd.NetAppSD
}

// AddTo implements the go-bits/httpapi.API interface. It registers the handler
// for the /next/filer.json endpoint, which returns the next filer to be
// worked on. It also registers the /healthz endpoint, which is used by the
// Kubernetes readiness/liveness probe.
func (n *NetappsdMaster) AddTo(r *mux.Router) {
	// next filer endpoint
	r.Methods("GET").
		Path("/next/filer").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			podname := r.URL.Query().Get("pod")
			if podname == "" {
				respondwith.JSON(w, http.StatusBadRequest, "missing pod parameter")
				return
			}
			if !n.IsValidPodName(podname) {
				respondwith.JSON(w, http.StatusBadRequest, "invalid pod name")
				return
			}
			if filer, err := n.NextFiler(ctx, podname); err != nil {
				respondwith.ErrorText(w, err)
			} else {
				respondwith.JSON(w, 200, filer)
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

func (n *NetappsdMaster) IsValidPodName(podname string) bool {
	return strings.Contains(podname, "-")
}
