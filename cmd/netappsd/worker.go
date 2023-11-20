package main

import (
	"bytes"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
)

var (
	masterUrl        string
	templateFilePath string = "deployments/netappsd/harvest.yaml.tpl"
	outputFilePath   string = "_output/arm64/harvest.yaml"
)

var cmdWorker = &cobra.Command{
	Use:  "worker",
	Long: `A simple network application service daemon worker`,
	Run: func(_ *cobra.Command, _ []string) {

		tpl, err := template.ParseGlob(templateFilePath)
		if err != nil {
			panic(err.Error())
		}
		// TODO: add graceful shutdown
		startWorker(log, tpl)
	},
}

func init() {
	// cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://netappsd-master.netapp-exporters.svc:8080", "The url of the netappsd-master")
	// cmd.Flags().DurationVarP(&sleepTime, "sleep-time", "s", 10*time.Second, "The time to sleep between requests")
	cmdWorker.Flags().StringVarP(&masterUrl, "master-url", "m", "http://localhost:8080", "The url of the netappsd-master")
}

func startWorker(log logr.Logger, tpl *template.Template) {
	log = log.WithName("netappsd-worker")
	log.Info("Starting netappsd worker")

	ch := make(chan struct{})

	// get next item from netappsd-master every 10 seconds
	// after getting next item, start worker
	// then check health of worker every 10 seconds
	go func() {
		var sleepTime = 1 * time.Second
		for ; true; <-time.After(sleepTime) {
			resp, err := http.Get(masterUrl + "/next/filer.json")
			if err != nil {
				log.Error(err, "Failed to get next filer")
				continue
			}
			var b bytes.Buffer
			err = tpl.Execute(&b, resp)
			if err != nil {
				log.Error(err, "Failed to execute template")
				continue
			}
			// write to file
			f, err := os.Create(outputFilePath)
			if err != nil {
				log.Error(err, "Failed to create file", "path", outputFilePath)
				continue
			}
			_, err = f.Write(b.Bytes())
			if err != nil {
				log.Error(err, "Failed to write to file", "path", outputFilePath)
				continue
			}
			f.Close()

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
