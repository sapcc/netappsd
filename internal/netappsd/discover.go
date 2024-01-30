package netappsd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sapcc/netappsd/internal/pkg/netbox"
	"github.com/sapcc/netappsd/internal/pkg/utils"
)

type NetAppSD struct {
	NetboxHost  string
	NetboxToken string
	Namespace   string
	Region      string
	ServiceType string
	WorkerName  string

	netboxClient  *netbox.Client
	kubeClientset *kubernetes.Clientset

	mu         sync.Mutex
	replicaset string
	filers     []*netbox.Filer
	queue      []*netbox.Filer
}

// Start initializes the NetAppSD service. It starts a goroutine to discover
// NetApp filers from netbox and update the filer queue. It also starts a
// goroutine to update the number of replicas of the worker deployment.
//
// Discovering filers from netbox is done every 5 minutes. Updating the number
// of replicas of the worker deployment is done every 30 seconds.
func (n *NetAppSD) Start(ctx context.Context) error {
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
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-time.After(30 * time.Second):
				if len(n.filers) == 0 {
					continue
				}
				if err := n.setDeploymentReplicas(int32(len(n.filers))); err != nil {
					slog.Error("failed to set deployment replicas", "error", err)
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

	queue := []*netbox.Filer{}

	for _, filer := range n.filers {
		podLabel := fmt.Sprintf("name=%s", n.WorkerName)
		podLabel += fmt.Sprintf(",filer=%s", filer.Name)
		pod, err := n.getPodInReplicaset(n.replicaset, podLabel)
		if err != nil {
			slog.Error("failed to get pod in replicaset", "error", err)
			return
		}
		if pod == nil {
			queue = append(queue, filer)
		}
	}

	n.queue = queue
}

// NextFiler returns the next filer to work on. It returns an error if no filer
// is available. We update the filer queue if the replicaset changes.
func (n *NetAppSD) NextFiler(ctx context.Context, podName string) (*netbox.Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.filers) > 0 {
		r := strings.Split(podName, "-")
		replicaset := strings.Join(r[:len(r)-1], "-")
		if replicaset != n.replicaset {
			n.updateQueue(false)
		}
		if len(n.queue) > 0 {
			return n.queue[0], nil
		}
	}
	return nil, fmt.Errorf("no filer available")
}

func (n *NetAppSD) IsReady() bool {
	return len(n.filers) > 0
}

// discover queries netbox for filers and cache the filer list.
func (n *NetAppSD) discover() {
	n.mu.Lock()
	defer n.mu.Unlock()

	filers, err := n.netboxClient.GetFilers(n.Region, n.ServiceType)
	if err != nil {
		slog.Error("failed to get filers", "error", err)
		return
	}

	// log the new filers if they are not in n.filers list yet
	oldFilers := make(map[string]bool)
	for _, filer := range n.filers {
		oldFilers[filer.Name] = true
	}
	for _, filer := range filers {
		if _, found := oldFilers[filer.Name]; !found {
			slog.Info("discovered new filer", "filer", filer.Name)
		}
	}

	slog.Info(fmt.Sprintf("discovered %d filers", len(filers)))
	n.filers = filers
}

// getPodInReplicaset gets a pod in a replicaset filtered by label
func (n *NetAppSD) getPodInReplicaset(replicaset, label string) (*corev1.Pod, error) {
	pods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, replicaset) {
			return &pod, nil
		}
	}
	return nil, nil
}
