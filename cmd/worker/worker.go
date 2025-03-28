package worker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/sapcc/netappsd/internal/pkg/netapp"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
	"github.com/spf13/viper"
)

type NetappsdWorker struct {
	FilerClient *netapp.FilerClient
	netbox.Filer
}

func (f *NetappsdWorker) RequestFiler(url string) error {
	if err := f.fetch(url); err != nil {
		return err
	}
	username := viper.GetString("netapp_username")
	password := viper.GetString("netapp_password")
	f.FilerClient = netapp.NewFilerClient(f.Host, username, password)
	return nil
}

func (f *NetappsdWorker) fetch(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", b)
	}
	if err = json.NewDecoder(resp.Body).Decode(f); err != nil {
		return err
	}
	return nil
}

func (f *NetappsdWorker) Render(templateFilePath, outputFilePath string) error {
	fo, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer fo.Close()
	tpl, err := template.ParseGlob(templateFilePath)
	if err != nil {
		return err
	}
	return tpl.Execute(fo, f)
}

func (f *NetappsdWorker) AddTo(r *mux.Router) {
	r.Methods("GET").Path("/healthz").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if f.FilerClient == nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("no filer"))
			} else if err := f.FilerClient.Probe(ctx); err != nil {
				err = fmt.Errorf("failed to probe filer: %s", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(err.Error()))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}
		})
}
