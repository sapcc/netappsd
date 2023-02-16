package monitor

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

func (q *MonitorQueue) AddRoutes(r *mux.Router) {
	r.Methods("GET", "HEAD").Path("/name").HandlerFunc(q.handleRequest)
	r.Methods("GET", "HEAD").Path("/{templateName}.yaml").HandlerFunc(q.handleRenderRequest)
}

func (q *MonitorQueue) handleRequest(w http.ResponseWriter, r *http.Request) {
	n, found := q.NextName()
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Write([]byte(n))
}

func (q *MonitorQueue) handleRenderRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tplName := vars["templateName"]
	tpl, err := template.ParseGlob(filepath.Join(q.tplDir, fmt.Sprintf("%s.yaml.tpl", tplName)))
	if err != nil {
		log.Err(err).Send()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, found := q.NextItem()
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var b bytes.Buffer
	err = tpl.Execute(&b, data)
	if err != nil {
		log.Err(err).Send()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(b.Bytes())
}
