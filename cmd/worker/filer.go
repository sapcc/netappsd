package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/sapcc/netappsd/internal/pkg/netapp"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
	"github.com/sapcc/netappsd/internal/pkg/utils"
)

type FilerClient struct {
	Filer    *netbox.Filer
	Client   *netapp.RestClient
	probeerr error
}

func (f *FilerClient) RequestFiler(ctx context.Context, url string, requestInterval, requestTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	timeout := time.NewTimer(requestTimeout)
	t := new(utils.TickTick) // use ticktick to avoid delay on first request

	defer func() {
		cancel()
		timeout.Stop()
	}()

	for {
		select {
		case <-t.After(requestInterval):
			filer, err := f.fetch(url)
			if err != nil {
				log.Warn("failed to fetch filer", "error", err.Error())
				continue
			}
			f.Filer = filer
			return nil
		case <-timeout.C:
			return fmt.Errorf("timed out after %s", requestTimeout)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (f *FilerClient) ProbeFiler(ctx context.Context, wg *sync.WaitGroup, probeInterval time.Duration) {
	log.Debug("starting filer probe")
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-time.After(probeInterval):
			if f.Client == nil {
				f.Client = netapp.NewRestClient(f.Filer.Host, &netapp.ClientOptions{
					BasicAuthUser:     netappUsername,
					BasicAuthPassword: netappPassword,
					Timeout:           30 * time.Second,
				})
			}
			f.probeerr = nil
			if _, err := f.Client.Get("/api/storage/aggregates", nil); err != nil {
				f.probeerr = err
			}
		case <-ctx.Done():
			return
		}
	}
}

func (f *FilerClient) fetch(url string) (*netbox.Filer, error) {
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

func (f *FilerClient) Render(templatePath, outputPath string) error {
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

func (f *FilerClient) AddTo(r *mux.Router) {
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
