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
)

type NetAppSD struct {
	NetboxClient        *netbox.Client
	KubernetesClientset *kubernetes.Clientset
	AppLabel            string
	Namespace           string
	Region              string
	ServiceType         string

	mu            sync.Mutex
	filers        []netbox.Filer
	filerQueue    []*netbox.Filer
	newreplicaset string
}

func (n *NetAppSD) IsReady() bool {
	return len(n.filers) > 0
}

// Start periodically queries netbox for filers and builds a queue of filers to
// be harvested.
func (n *NetAppSD) Start(ctx context.Context) {
	// discover filers periodically with relatively long interval
	go func() {
		for {
			select {
			case <-time.After(5 * time.Minute):
				n.discover(true)
			case <-ctx.Done():
				return
			}
		}
	}()

	// make queue periodically with relatively short interval
	for {
		select {
		case <-time.After(30 * time.Second):
			n.makeQueue(true)
		case <-ctx.Done():
			return
		}
	}
}

// NextFiler returns the next working item from the queue. Internal errors
// while making the queue are not propagated. Only a generic empty queue
// error is returned.
func (n *NetAppSD) NextFiler(podName string) (*netbox.Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// remake target queue if request is from new replicaset
	r := strings.Split(podName, "-")
	replicaset := strings.Join(r[:len(r)-1], "-")
	if replicaset != n.newreplicaset {
		slog.Info("NextFiler: make Filer queue for replicaset", "replicaSet", replicaset)
		n.newreplicaset = replicaset
		n.makeQueue(false)
	}

	if len(n.filerQueue) == 0 {
		err := fmt.Errorf("NextFiler: filer queue is empty")
		return nil, err
	}

	// get first filer from n.filerQueue and remove it from the queue
	next := n.filerQueue[0]
	n.filerQueue = n.filerQueue[1:]
	if len(n.filerQueue) == 0 {
		slog.Info("NextFiler: all filers are harvested")
	} else {
		slog.Info("NextFiler: filer is harvested", "filer", next.Name)
		slog.Info(fmt.Sprintf("NextFiler: %d filers left in queue", len(n.filerQueue)))
	}
	return next, nil
}

// discover queries netbox for filers and cache the filer list. It also
// rebuilds the queue of filers to be harvested.
func (n *NetAppSD) discover(lockq bool) {
	if lockq {
		n.mu.Lock()
		defer n.mu.Unlock()
	}
	filers, err := n.NetboxClient.GetFilers(n.Region, n.ServiceType)
	if err != nil {
		slog.Error("failed to fetch filers from netbox", "error", err)
		return
	}
	n.filers = filers
	n.makeQueue(false)
	slog.Info(fmt.Sprintf("fetched %d filers from netbox", len(filers)))
}

// makeQueue builds a queue of filers to be harvested. It reads the label of
// the pods to determine which filers are already harvested and which filers
// are not harvested yet.
func (n *NetAppSD) makeQueue(lockq bool) {
	if lockq {
		n.mu.Lock()
		defer n.mu.Unlock()
	}
	if len(n.filers) == 0 {
		slog.Info("makeQueue: discover filers since no filer is cached")
		n.discover(false)
	}
	if n.newreplicaset == "" {
		slog.Warn("makeQueue canceld: replicaset not set")
		return
	}

	// Get pods and check which filers are already harvested by checking the
	// label. Only pods belongs to the new replicaset are considered.
	// Label example: app=netapp-harvest, filer=netapp-qa-de-1-01
	pods, err := n.getPods(n.AppLabel)
	if err != nil {
		slog.Error("makeQueue failed", "error", err)
		return
	}

	// Build queue of filers that are not running
	n.filerQueue = make([]*netbox.Filer, 0, len(n.filers))
	running := make(map[string]bool)
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, n.newreplicaset) {
			if filer, found := pod.Labels["filer"]; found {
				running[filer] = true
			}
		}
	}

	for i, filer := range n.filers {
		if ok := running[filer.Name]; !ok {
			n.filerQueue = append(n.filerQueue, &n.filers[i])
			slog.Debug("makeQueue: append filer to queue", "filer", filer.Name, "replicaSet", n.newreplicaset)
		}
	}

	if l := len(n.filerQueue); l > 0 {
		slog.Info("makeQueue done", "queueLength", l, "replicaSet", n.newreplicaset)
	}
}

// getPods is an example to get pods in a k8s namespace filtered by label
func (n *NetAppSD) getPods(label string) (*corev1.PodList, error) {
	return n.KubernetesClientset.CoreV1().Pods(n.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
}
