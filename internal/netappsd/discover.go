package netappsd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/core/v1"
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
			slog.Info("updating worker replicas")
			if err := n.updateWorkerReplica(ctx); err != nil {
				slog.Error("failed to update worker replicas", "error", err)
			}
		}
	}()

	return nil
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

// getFilerScore returns the score of the filer. If the filer is not found, it returns -1.
func (n *NetAppSD) getFilerScore(filerName string) int {
	if score, found := n.filerscores[filerName]; found {
		return score
	}
	return -1
}

// discover queries netbox for filers and cache the filer list.
func (n *NetAppSD) discover(ctx context.Context) (int, error) {
	// Initialize the filer map
	n.filers = make(map[string]*Filer)

	// Get filers from netbox
	filers, err := n.netboxClient.GetFilers(n.Region, n.FilerTag)
	if err != nil {
		return 0, err
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Populate the filer map; initialize filerscores for new filers
	for _, f := range filers {
		if f.Status != "active" {
			slog.Info("filer is not active", "filer", f.Name, "status", f.Status)
			continue
		}
		if _, ok := n.filerscores[f.Name]; !ok {
			slog.Info("new filer found", "filer", f.Name)
			n.filerscores[f.Name] = 2
		}

		n.filers[f.Name] = (*Filer)(f)
	}

	// Decrease score for filers not found in netbox
	for filerName := range n.filerscores {
		if _, ok := n.filers[filerName]; !ok {
			n.filerscores[filerName]--
			slog.Info("filer not found in netbox", "filer", filerName, "score", n.filerscores[filerName])
		}
	}

	wg := sync.WaitGroup{}
	count := atomic.Uint32{}
	discoveredFiler.Reset()

	for _, f := range n.filers {
		wg.Add(1)

		go func(ctx context.Context, filer *Filer) {
			defer wg.Done()

			IpOrHostname := filer.Ip
			if filer.Ip == "" {
				slog.Warn("filer ip is empty", "filer", filer.Name)
				IpOrHostname = filer.Host
			}

			c := netapp.NewFilerClient(IpOrHostname, n.NetAppUsername, n.NetAppPassword)

			if err := c.Probe(ctx); err != nil {
				n.filerscores[filer.Name]--
				probeFilerErrors.WithLabelValues(filer.Name, filer.Host, filer.Ip).Inc()
				slog.Warn("probe filer failed", "filer", filer.Name, "host", filer.Ip, "error", err)
			} else {
				n.filerscores[filer.Name] = 2
				discoveredFiler.WithLabelValues(filer.Name, filer.Host, filer.Ip).Set(1)
				count.Add(1)
			}
		}(ctx, f)
	}

	wg.Wait()
	return int(count.Load()), nil
}

// updateWorkerReplica updates the worker replicas based on the current state of the system.
// It retrieves the worker details, enqueues new filers, scales up worker replicas if the queue is not empty,
// and retires workers that are not associated with any observed filer.
// It returns an error if any of the operations fail.
func (n *NetAppSD) updateWorkerReplica(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	freeWorkers, workermap, err := n.getWorkerDetails(ctx)
	if err != nil {
		return err
	}

	n.enqueueNewFilers(workermap)

	// Scale up worker replicas if the queue is not empty.
	// We will continue to retire workers only when the queue is empty.
	if len(n.queue) > 0 {
		count := len(n.queue) - freeWorkers
		return n.scaleUpWorkers(ctx, count)
	}

	// Retire workers that are not associated with any observed filer.
	// Set the pod deletion cost to -999, then scale down the worker replicas.
	countRetiredFilers, err := n.retireWorkers(ctx)
	if err != nil {
		return err
	}
	return n.scaleDownWorkers(ctx, countRetiredFilers)
}

func (n *NetAppSD) getWorkerDetails(ctx context.Context) (int, map[string]struct{}, error) {
	freeWorkers := 0
	workermap := make(map[string]struct{})

	pods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	})
	if err != nil {
		return 0, nil, err
	}

	for _, pod := range pods.Items {
		if filerName, found := pod.Labels["filer"]; !found {
			freeWorkers++
			slog.Warn("pod is a free worker", "pod", pod.Name)
		} else {
			workermap[filerName] = struct{}{}
		}
	}

	return freeWorkers, workermap, nil
}

func (n *NetAppSD) enqueueNewFilers(workermap map[string]struct{}) {
	n.queue = make([]*Filer, 0)
	enqueuedFiler.Reset()

	for filerName, score := range n.filerscores {
		if _, found := workermap[filerName]; !found && score == 2 {
			filer := n.filers[filerName]
			n.queue = append(n.queue, filer)
			enqueuedFiler.WithLabelValues(filer.Name, filer.Host, filer.Ip).Set(1)
			slog.Info("enqueue filer", "filer", filer.Name, "host", filer.Host)
		}
	}
}

func (n *NetAppSD) scaleUpWorkers(ctx context.Context, count int) error {
	if count <= 0 {
		return nil
	}

	workerDeployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, n.WorkerName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentReplicas := *workerDeployment.Spec.Replicas
	targetReplicas := int32(int(currentReplicas) + count)
	workerDeployment.Spec.Replicas = &targetReplicas

	if _, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, workerDeployment, metav1.UpdateOptions{}); err != nil {
		return err
	}

	workerReplicas.WithLabelValues().Set(float64(targetReplicas))
	slog.Info("scale up worker deployment", "current", currentReplicas, "target", targetReplicas)
	return nil
}

func (n *NetAppSD) scaleDownWorkers(ctx context.Context, count int) error {
	if count <= 0 {
		return nil
	}

	workerDeployment, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, n.WorkerName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentReplicas := *workerDeployment.Spec.Replicas
	targetReplicas := int32(int(currentReplicas) - count)
	workerDeployment.Spec.Replicas = &targetReplicas

	if _, err = n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, workerDeployment, metav1.UpdateOptions{}); err != nil {
		return err
	}

	workerReplicas.WithLabelValues().Set(float64(targetReplicas))
	slog.Info("scale down worker replicas", "current", currentReplicas, "target", targetReplicas)
	return nil
}

func (n *NetAppSD) retireWorkers(ctx context.Context) (int, error) {
	workerPods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	})
	if err != nil {
		return 0, err
	}

	retiredWorkers := make(map[string]v1.Pod)

	for _, pod := range workerPods.Items {
		// skip worker being deleted
		if pod.DeletionTimestamp != nil {
			slog.Info("skip terminating pod", "pod", pod.Name)
			continue
		}

		// retire free workers
		filerName, found := pod.Labels["filer"]
		if !found {
			retiredWorkers[pod.Name] = pod
			continue
		}

		// retire workers associated with filers not found in netbox
		if n.getFilerScore(filerName) < 0 {
			retiredWorkers[pod.Name] = pod
			delete(n.filerscores, filerName)
		}
	}

	for _, pod := range retiredWorkers {
		pod.Annotations["controller.kubernetes.io/pod-deletion-cost"] = "-999"
		if _, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, &pod, metav1.UpdateOptions{}); err != nil {
			return 0, err
		}
		if filerName, found := pod.Labels["filer"]; !found {
			slog.Info("set pod annotation", "pod", pod.Name, "annotation", "controller.kubernetes.io/pod-deletion-cost=-999", "filer", "none")
		} else {
			slog.Info("set pod annotation", "pod", pod.Name, "annotation", "controller.kubernetes.io/pod-deletion-cost=-999", "filer", filerName)
		}
	}

	return len(retiredWorkers), nil
}
