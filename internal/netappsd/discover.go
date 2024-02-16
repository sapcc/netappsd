package netappsd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

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

	slog.Info("set pod label", "filer", next.Name, "pod", podName)
	pod, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %s", err)
	}
	pod.Labels["filer"] = next.Name
	_, err = n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update pod: %s", err)
	}

	n.queue = n.queue[1:]
	slog.Info("next filer", "filer", next.Name, "pod", podName)

	return next, nil
}

func (n *NetAppSD) IsReady() bool {
	return len(n.filers) > 0
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

	slog.Info("updating filer queue", "filers", len(n.filers), "queue_len", len(n.queue))

	queue := []*netbox.Filer{}

	pods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	})
	if err != nil {
		slog.Error("failed to get pods", "error", err)
		return
	}

	for _, filer := range n.filers {
		found := false
		for _, pod := range pods.Items {
			if pod.Labels["filer"] == filer.Name {
				found = true
				break
			}
		}
		if !found {
			slog.Info("filer queue", "filer", filer.Name)
			queue = append(queue, filer)
		}
	}

	n.queue = queue
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
