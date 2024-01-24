package worker

import (
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/sapcc/netappsd/internal/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	httpListenAddr   string
	masterUrl        string
	outputFilePath   string
	templateFilePath string
)

var Cmd = &cobra.Command{
	Use:          "worker",
	Short:        "Netappsd worker: initialize filer exporter",
	Run:          run,
	SilenceUsage: true,
}

func init() {
	Cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://localhost:8080", "The url of the netappsd-master")
	Cmd.Flags().StringVarP(&httpListenAddr, "listen-addr", "l", ":8082", "The address to listen on")
	Cmd.Flags().StringVarP(&outputFilePath, "output-file", "o", "harvest.yaml", "The path to the output file")
	Cmd.Flags().StringVarP(&templateFilePath, "template-file", "t", "harvest.yaml.tpl", "The path to the template file")
}

func run(cmd *cobra.Command, args []string) {
	ctx := httpext.ContextWithSIGINT(context.Background(), 0)
	wg := new(sync.WaitGroup)
	defer wg.Wait()

	r := time.Duration(rand.Intn(30)) * time.Second

	slog.Info("Starting netappsd worker")
	slog.Info("Waiting for random seconds", "seconds", r)

	select {
	case <-ctx.Done():
		slog.Info("Context done")
		os.Exit(1)
	case <-time.After(r):
	}

	f := new(NetappsdWorker)
	podNamespace := viper.GetString("pod_namespace")
	podName := viper.GetString("pod_name")

	url := masterUrl + "/next/filer?pod=" + podName
	if err := f.RequestFiler(ctx, url, 5*time.Second /* requestInterval */, 5*time.Minute /* requestTimeout */); err != nil {
		slog.Error("failed to request filer", "error", err.Error())
		os.Exit(2)
	}
	if err := f.Render(templateFilePath, outputFilePath); err != nil {
		slog.Error("failed to render filer template", "error", err.Error())
		os.Exit(2)
	}
	if err := setPodLabel(ctx, podNamespace, podName, "filer", f.Filer.Name); err != nil {
		slog.Error("failed to set pod label", "error", err.Error(), "filer", f.Filer.Name)
		os.Exit(2)
	}

	slog.Info("set pod label", "filer", f.Filer.Name, "pod", podName)

	// probe filer and set health status to unhealthy if probe fails
	// pod will be reest by kubernetes via health check
	go f.ProbeFiler(ctx, wg, 5*time.Minute /* probeInterval */)
	mux := http.NewServeMux()
	mux.Handle("/", httpapi.Compose(f))
	must.Succeed(httpext.ListenAndServeContext(ctx, httpListenAddr, mux))
}

func setPodLabel(ctx context.Context, namespace, podName, labelKey, labelValue string) error {
	kubeclientset, err := utils.NewKubeClient()
	if err != nil {
		return err
	}
	pod, err := kubeclientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	pod.Labels[labelKey] = labelValue
	_, err = kubeclientset.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
	return err
}
