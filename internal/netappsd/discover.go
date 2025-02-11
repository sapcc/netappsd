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

	filerList        map[string]Filer
	filerQueue       []Filer
	lastProbeError   error
	lastProbeFilerTs SyncMapTimestamp

	netboxClient  *netbox.Client
	kubeClientset *kubernetes.Clientset
	mu            sync.Mutex
}

type SyncMapTimestamp struct {
	sync.Map
}

func (m *SyncMapTimestamp) LoadTime(key string) time.Time {
	v, _ := m.LoadOrStore(key, int64(0))
	return time.Unix(v.(int64), 0)
}

func (m *SyncMapTimestamp) Store(key string, value int64) {
	m.Map.Store(key, value)
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

	n.lastProbeFilerTs = SyncMapTimestamp{}
	n.filerList = make(map[string]Filer)
	n.filerQueue = make([]Filer, 0)
	discoveryDone := make(chan struct{})

	go func() {
		defer close(discoveryDone)
		tick := new(utils.TickTick)

		for {
			select {
			case <-tick.Every(5 * time.Minute):
			case <-ctx.Done():
				return
			}
			success := 0
			failed := 0
			success, failed, n.lastProbeError = n.discoverFilers(ctx)
			if n.lastProbeError != nil {
				slog.Warn("filer discovery failed", "error", n.lastProbeError)
			} else {
				slog.Info("filer discovery done", "success", success, "failed", failed)
				discoveryDone <- struct{}{}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-discoveryDone: // update worker replicas after filer discovery
			case <-time.After(30 * time.Second): // update worker replicas every 30 seconds
			case <-ctx.Done():
				return
			}
			if err := n.updateWorkerReplica(ctx); err != nil {
				slog.Error("update worker replicas failed", "error", err)
			}
		}
	}()

	return nil
}

// NextFiler returns the next filer in queue and sets the filer label on the
// worker pod. It returns error if there are no filers in the queue or if the
// filer label could not be set on the worker pod. The filer queue is updated
// only when the filer label is set successfully.
func (n *NetAppSD) NextFiler(ctx context.Context, podName string) (*Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Remove filer label; pod restart may leave stale request. Filer request implies no active work.
	if err := n.clearFilerLabelForPod(ctx, podName); err != nil {
		return nil, err
	}

	if len(n.filerQueue) == 0 {
		return nil, fmt.Errorf("no filer to work on")
	}

	// Set filer label; remove filer from queue on success.
	nextFiler := n.filerQueue[0]
	if err := n.setFilerLabelForPod(ctx, podName, nextFiler.Name); err != nil {
		return nil, err
	}
	n.filerQueue = n.filerQueue[1:]

	enqueuedFiler.Reset()
	for _, filer := range n.filerQueue {
		enqueuedFiler.WithLabelValues(filer.Name, filer.Host, filer.Ip).Set(1)
	}
	slog.Info("next filer for worker", "filer", nextFiler.Name, "pod", podName)
	return &nextFiler, nil
}

func (n *NetAppSD) setFilerLabelForPod(ctx context.Context, podName, value string) error {
	pod, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %s", err)
	}
	slog.Info("set pod label", "filer", value, "pod", podName)
	pod.Labels["filer"] = value
	_, err = n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod: %s", err)
	}
	return nil
}

func (n *NetAppSD) clearFilerLabelForPod(ctx context.Context, podName string) error {
	pod, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %s", err)
	}
	if _, found := pod.Labels["filer"]; found {
		slog.Info("delete filer label from pod", "pod", podName)
		delete(pod.Labels, "filer")
		_, err = n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update pod: %s", err)
		}
	}
	return nil
}

func (n *NetAppSD) IsReady() bool {
	return len(n.filerList) > 0
}

// discoverFilers queries netbox for filers and updates their timestamps.
func (n *NetAppSD) discoverFilers(ctx context.Context) (int, int, error) {
	// TODO: Add context to signature of the function
	filers, err := n.netboxClient.GetFilers(n.Region, n.FilerTag)
	if err != nil {
		return 0, 0, err
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// probe filer in parallel
	wg := sync.WaitGroup{}
	successCounter := atomic.Int32{}
	failedCounter := atomic.Int32{}
	discoveredFiler.Reset()

	for _, f := range filers {
		if f.Status != "active" {
			slog.Info("filer's status is not active in Netbox", "filer", f.Name, "status", f.Status)
			continue
		}

		wg.Add(1)

		go func(filer Filer) {
			ctx, fn := context.WithTimeout(ctx, 60*time.Second)
			defer fn()
			defer wg.Done()

			if err := n.probeFiler(ctx, filer); err != nil {
				failedCounter.Add(1)
				probeFilerErrors.WithLabelValues(filer.Name, filer.Host, filer.Ip).Inc()
				slog.Warn("filer probe failed", "filer", filer.Name, "error", err, "timeout", 60)
				return
			}

			successCounter.Add(1)
			discoveredFiler.WithLabelValues(filer.Name, filer.Host, filer.Ip).Set(1)

			// initialize filer list if not exists
			if _, found := n.filerList[filer.Name]; !found {
				slog.Info("new filer discovered", "filer", filer.Name)
				n.filerList[filer.Name] = filer
			}
			// update filer probing timestamp
			n.lastProbeFilerTs.Store(filer.Name, time.Now().Unix())
		}(Filer(*f))
	}

	wg.Wait()
	return int(successCounter.Load()), int(failedCounter.Load()), nil
}

func (n *NetAppSD) probeFiler(ctx context.Context, filer Filer) error {
	filerAddress := filer.Ip
	if filer.Ip == "" {
		slog.Info("filer ip is empty", "filer", filer.Name)
		filerAddress = filer.Host
	}
	c := netapp.NewFilerClient(filerAddress, n.NetAppUsername, n.NetAppPassword)
	return c.Probe(ctx)
}

// updateWorkerReplica updates the worker replicas based on the current state of the system.
// It retrieves the worker details, enqueues new filers, scales up worker replicas if the queue is not empty,
// and retires workers that are not associated with any observed filer.
// It returns an error if any of the operations fail.
func (n *NetAppSD) updateWorkerReplica(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	cntFreeWorkers, filerInWorkers, err := n.getWorkerDetails(ctx)
	if err != nil {
		return err
	}

	// update queue
	n.updateFilerQueue(filerInWorkers)

	// update filer queue metrics
	enqueuedFiler.Reset()
	for _, filer := range n.filerQueue {
		enqueuedFiler.WithLabelValues(filer.Name, filer.Host, filer.Ip).Set(1)
	}

	// increase worker replicas if more workers are needed
	if len(n.filerQueue) > cntFreeWorkers {
		slog.Info("more workers needed", "freeWorkers", cntFreeWorkers, "queue", len(n.filerQueue))
		if err := n.scaleUpWorkers(ctx, len(n.filerQueue)-cntFreeWorkers); err != nil {
			slog.Warn("scale up worker replicas failed", "error", err)
			return err
		}
	}

	// We will skip deleting inactive workers ONLY if the queue is not empty.
	// Because we can not delete specific workers, we can only scale down the deployment.
	// If we scale down the deployment while there are workers waiting in the queue, we might lose them.
	if len(n.filerQueue) > 0 {
		slog.Info("skip worker retirement", "queue", len(n.filerQueue))
		return nil
	}

	// Retire inactive workers. We set the worker pod deletion cost to -999 to mark it for deletion.
	cnt, err := n.prepareDeletingWorkers(ctx)
	if err != nil {
		return err
	}
	return n.scaleDownWorkers(ctx, cnt)
}

// getWorkerDetails returns the number of free workers and a map of filers that
// are being worked on.
func (n *NetAppSD) getWorkerDetails(ctx context.Context) (int, map[string]struct{}, error) {
	workers := make(map[string]struct{})
	pods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	})
	if err != nil {
		return 0, nil, err
	}
	for _, pod := range pods.Items {
		if filerName, found := pod.Labels["filer"]; found {
			workers[filerName] = struct{}{}
		}
	}
	freeWorkers := len(pods.Items) - len(workers)
	return freeWorkers, workers, nil
}

// updateFilerQueue appends filer queue with filers that are not being worked
// on. It skips filers that are already in the worker or in the queue.
func (n *NetAppSD) updateFilerQueue(filerInWorkers map[string]struct{}) {
	filerInQueue := make(map[string]struct{})
	for _, filer := range n.filerQueue {
		filerInQueue[filer.Name] = struct{}{}
	}
	for filerName := range n.filerList {
		if _, ok := filerInWorkers[filerName]; ok {
			continue
		}
		if _, ok := filerInQueue[filerName]; ok {
			continue
		}
		// Do not add filer to the queue if it is not probed in the last
		// 5 minutes. This is to avoid adding filers that are not
		// reachable. Filers are fetched from Netbox and probed every 5
		// minutes. And queue is updated every 30 seconds.
		lastProbeTime := n.lastProbeFilerTs.LoadTime(filerName)
		if time.Since(lastProbeTime) > 5*time.Minute {
			slog.Info("skip filer", "filer", filerName, "lastProbeTime", lastProbeTime)
			continue
		}
		n.filerQueue = append(n.filerQueue, n.filerList[filerName])
		slog.Info("enqueue filer", "filer", filerName)
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

// prepareDeletingWorkers marks the pod by setting the deletion cost to -999 and
// returns the number of pods marked. It skips the pods that are being deleted.
// The pods that are associated with filers that are not probed in the last 48
// hours are marked for deletion.
func (n *NetAppSD) prepareDeletingWorkers(ctx context.Context) (int, error) {
	workerPods, err := n.kubeClientset.CoreV1().Pods(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.WorkerLabel,
	})
	if err != nil {
		return 0, err
	}
	cnt := 0
	for _, pod := range workerPods.Items {
		// skip worker being deleted
		if pod.DeletionTimestamp != nil {
			slog.Info("skip terminating pod", "pod", pod.Name)
			continue
		}
		if filerName, hasFilerLabel := pod.Labels["filer"]; hasFilerLabel {
			lastProbeTime := n.lastProbeFilerTs.LoadTime(filerName)
			if n.lastProbeError == nil && time.Since(lastProbeTime) > 48*time.Hour {
				slog.Info("retire old worker", "filer", filerName, "pod", pod.Name, "lastProbeTime", lastProbeTime)
				if err := n.updatePodDeletionCost(ctx, pod); err != nil {
					return 0, err
				}
				cnt++
			}
		} else {
			slog.Warn("pod does not have filer label", "pod", pod.Name)
			if err := n.updatePodDeletionCost(ctx, pod); err != nil {
				return 0, err
			}
			cnt++
		}
	}
	return cnt, nil
}

func (n *NetAppSD) updatePodDeletionCost(ctx context.Context, pod v1.Pod) error {
	pod.Annotations["controller.kubernetes.io/pod-deletion-cost"] = "-999"
	if _, err := n.kubeClientset.CoreV1().Pods(n.Namespace).Update(ctx, &pod, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}
