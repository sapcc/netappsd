package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/sapcc/netappsd/internal/pkg/netapp"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
	"github.com/sapcc/netappsd/internal/pkg/utils"
	"github.com/spf13/viper"
)

type NetappsdWorker struct {
	Filer    *netbox.Filer
	Client   *netapp.RestClient
	probeerr error
}

func (f *NetappsdWorker) RequestFiler(ctx context.Context, url string, requestInterval, requestTimeout time.Duration) error {
	timeout := time.NewTimer(requestTimeout)
	t := new(utils.TickTick) // use ticktick to avoid delay on first request

	defer func() {
		timeout.Stop()
	}()

	for {
		select {
		case <-t.After(requestInterval):
			filer, err := f.fetch(url)
			if err != nil {
				slog.Warn("failed to fetch filer", "error", err.Error())
				continue
			}
			f.Filer = filer
			f.Filer.Username = viper.GetString("netapp_username")
			f.Filer.Password = viper.GetString("netapp_password")
			return nil
		case <-timeout.C:
			return fmt.Errorf("timed out after %s", requestTimeout)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (f *NetappsdWorker) ProbeFiler(ctx context.Context, wg *sync.WaitGroup, probeInterval time.Duration) {
	slog.Info("start probing filer", "filer", f.Filer.Name)
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-time.After(probeInterval):
			if f.Client == nil {
				f.Client = netapp.NewRestClient(f.Filer.Host, &netapp.ClientOptions{
					BasicAuthUser:     f.Filer.Username,
					BasicAuthPassword: f.Filer.Password,
					Timeout:           30 * time.Second,
				})
			}
			f.probeerr = nil
			if _, err := f.Client.Get("/api/storage/aggregates"); err != nil {
				slog.Warn("probe failed", "filer", f.Filer.Name, "error", f.probeerr)
				f.probeerr = err
			}
		case <-ctx.Done():
			return
		}
	}
}

func (f *NetappsdWorker) fetch(url string) (*netbox.Filer, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s", b)
	}
	filer := new(netbox.Filer)
	if err = json.NewDecoder(resp.Body).Decode(filer); err != nil {
		return nil, err
	}
	return filer, nil
}

func (f *NetappsdWorker) Render(templatePath, outputPath string) error {
	outputPath = outputPath + "/" + f.Filer.Name + ".yaml"
	fo, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer fo.Close()
	tpl, err := template.ParseGlob(templatePath)
	if err != nil {
		return err
	}
	return tpl.Execute(fo, f.Filer)
}

func (f *NetappsdWorker) AddTo(r *mux.Router) {
	r.Methods("GET").Path("/healthz").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if f.probeerr != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("FAIL - " + f.probeerr.Error()))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}
		})
}
