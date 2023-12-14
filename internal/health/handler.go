package health

import "net/http"

type Handler struct {
	http.ServeMux
	liveness  bool
	readiness bool
}

func newHandler() *Handler {
	h := &Handler{}
	h.Handle("/live", http.HandlerFunc(h.LiveEndpoint))
	h.Handle("/ready", http.HandlerFunc(h.ReadyEndpoint))
	return h
}

func (h *Handler) LiveEndpoint(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, h.liveness)
}

func (h *Handler) ReadyEndpoint(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, h.readiness)
}

func (h *Handler) SetLiveness(live bool) {
	h.liveness = live
}

func (h *Handler) SetReadiness(ready bool) {
	h.readiness = ready
}

func (s *Handler) handle(w http.ResponseWriter, r *http.Request, check bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := http.StatusOK
	if !check {
		status = http.StatusInternalServerError
	}

	// write out the response code and content type header
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte("{}\n"))
}
