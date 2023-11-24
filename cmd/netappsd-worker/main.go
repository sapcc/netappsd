package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/sapcc/netappsd/internal/pkg/netbox"
	"github.com/spf13/cobra"
)

var (
	debug            bool
	masterUrl        string
	outputFilePath   string
	podName          string
	templateFilePath string

	log    *slog.Logger
	logLvl *slog.LevelVar = new(slog.LevelVar)
)

var cmd = &cobra.Command{
	Use:  "worker",
	Long: `A simple network application service daemon worker`,
	PreRun: func(cmd *cobra.Command, _ []string) {
		if debug {
			logLvl.Set(slog.LevelDebug)
			log.Debug("debug logging enabled")
		}
	},
	Run: func(_ *cobra.Command, _ []string) {
		log.Info("Starting netappsd worker", "master-url", masterUrl, "template-file", templateFilePath, "output-file", outputFilePath)

		podName = os.Getenv("POD_NAME")
		if podName == "" {
			log.Error("Failed to get POD_NAME environment variable")
			os.Exit(1)
		}
		tpl, err := template.ParseGlob(templateFilePath)
		if err != nil {
			log.Error("Failed to parse template", "error", err)
			os.Exit(1)
		}
		err = fetchNextFiler(tpl)
		if err != nil {
			log.Error("Failed to fetch next filer", "error", err)
			os.Exit(1)
		}

		// TODO: add graceful shutdown
		// TODO: implement healthz endpoint
		for {
			<-time.After(60 * time.Second)
			log.Info("Worker is running")
		}
	},
}

func main() {
	if err := cmd.Execute(); err != nil {
		log.Error("Failed to execute command", "error", err)
		os.Exit(1)
	}
}

func init() {
	loghandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{AddSource: false, Level: logLvl})
	log = slog.New(loghandler)

	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://localhost:8080", "The url of the netappsd-master")
	cmd.Flags().StringVarP(&outputFilePath, "output-file", "o", "_output/harvest.yaml", "The path to the output file")
	cmd.Flags().StringVarP(&templateFilePath, "template-file", "t", "deployments/templates/harvest.yaml.tpl", "The path to the template file")
	// cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://netappsd-master.netapp-exporters.svc:8080", "The url of the netappsd-master")
	// cmd.Flags().DurationVarP(&sleepTime, "sleep-time", "s", 10*time.Second, "The time to sleep between requests")
}

func fetchNextFiler(tpl *template.Template) error {
	var filer netbox.Filer

	for ; true; <-time.After(5 * time.Second) {
		// fetch a filer to work on from master
		// if request fails, log error and retry after sleep
		resp, err := http.Get(masterUrl + fmt.Sprintf("/%s/next/filer.json", podName))
		if err != nil {
			log.Warn("Failed to get next filer", "reason", err)
			continue
		}
		if resp.StatusCode != 200 {
			reason, _ := parseRespBody(resp)
			log.Warn("Failed to get next filer", "reason", reason)
			continue
		}

		// decode response body into filer struct and render template
		// break out of loop if successful
		err = json.NewDecoder(resp.Body).Decode(&filer)
		if err != nil {
			return err
		} else {
			log.Debug("Got next filer", "filer", filer.Name)
		}
		err = renderTemplateTo(outputFilePath, tpl, filer)
		if err != nil {
			return err
		}

		break
	}

	return nil
}

func parseRespBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func renderTemplateTo(outputFilePath string, tpl *template.Template, filer netbox.Filer) error {
	f, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return tpl.Execute(f, filer)
}
