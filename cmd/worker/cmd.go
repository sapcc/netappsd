package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/fatih/structs"
	"github.com/sapcc/go-bits/httpapi"
	"github.com/sapcc/go-bits/httpext"
	"github.com/sapcc/go-bits/must"
	"github.com/sapcc/netappsd/internal/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
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

type Config struct {
	NetappUsername string `env:"NETAPP_USERNAME"`
	NetappPassword string `env:"NETAPP_PASSWORD"`
	PodName        string `env:"POD_NAME"`
	PodNamespace   string `env:"POD_NAMESPACE"`
	ListenAddr     string `pflag:"listen-addr"`
}

var config = &Config{}

var Cmd = &cobra.Command{
	Use:   "worker",
	Short: "Netappsd worker: initialize filer exporter",
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		if viper.GetBool("debug") {
			logLvl.Set(slog.LevelDebug)
		}
		log.Debug("log level is set", "logLvl", logLvl)

		if err := readConfig(config); err != nil {
			return err
		}
		log.Debug("config", "config", config)
		return nil
	},
	Run:          run,
	SilenceUsage: true,
}

func init() {
	log = utils.NewLogger(logLvl)

	Cmd.Flags().StringVarP(&masterUrl, "master-url", "m", "http://localhost:8080", "The url of the netappsd-master")
	Cmd.Flags().StringVarP(&httpListenAddr, "listen-addr", "l", ":8082", "The address to listen on")
	Cmd.Flags().StringVarP(&outputFilePath, "output-file", "o", "harvest.yaml", "The path to the output file")
	Cmd.Flags().StringVarP(&templateFilePath, "template-file", "t", "harvest.yaml.tpl", "The path to the template file")

	bindFlagAndEnv(config)
}

func bindFlagAndEnv(cfg interface{}) error {
	for _, field := range structs.Fields(cfg) {
		if envconfig := field.Tag("env"); envconfig != "" {
			if err := viper.BindEnv(field.Name(), envconfig); err != nil {
				return err
			}
		}
		if pflag := field.Tag("pflag"); pflag != "" {
			if err := viper.BindPFlag(field.Name(), Cmd.Flags().Lookup(pflag)); err != nil {
				return err
			}
		}
	}
	return nil
}

func readConfig(cfg interface{}) error {
	if err := viper.Unmarshal(cfg); err != nil {
		return err
	}
	for _, field := range structs.Fields(cfg) {
		if envconfig := field.Tag("env"); envconfig != "" {
			if field.Value().(string) == "" {
				return fmt.Errorf("failed to get %s environment variable", envconfig)
			}
		}
	}
	return nil
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
	log.Info("requesting filer from master", "url", url, "timeout", timeout, "interval", interval)
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
