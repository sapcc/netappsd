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

type Filer netbox.Device

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
	filers        []*Filer
	queue         []*Filer
	mu            sync.Mutex
}

// NextFiler returns the next filer to work on. It returns an error if no filer
// is available. We update the filer queue if the queue is empty. If the queue
// is still empty, we return an error.
func (n *NetAppSD) NextFiler(ctx context.Context, podName string) (*Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.filers) == 0 {
		return nil, fmt.Errorf("filer list is empty")
	}
	if len(n.queue) == 0 {
		return nil, fmt.Errorf("no filer to work on")
	}
	nextFiler := n.queue[0]

	if pod, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Get(ctx, podName, metav1.GetOptions{}); err != nil {
		return nil, fmt.Errorf("failed to get pod: %s", err)
	} else {
		pod.Labels["filer"] = nextFiler.Name
		if _, err = n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, pod, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("failed to update pod: %s", err)
		}
		slog.Info("set pod label", "filer", nextFiler.Name, "pod", podName)
	}

	n.queue = n.queue[1:]
	slog.Info("next filer", "filer", nextFiler.Name, "pod", podName)

	return nextFiler, nil
}

func (n *NetAppSD) IsReady() bool {
	return len(n.filers) > 0
}

// Run starts the netappsd service discovery. It runs a goroutine to discover
// the filers and update the filer queue every 5 minutes. It also sets the
// replicas of the worker deployment to the number of filers.
func (n *NetAppSD) Run(ctx context.Context) error {
	if netboxClient, err := netbox.NewClient(n.NetboxHost, n.NetboxToken); err != nil {
		return err
	} else {
		n.netboxClient = &netboxClient
	}
	if clientset, err := utils.NewKubeClient(); err != nil {
		return err
	} else {
		n.kubeClientset = clientset
	}

	go func() {
		tick := new(utils.TickTick)
		for {
			select {
			case <-tick.After(5 * time.Minute):
				if err := n.discover(ctx); err != nil {
					slog.Error("failed to discover filers", "error", err)
				}
				if err := n.updateWorkerDeployment(ctx); err != nil {
					slog.Error("failed to set worker replicas", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-time.After(30 * time.Second):
				if err := n.updateWorkerDeployment(ctx); err != nil {
					slog.Error("failed to set worker replicas", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// discover queries netbox for filers and cache the filer list.
func (n *NetAppSD) discover(ctx context.Context) error {
	// cache old filers, so that only new filer will be logged
	newFilers := make([]*Filer, 0)
	oldFilers := make(map[string]bool)
	for _, filer := range n.filers {
		oldFilers[filer.Name] = true
	}

	// query netbox for filers with the specified tag
	filers, err := n.netboxClient.GetFilers(n.Region, n.FilerTag)
	if err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("%d filers discovered; check if they are reachable", len(filers)))

	wg := sync.WaitGroup{}

	// probe the filers to check if they are reachable
	for _, f := range filers {
		wg.Add(1)

		go func(ctx context.Context, filer *Filer) {
			defer wg.Done()

			f := netapp.NewFiler(filer.Host, n.NetAppUsername, n.NetAppPassword)
			if err := f.Probe(ctx); err != nil {
				slog.Info("probe filer failed", "filer", filer.Name, "host", filer.Host, "error", err)
				probeFilerErrors.WithLabelValues(filer.Name, filer.Host).Inc()
			} else {
				newFilers = append(newFilers, filer)
				if _, found := oldFilers[filer.Name]; !found {
					slog.Info("new filer", "filer", filer.Name, "host", filer.Host)
				}
				discoveredFiler.WithLabelValues(filer.Name, filer.Host).Inc()
			}
		}(ctx, (*Filer)(f))
	}

	wg.Wait()
	n.filers = newFilers
	return nil
}

// updateWorkerDeployment annotates worker pods that are not associated with any discovered filer
// and updates the number of replicas of the worker deployment to the number of filers. It also
// updates the filer queue with the filers that are to be worked on.
func (n *NetAppSD) updateWorkerDeployment(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	filerMap := make(map[string]bool)
	for _, filer := range n.filers {
		filerMap[filer.Name] = true
	}

	workerPods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	})
	if err != nil {
		return err
	}

	// remove worker pods that are not associated with any discovered filer
	for _, pod := range workerPods.Items {
		if _, found := filerMap[pod.Labels["filer"]]; found {
			delete(filerMap, pod.Labels["filer"])
		} else {
			pod.Annotations["controller.kubernetes.io/pod-deletion-cost"] = "-999"
			if _, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, &pod, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	// update the number of replicas of the worker deployment
	targetReplicas := int32(len(n.filers))
	workerDeployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, n.WorkerName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if *workerDeployment.Spec.Replicas != targetReplicas {
		workerDeployment.Spec.Replicas = &targetReplicas
		if _, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, workerDeployment, metav1.UpdateOptions{}); err != nil {
			return err
		}
		slog.Info("set number of replicas", "target", targetReplicas, "current", *workerDeployment.Spec.Replicas)
		workerReplicas.WithLabelValues().Set(float64(targetReplicas))
	}

	// set the filer queue with the filers that are to be worked on
	queue := make([]*Filer, 0)
	for _, filer := range n.filers {
		if _, found := filerMap[filer.Name]; found {
			queue = append(queue, filer)
		}
	}
	n.queue = queue

	slog.Info("updated worker deployment and queue", "replicas", targetReplicas, "filers", len(n.filers), "queue", len(n.queue))
	return nil
}
