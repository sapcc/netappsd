package netappsd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sapcc/netappsd/internal/pkg/netapp"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
	"github.com/sapcc/netappsd/internal/pkg/utils"
)

type NetAppSD struct {
	NetboxHost     string
	NetboxToken    string
	Namespace      string
	Region         string
	ServiceType    string
	WorkerName     string
	WorkerLabel    string
	NetAppUsername string
	NetAppPassword string

	netboxClient  *netbox.Client
	kubeClientset *kubernetes.Clientset

	mu         sync.Mutex
	replicaset string
	filers     []*netbox.Filer
	queue      []*netbox.Filer
}

// Run starts the netappsd service discovery. It runs a goroutine to discover
// the filers and update the filer queue every 5 minutes. It also sets the
// replicas of the worker deployment to the number of filers.
func (n *NetAppSD) Run(ctx context.Context) error {
	netboxClient, err := netbox.NewClient(n.NetboxHost, n.NetboxToken)
	if err != nil {
		return err
	}
	n.netboxClient = &netboxClient

	clientset, err := utils.NewKubeClient()
	if err != nil {
		return err
	}
	n.kubeClientset = clientset

	go func() {
		tick := new(utils.TickTick)
		for {
			select {
			case <-tick.After(5 * time.Minute):
				n.discover()
				n.updateQueue(true)
				if len(n.filers) > 0 {
					if err := n.setDeploymentReplicas(int32(len(n.filers))); err != nil {
						slog.Error("failed to set deployment replicas", "error", err)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// setDeploymentReplicas sets the number of replicas of the worker deployment.
func (n *NetAppSD) setDeploymentReplicas(targetReplicas int32) error {
	deploymentName := n.WorkerName
	deployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if *deployment.Spec.Replicas != targetReplicas {
		slog.Info("set number of replicas", "target", targetReplicas, "current", *deployment.Spec.Replicas)
		deployment.Spec.Replicas = &targetReplicas
		_, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		return err
	}
	return nil
}

// updateQueue updates the filer queue. It removes filers that are already
// assigned to a worker pod. If error occurs, it returns without updating the
// queue. It receives lockq as an argument to determine whether to lock the
// filer queue, so that it can be called from a method that already locks the
// queue.
func (n *NetAppSD) updateQueue(lockq bool) {
	if lockq {
		n.mu.Lock()
		defer n.mu.Unlock()
	}

	slog.Info("updating filer queue")

	queue := []*netbox.Filer{}

	for _, filer := range n.filers {
		podLabel := n.WorkerLabel + ",filer=" + filer.Name
		pod, err := n.getPodWithLabel(podLabel)
		if err != nil {
			slog.Error("failed to get pod with label", "error", err)
			return
		}
		if pod == nil {
			queue = append(queue, filer)
		}
	}

	for _, q := range queue {
		slog.Info("filer queue", "filer", q.Name)
	}

	n.queue = queue
}

// NextFiler returns the next filer to work on. It returns an error if no filer
// is available. We update the filer queue if the queue is empty. If the queue
// is still empty, we return an error.
func (n *NetAppSD) NextFiler(ctx context.Context, podName string) (*netbox.Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.filers) == 0 {
		return nil, fmt.Errorf("filer list is empty")
	}
	if len(n.queue) == 0 {
		n.updateQueue(false)
	}
	if len(n.queue) == 0 {
		return nil, fmt.Errorf("no filer to work on")
	}
	next := n.queue[0]
	n.queue = n.queue[1:]
	slog.Info("next filer", "filer", next.Name, "pod", podName)
	return next, nil
}

func (n *NetAppSD) IsReady() bool {
	return len(n.filers) > 0
}

// discover queries netbox for filers and cache the filer list.
func (n *NetAppSD) discover() {
	// fetch filers from netbox
	filers, err := n.netboxClient.GetFilers(n.Region, n.ServiceType)
	if err != nil {
		slog.Error("failed to get filers", "error", err)
		return
	}

	// probe the filers to check if they are reachable
	healthyFilers := make([]*netbox.Filer, 0)
	for _, filer := range filers {
		client := netapp.NewRestClient(filer.Host, &netapp.ClientOptions{
			BasicAuthUser:     n.NetAppUsername,
			BasicAuthPassword: n.NetAppPassword,
			Timeout:           30 * time.Second,
		})
		if _, err := client.Get("/api/storage/aggregates"); err != nil {
			slog.Warn("failed to probe filer", "filer", filer.Name, "error", err)
			continue
		}
		healthyFilers = append(healthyFilers, filer)
	}

	// log the filer if it is new to the filer list
	oldFilers := make(map[string]bool)
	for _, filer := range n.filers {
		oldFilers[filer.Name] = true
	}
	for _, filer := range healthyFilers {
		if _, found := oldFilers[filer.Name]; !found {
			slog.Info("discovered new filer", "filer", filer.Name)
		}
	}

	slog.Info(fmt.Sprintf("discovered %d filers", len(healthyFilers)))
	n.filers = healthyFilers
}

// getPodWithLabel gets a pod in a replicaset filtered by label
func (n *NetAppSD) getPodWithLabel(label string) (*corev1.Pod, error) {
	pods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		// check if pod is running
		if pod.Status.Phase == corev1.PodRunning {
			return &pod, nil
		}
	}

	return nil, nil
}
