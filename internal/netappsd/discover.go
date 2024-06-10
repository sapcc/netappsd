package netappsd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
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
	queue         []*Filer
	filers        map[string]*Filer
	filerscores   map[string]int
	mu            sync.Mutex
}

// NextFiler returns the next filer to work on. It returns an error if no filer
// is available. We update the filer queue if the queue is empty. If the queue
// is still empty, we return an error.
func (n *NetAppSD) NextFiler(ctx context.Context, podName string) (*Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

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

	n.filers = make(map[string]*Filer)
	n.filerscores = make(map[string]int)
	discoverCh := make(chan struct{})

	go func() {
		tick := new(utils.TickTick)
		for {
			select {
			case <-tick.After(5 * time.Minute):
				if count, err := n.discover(ctx); err != nil {
					slog.Error("failed to discover filers", "error", err)
				} else {
					slog.Info("discovered filers", "count", count)
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
			slog.Info("update worker replicas")
			if err := n.updateWorkerReplica(ctx); err != nil {
				slog.Error("failed to update worker replicas", "error", err)
			}
		}
	}()

	return nil
}

// discover queries netbox for filers and cache the filer list.
func (n *NetAppSD) discover(ctx context.Context) (int, error) {
	if filers, err := n.netboxClient.GetFilers(n.Region, n.FilerTag); err != nil {
		return 0, err
	} else {
		// update filers map and update filerscores map
		n.filers = make(map[string]*Filer)
		for _, f := range filers {
			n.filers[f.Name] = (*Filer)(f)
			if _, ok := n.filerscores[f.Name]; !ok {
				n.filerscores[f.Name] = 2
				slog.Info("new filer discovered", "name", f.Name, "host", f.Host)
			}
		}
		for filerName := range n.filerscores {
			if _, ok := n.filers[filerName]; !ok {
				n.filerscores[filerName]--
				slog.Info("filer not found in netbox", "filer", filerName)
			}
		}
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	wg := sync.WaitGroup{}
	count := atomic.Uint32{}

	for _, f := range n.filers {
		wg.Add(1)

		go func(ctx context.Context, filer *Filer) {
			defer wg.Done()

			c := netapp.NewFilerClient(filer.Ip, n.NetAppUsername, n.NetAppPassword)

			if err := c.Probe(ctx); err != nil {
				n.filerscores[filer.Name]--
				probeFilerErrors.WithLabelValues(filer.Name, filer.Ip).Inc()
				slog.Warn("probe filer failed", "filer", filer.Name, "host", filer.Ip, "error", err)
			} else {
				n.filerscores[filer.Name] = 2
				discoveredFiler.WithLabelValues(filer.Name, filer.Ip).Inc()
				count.Add(1)
			}
		}(ctx, f)
	}

	wg.Wait()
	return int(count.Load()), nil
}

func (n *NetAppSD) updateWorkerReplica(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	freeWorkers := 0
	n.queue = make([]*Filer, 0)
	workermap := make(map[string]struct{})

	if workerPods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	}); err != nil {
		return err
	} else {
		for _, pod := range workerPods.Items {
			if filerName, found := pod.Labels["filer"]; !found {
				freeWorkers++
				slog.Warn("pod is a free worker", "pod", pod.Name)
			} else {
				workermap[filerName] = struct{}{}
			}
		}
	}

	for filerName, score := range n.filerscores {
		if score == 2 {
			if _, found := workermap[filerName]; !found {
				filer := n.filers[filerName]
				n.queue = append(n.queue, filer)
				enqueuedFiler.WithLabelValues(filer.Name, filer.Host).Set(1)
			}
		}
		if score < 0 {
			if _, found := workermap[filerName]; !found {
				delete(n.filerscores, filerName)
			}
		}
	}

	// scale up if there are not enough free workers
	if len(n.queue) > freeWorkers {
		workerDeployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, n.WorkerName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		currentReplicas := *workerDeployment.Spec.Replicas
		targetReplicas := int32(int(currentReplicas) + len(n.queue) - freeWorkers)
		workerDeployment.Spec.Replicas = &targetReplicas
		_, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, workerDeployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		workerReplicas.WithLabelValues().Set(float64(targetReplicas))
		slog.Info("scale up worker deployment", "current", currentReplicas, "target", targetReplicas)
		return nil
	}

	// return if the queue is not empty, we will scale down later to avoid free workers being deleted
	if len(n.queue) > 0 {
		return nil
	}

	if freeWorkers > 0 {
		workerDeployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, n.WorkerName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		currentReplicas := *workerDeployment.Spec.Replicas
		targetReplicas := int32(int(currentReplicas) - freeWorkers)
		workerDeployment.Spec.Replicas = &targetReplicas
		_, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, workerDeployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		workerReplicas.WithLabelValues().Set(float64(targetReplicas))
		slog.Info("scale down worker deployment", "current", currentReplicas, "target", targetReplicas)
		return nil
	}

	// retire workers that are not associated with any filer
	countRetiredFilers := 0
	if workerPods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	}); err != nil {
		return err
	} else {
		for _, pod := range workerPods.Items {
			if filerName, hasLabel := pod.Labels["filer"]; !hasLabel {
				pod.Annotations["controller.kubernetes.io/pod-deletion-cost"] = "-999"
				if _, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, &pod, metav1.UpdateOptions{}); err != nil {
					return err
				}
				countRetiredFilers++
				slog.Info("set pod annotation", "pod", pod.Name, "annotation", "controller.kubernetes.io/pod-deletion-cost=-999", "filer", "none")
			} else {
				if score, found := n.filerscores[filerName]; !found || score < 0 {
					if pod.DeletionTimestamp != nil {
						slog.Info("skip terminating pod", "pod", pod.Name)
						continue
					}
					pod.Annotations["controller.kubernetes.io/pod-deletion-cost"] = "-999"
					if _, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, &pod, metav1.UpdateOptions{}); err != nil {
						return err
					}
					countRetiredFilers++
					delete(n.filerscores, filerName)
					slog.Info("set pod annotation", "pod", pod.Name, "annotation", "controller.kubernetes.io/pod-deletion-cost=-999", "filer", filerName)
				}
			}
		}
	}

	if countRetiredFilers > 0 {
		workerDeployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, n.WorkerName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		currentReplicas := *workerDeployment.Spec.Replicas
		targetReplicas := int32(int(currentReplicas) - countRetiredFilers)
		workerDeployment.Spec.Replicas = &targetReplicas
		if _, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, workerDeployment, metav1.UpdateOptions{}); err != nil {
			return err
		}
		workerReplicas.WithLabelValues().Set(float64(targetReplicas))
		slog.Info("scale down worker replicas", "current", currentReplicas, "target", targetReplicas)
	}
	return nil
}
