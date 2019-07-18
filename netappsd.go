package netappsd

import (
	"os"
	"path/filepath"

	"github.com/go-kit/kit/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func NewConfigMapOutofCluster(cmName, namespace string, logger log.Logger) (cw *ConfigMap, err error) {
	home := os.Getenv("HOME")
	kubeconfig := filepath.Join(home, ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return cw, err
	}

	return &ConfigMap{
		client:    clientset,
		configMap: cmName,
		logger:    logger,
		ns:        namespace,
	}, err
}

func run() string {
	return "Hello"
}
