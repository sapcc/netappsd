package main

import (
	"bytes"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/spf13/cobra"
)

var (
	masterUrl string
)

var cmd = &cobra.Command{
	Use:   "netappsd-worker",
	Short: "A simple network application service daemon",
	Long:  `A simple network application service daemon`,
	Run: func(cmd *cobra.Command, args []string) {
		log := stdr.NewWithOptions(stdlog.New(os.Stderr, "", stdlog.LstdFlags), stdr.Options{LogCaller: stdr.All})
		log = log.WithName("netappsd-worker")
		log.Info("Starting netappsd worker")

		templateDir := ""
		tplName := "config.yaml.tpl"
		tpl, err := template.ParseGlob(filepath.Join(templateDir, fmt.Sprintf("%s.yaml.tpl", tplName)))
		if err != nil {
			panic(err.Error())
		}
		// TODO: add graceful shutdown
		startRun(log, tpl)
	},
}

func main() {

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func init() {
	// cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://netappsd-master.netapp-exporters.svc:8080", "The url of the netappsd-master")
	// cmd.Flags().DurationVarP(&sleepTime, "sleep-time", "s", 10*time.Second, "The time to sleep between requests")
	cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://localhost:8080", "The url of the netappsd-master")
}

func startRun(log logr.Logger, tpl *template.Template) {

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
