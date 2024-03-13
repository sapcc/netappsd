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
	FilerTag       string
	WorkerName     string
	WorkerLabel    string
	NetAppUsername string
	NetAppPassword string

	netboxClient  *netbox.Client
	kubeClientset *kubernetes.Clientset
	filers        []*netbox.Device
	queue         []*netbox.Device
	mu            sync.Mutex
}

// NextFiler returns the next filer to work on. It returns an error if no filer
// is available. We update the filer queue if the queue is empty. If the queue
// is still empty, we return an error.
func (n *NetAppSD) NextFiler(ctx context.Context, podName string) (*netbox.Device, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.filers) == 0 {
		return nil, fmt.Errorf("filer list is empty")
	}
	if len(n.queue) == 0 {
		n.updateQueue(ctx, false)
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
				n.discover(ctx)
				n.updateQueue(ctx, true)
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				if err := n.setDeploymentReplicas(ctx); err != nil {
					slog.Warn("failed to set deployment replicas", "error", err)
				}
			}
		}
	}()

	return nil
}

// discover queries netbox for filers and cache the filer list.
func (n *NetAppSD) discover(ctx context.Context) {
	// cache old filers, so that only new filer will be logged
	newFilers := make([]*netbox.Device, 0)
	oldFilers := make(map[string]bool)
	for _, filer := range n.filers {
		oldFilers[filer.Name] = true
	}

	// query netbox for filers with the specified tag
	filers, err := n.netboxClient.GetFilers(n.Region, n.FilerTag)
	if err != nil {
		slog.Error("discover failed", "error", err)
		return
	}

	wg := sync.WaitGroup{}

	// probe the filers to check if they are reachable
	for _, f := range filers {
		wg.Add(1)

		go func(filer *netbox.Device) {
			_ctx, _cancel := context.WithTimeout(ctx, 10*time.Second)
			defer func() {
				_cancel()
				wg.Done()
			}()

			discoveredFilers.Reset()

			f := netapp.NewFiler(filer.Host, n.NetAppUsername, n.NetAppPassword)
			if err := f.Probe(_ctx); err != nil {
				slog.Warn("failed to probe filer", "filer", filer.Name, "host", filer.Host, "error", err)
			} else {
				newFilers = append(newFilers, filer)
				discoveredFilers.WithLabelValues(filer.Name, filer.Host).Set(1)
				if _, found := oldFilers[filer.Name]; !found {
					slog.Info("discovered new filer", "filer", filer.Name, "host", filer.Host)
				}
			}
		}(f)
	}

	wg.Wait()

	slog.Info(fmt.Sprintf("discovered %d filers", len(newFilers)))
	n.filers = newFilers
}

// updateQueue updates the filer queue. It removes filers that are already
// assigned to a worker pod. If error occurs, it returns without updating the
// queue. It receives lockq as an argument to determine whether to lock the
// filer queue, so that it can be called from a method that already locks the
// queue.
func (n *NetAppSD) updateQueue(ctx context.Context, lockq bool) {
	if lockq {
		n.mu.Lock()
		defer n.mu.Unlock()
	}

	slog.Info("updating filer queue", "filers", len(n.filers), "queue_len", len(n.queue))

	queue := []*netbox.Device{}

	pods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
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
			slog.Debug("filer queue", "filer", filer.Name)
			queue = append(queue, filer)
		}
	}

	n.queue = queue
}

// setDeploymentReplicas sets the number of replicas of the worker deployment.
func (n *NetAppSD) setDeploymentReplicas(ctx context.Context) error {
	deploymentName := n.WorkerName
	deployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentReplicas := *deployment.Spec.Replicas
	targetReplicas := int32(len(n.filers))

	// no need to scale up or down
	if currentReplicas == targetReplicas {
		return nil
	}

	// make sure to delete the correct pods before scaling down
	// the worker for discovered filers should be running
	// the worker for undiscovered filers should be not ready
	// if currentReplicas > targetReplicas {
	// 	filerMap := make(map[string]bool)
	// 	for _, filer := range n.filers {
	// 		filerMap[filer.Name] = true
	// 	}
	//
	// 	pods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
	// 		LabelSelector: n.WorkerLabel,
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}
	// 	for _, pod := range pods.Items {
	// 		if _, found := filerMap[pod.Labels["filer"]]; found {
	// 			// found pod should be running
	// 			if pod.Status.Phase != "Running" {
	// 				return nil
	// 			}
	// 		} else {
	// 			// unfound pod should be not ready
	// 			if pod.Status.Phase == "Running" {
	// 				return nil
	// 			}
	// 		}
	// 	}
	// }

	// do not scale down pods
	if currentReplicas > targetReplicas {
		return fmt.Errorf("current replicas is greater than target replicas")
	}

	slog.Info("set number of replicas", "target", targetReplicas, "current", *deployment.Spec.Replicas)
	deployment.Spec.Replicas = &targetReplicas
	_, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

// ProbeFiler checks if the filer is reachable. It returns an error if the filer
// func (f *NetappsdWorker) ProbeFiler(ctx context.Context) error {
// 	if f.Client == nil {
// 		f.Client = netapp.NewRestClient(f.Filer.Host, &netapp.ClientOptions{
// 			BasicAuthUser:     f.Filer.Username,
// 			BasicAuthPassword: f.Filer.Password,
// 		})
// 	}
// 	_, err := f.Client.Get(ctx, "/api/storage/aggregates")
// 	if err != nil {
// 		slog.Warn("probe failed", "filer", f.Filer.Name, "error", err)
// 	}
// 	return err
// }
//
