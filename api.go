package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
)

func handleNameRequest(w http.ResponseWriter, r *http.Request) {
	n, found := q.NextName()
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Write([]byte(n))
}

func handleYamlRequest(templateDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		tplName := vars["templateName"]
		tpl, err := template.ParseGlob(filepath.Join(templateDir, fmt.Sprintf("%s.yaml.tpl", tplName)))
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
}
