package worker

import (
	"context"
	"log/slog"
	"net/http"
	"os"
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
	slog.Info("Starting netappsd worker")
	f := new(NetappsdWorker)

	ctx := httpext.ContextWithSIGINT(context.Background(), 0)
	requestURL := masterUrl + "/next/filer?pod=" + viper.GetString("pod_name")
	ticker := new(utils.TickTick)

REQUESTFILER:
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Every(10 * time.Second):
			if err := f.RequestFiler(requestURL); err != nil {
				slog.Warn("failed to request filer", "error", err.Error())
			} else {
				slog.Info("filer requested", "filer", f.Name, "host", f.Host)
				break REQUESTFILER
			}
		}
	}

	if err := f.Render(templateFilePath, outputFilePath); err != nil {
		slog.Error("failed to render filer template", "error", err.Error())
		os.Exit(1)
	}

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
