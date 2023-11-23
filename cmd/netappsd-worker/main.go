package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	log              *slog.Logger
	logLvl           *slog.LevelVar
	podName          string
	masterUrl        string
	templateFilePath string
	outputFilePath   string
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
	RunE: func(_ *cobra.Command, _ []string) error {
		log.Info("Starting netappsd worker", "master-url", masterUrl, "template-file", templateFilePath, "output-file", outputFilePath)

		podName = os.Getenv("POD_NAME")
		if podName == "" {
			return fmt.Errorf("Failed to get POD_NAME environment variable")
		}
		tpl, err := template.ParseGlob(templateFilePath)
		if err != nil {
			return err
		}

		// TODO: add graceful shutdown
		startWorker(tpl)
		return nil
	},
}

func main() {
	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://localhost:8080", "The url of the netappsd-master")
	cmd.Flags().StringVarP(&outputFilePath, "output-file", "o", "_output/arm64/harvest.yaml", "The path to the output file")
	cmd.Flags().StringVarP(&templateFilePath, "template-file", "t", "deployments/harvest.yaml.tpl", "The path to the template file")
	// cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://netappsd-master.netapp-exporters.svc:8080", "The url of the netappsd-master")
	// cmd.Flags().DurationVarP(&sleepTime, "sleep-time", "s", 10*time.Second, "The time to sleep between requests")

	if err := cmd.Execute(); err != nil {
		fmt.Println(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	logLvl = new(slog.LevelVar)
	log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{AddSource: false, Level: logLvl}))
}

func startWorker(tpl *template.Template) {
	ch := make(chan struct{})

	go func() {
		for ; true; <-time.After(1 * time.Second) {
			resp, err := http.Get(masterUrl + fmt.Sprintf("/%s/next/filer.json", podName))
			if err != nil {
				log.Error("Failed to get next filer", "error", err)
				continue
			}
			if resp.StatusCode != 200 {
				log.Error("Failed to get next filer", "status", resp.StatusCode)
				continue
			}

			// print response body
			respBody := &bytes.Buffer{}
			_, err = respBody.ReadFrom(resp.Body)
			if err != nil {
				log.Error("Failed to read response body", "error", err)
				continue
			}
			log.Debug("Response body", "body", respBody.String())

			var b bytes.Buffer
			var filer netbox.Filer
			err = json.NewDecoder(respBody).Decode(&filer)
			if err != nil {
				log.Error("Failed to decode filer", "error", err)
				continue
			}
			err = tpl.Execute(&b, filer)
			if err != nil {
				log.Error("Failed to execute template", "error", err)
				continue
			}

			// write to file
			f, err := os.Create(outputFilePath)
			if err != nil {
				log.Error("Failed to create file", "path", outputFilePath, "error", err)
				continue
			}
			_, err = f.Write(b.Bytes())
			if err != nil {
				log.Error("Failed to write to file", "path", outputFilePath, "error", err)
				continue
			}
			f.Close()
			return

			// body, err := ioutil.ReadAll(resp.Body)
			// if err != nil {
			// 	fmt.Println(err)
			// }
			// // close response body
			// resp.Body.Close()
			//
			// // print response body
			// fmt.Println(string(body))
			// // if resp != nil {
			// // 	log.Println(resp.StatusCode)
			// // 	log.Println(resp.Request.GetBody())
			// // 	ch <- struct{}{}
			// // 	// return
			// // }
		}
	}()

	<-ch
	for {
		log.Info("Worker is running")
		time.Sleep(10 * time.Second)
	}
}
