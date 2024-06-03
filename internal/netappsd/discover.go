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
	n.queue = n.queue[1:]

	slog.Info("next filer", "filer", nextFiler.Name, "pod", podName)

	if pod, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Get(ctx, podName, metav1.GetOptions{}); err != nil {
		return nil, fmt.Errorf("failed to get pod: %s", err)
	} else {
		pod.Labels["filer"] = nextFiler.Name
		if _, err = n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, pod, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("failed to update pod: %s", err)
		}
		slog.Info("set pod label", "filer", nextFiler.Name, "pod", podName)
	}

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

	discoverCh := make(chan struct{})

	go func() {
		tick := new(utils.TickTick)
		for {
			select {
			case <-tick.After(5 * time.Minute):
				if err := n.discover(ctx); err != nil {
					slog.Error("failed to discover filers", "error", err)
				} else {
					discoverCh <- struct{}{}
				}
			case <-ctx.Done():
				close(discoverCh)
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-discoverCh:
			case <-time.After(30 * time.Second):
			case <-ctx.Done():
				return
			}
			if err := n.updateWorkerReplica(ctx); err != nil {
				slog.Error("failed to update worker replicas", "error", err)
			}
		}
	}()

	return nil
}

// discover queries netbox for filers and cache the filer list.
func (n *NetAppSD) discover(ctx context.Context) error {
	newFilersMu := sync.Mutex{}
	newFilers := make([]*Filer, 0)

	// cache old filers, so that only new filer will be logged
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

	for _, f := range filers {
		wg.Add(1)

		go func(ctx context.Context, filer *Filer) {
			defer wg.Done()

			f := netapp.NewFiler(filer.Host, n.NetAppUsername, n.NetAppPassword)
			if err := f.Probe(ctx); err != nil {
				slog.Warn("probe filer failed", "filer", filer.Name, "host", filer.Host, "error", err)
				probeFilerErrors.WithLabelValues(filer.Name, filer.Host).Inc()
			} else {
				newFilersMu.Lock()
				newFilers = append(newFilers, filer)
				newFilersMu.Unlock()
				if _, found := oldFilers[filer.Name]; !found {
					slog.Info("filer discovered", "filer", filer.Name, "host", filer.Host)
				}
				discoveredFiler.WithLabelValues(filer.Name, filer.Host).Inc()
			}
		}(ctx, (*Filer)(f))
	}

	wg.Wait()

	n.mu.Lock()
	n.filers = newFilers
	n.mu.Unlock()

	return nil
}

func (n *NetAppSD) updateWorkerReplica(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	freeWorkers := 0
	filermap := make(map[string]int)
	workermap := make(map[string]int)
	queue := make([]*Filer, 0)

	for _, filer := range n.filers {
		filermap[filer.Name] = 1
	}

	workerDeployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, n.WorkerName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentReplicas := *workerDeployment.Spec.Replicas

	workerPods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	})
	if err != nil {
		return err
	}

	for _, pod := range workerPods.Items {
		if filerName, found := pod.Labels["filer"]; found {
			workermap[filerName] = 1
		} else {
			freeWorkers++
			slog.Warn("pod is a free worker", "pod", pod.Name)
		}
	}

	for _, filer := range n.filers {
		if _, found := workermap[filer.Name]; !found {
			queue = append(queue, filer)
			enqueuedFiler.WithLabelValues(filer.Name, filer.Host).Set(1)
		}
	}

	n.queue = queue

	// scale up if there are not enough free workers
	if len(queue) > freeWorkers {
		targetReplicas := int32(int(currentReplicas) + len(queue) - freeWorkers)
		workerDeployment.Spec.Replicas = &targetReplicas
		_, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, workerDeployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		workerReplicas.WithLabelValues().Set(float64(targetReplicas))
		slog.Info("scale up worker deployment", "current", currentReplicas, "target", targetReplicas)
	}

	// return if the queue is not empty, we will scale down later to avoid free workers being deleted
	if len(queue) > 0 {
		return nil
	}

	// retire workers that are not associated with any filer
	retiredFilers := 0

	for _, pod := range workerPods.Items {
		if filerName, labelFound := pod.Labels["filer"]; labelFound {
			if _, filerFound := filermap[filerName]; !filerFound {
				pod.Annotations["controller.kubernetes.io/pod-deletion-cost"] = "-999"
				if _, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, &pod, metav1.UpdateOptions{}); err != nil {
					return err
				}
				retiredFilers++
				slog.Info("set pod annotation", "pod", pod.Name, "annotation", "controller.kubernetes.io/pod-deletion-cost")
			}
		}
	}

	if retiredFilers > 0 {
		targetReplicas := int32(int(currentReplicas) - retiredFilers)
		workerDeployment.Spec.Replicas = &targetReplicas
		if _, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, workerDeployment, metav1.UpdateOptions{}); err != nil {
			return err
		}
		workerReplicas.WithLabelValues().Set(float64(targetReplicas))
		slog.Info("scale down worker replicas", "current", currentReplicas, "target", targetReplicas)
	}
	return nil
}
