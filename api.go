package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
)

type Pod struct {
	Hash string `json:"hash"`
}

func handleNameRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "expect POST", http.StatusMethodNotAllowed)
		return
	}
	var p Pod
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	n, found := q.NextName(p.Hash)
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Write([]byte(n))
}

func handleYamlRequest(templateDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expect POST", http.StatusMethodNotAllowed)
			return
		}
		var p Pod
		err := json.NewDecoder(r.Body).Decode(&p)
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		vars := mux.Vars(r)
		tplName := vars["templateName"]
		tpl, err := template.ParseGlob(filepath.Join(templateDir, fmt.Sprintf("%s.yaml.tpl", tplName)))
		if err != nil {
			log.Err(err).Send()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data, found := q.NextItem(p.Hash)
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
