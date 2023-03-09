package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := negroni.NewResponseWriter(w)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(lrw, r)
		// Do stuff here
		e := log.Debug().Str("uri", r.RequestURI).Str("method", r.Method).Str("remote", r.RemoteAddr)
		e = e.Int("code", lrw.Status())
		e = e.Float64("time", float64(time.Since(start)))
		e.Msg("ok")
	})
}
