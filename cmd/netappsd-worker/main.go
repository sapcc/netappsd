package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	debug            bool
	httpListenAddr   string
	masterUrl        string
	outputFilePath   string
	templateFilePath string

	podName        string
	podNamespace   string
	netappUsername string
	netappPassword string

	log    *slog.Logger
	logLvl *slog.LevelVar = new(slog.LevelVar)
)

var cmd = &cobra.Command{
	Use:  "worker",
	Long: `A simple network application service daemon worker`,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		if debug {
			logLvl.Set(slog.LevelDebug)
			log.Debug("debug logging enabled")
		}
		netappUsername = os.Getenv("NETAPP_USERNAME")
		if netappUsername == "" {
			return fmt.Errorf("failed to get NETAPP_USERNAME environment variable")
		}
		netappPassword = os.Getenv("NETAPP_PASSWORD")
		if netappPassword == "" {
			return fmt.Errorf("failed to get NETAPP_PASSWORD environment variable")
		}
		podName = os.Getenv("POD_NAME")
		if podName == "" {
			return fmt.Errorf("failed to get POD_NAME environment variable")
		}
		podNamespace = os.Getenv("POD_NAMESPACE")
		if podNamespace == "" {
			return fmt.Errorf("failed to get POD_NAMESPACE environment variable")
		}
		return nil
	},
	Run:          run,
	SilenceUsage: true,
}

func main() {
	log = newLogger()

	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://localhost:8080", "The url of the netappsd-master")
	cmd.Flags().StringVarP(&httpListenAddr, "listen-addr", "l", ":8082", "The address to listen on")
	cmd.Flags().StringVarP(&outputFilePath, "output-file", "o", "harvest.yaml", "The path to the output file")
	cmd.Flags().StringVarP(&templateFilePath, "template-file", "t", "harvest.yaml.tpl", "The path to the template file")

	if err := cmd.Execute(); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	log.Info("Starting netappsd worker", "template-file", templateFilePath, "output-file", outputFilePath)

	ctx := httpext.ContextWithSIGINT(context.Background(), 0)
	f := new(FilerClient)
	wg := new(sync.WaitGroup)
	defer wg.Wait()

	// request filer from the master with timeout
	url := masterUrl + "/next/filer?pod=" + podName
	timeout := 5 * time.Minute
	interval := 5 * time.Second
	if err := f.RequestFiler(ctx, url, interval, timeout); err != nil {
		log.Error("failed to request filer", "error", err.Error())
		os.Exit(2)
	}
	if err := f.Render(templateFilePath, outputFilePath); err != nil {
		log.Error("failed to render filer template", "error", err.Error())
		os.Exit(2)
	}
	if err := setLabel(ctx, podNamespace, podName, "filer", f.Filer.Name); err != nil {
		log.Error("failed to set filer label", "error", err.Error())
		os.Exit(2)
	}

	go f.ProbeFiler(ctx, wg, 5*time.Minute /* probeInterval */)

	// start the health check server
	mux := http.NewServeMux()
	mux.Handle("/", httpapi.Compose(f))
	must.Succeed(httpext.ListenAndServeContext(ctx, httpListenAddr, mux))
}

func newLogger() *slog.Logger {
	logOptions := new(slog.HandlerOptions)
	logOptions.AddSource = false
	logOptions.Level = logLvl
	return slog.New(slog.NewTextHandler(os.Stderr, logOptions))
}

// setLabel sets the filer label on the pod
func setLabel(ctx context.Context, namespace, podName, key, value string) error {
	log.Info("setting label on pod", "pod", podName, "key", key, "value", value)
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	pod.Labels[key] = value
	_, err = clientset.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
