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
	Ready               bool
	filers              []netbox.Filer
	mu                  sync.Mutex
	newreplicaset       string
	queue               chan netbox.Filer
}

// Discover is a loop that periodically queries netbox for filers and builds a
// queue of filers to be harvested.
func (n *NetAppSD) Discover(cancel <-chan struct{}) {
	for ; true; <-time.After(5 * time.Minute) {
		select {
		case <-cancel:
			return
		case <-time.After(5 * time.Minute):
			n.discover(true)
		case <-time.After(5 * time.Second):
			n.buildQueue(true)
		}
	}
}

// NextItem returns the next item from the queue
func (n *NetAppSD) Next(podName string) (netbox.Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.Ready {
		go func() {
			n.discover(true)
			n.Ready = true
		}()
		err := fmt.Errorf("netappsd is not ready yet")
		return netbox.Filer{}, err
	}

	// rebuild target queue if request is from new replicaset
	r := strings.Split(podName, "-")
	replicaset := strings.Join(r[:len(r)-1], "-")
	if replicaset != n.newreplicaset {
		slog.Info("new replicaset detected, discovering new targets", "replicaSet", replicaset)
		n.newreplicaset = replicaset
		n.buildQueue(false)
	}

	// get next item from queue
	if len(n.queue) == 0 {
		err := fmt.Errorf("discovery queue is empty")
		return netbox.Filer{}, err
	}
	next := <-n.queue

	// set label on pod before returning
	err := n.setLabel(podName, "filer", next.Name)
	if err != nil {
		n.queue <- next
		return netbox.Filer{}, err
	}
	return next, nil
}

// discover queries netbox for filers and cache the filer list
func (n *NetAppSD) discover(lockq bool) {
	if lockq {
		n.mu.Lock()
		defer n.mu.Unlock()
	}
	slog.Info("fetching filers from netbox", "region", n.Region, "serviceType", n.ServiceType)
	filers, err := n.NetboxClient.GetFilers(n.Region, n.ServiceType)
	if err != nil {
		slog.Error("failed to fetch filers from netbox", "error", err)
	}
	slog.Info(fmt.Sprintf("fetched %d filers from netbox", len(filers)))
	n.filers = filers
}

// buildQueue builds a queue of filers to be harvested, based on the cached
// filer list and the pods that are already running
func (n *NetAppSD) buildQueue(lockq bool) {
	if lockq {
		n.mu.Lock()
		defer n.mu.Unlock()
	}

	// get labels from pods and check which filers are already harvested
	// label example: app=netapp-harvest, filer=netapp-qa-de-1-01
	running := make(map[string]bool)
	pods, err := n.getPods(n.AppLabel)
	if err != nil {
		msg := fmt.Sprintf("get pods with label %s", n.AppLabel)
		slog.Error(msg, "error", err)
	}
	for _, pod := range pods.Items {
		// check if pod belongs to n.newreplicaset
		if strings.Contains(pod.Name, n.newreplicaset) {
			lvalue, ok := pod.Labels["filer"]
			if ok {
				running[lvalue] = true
			}
		}
	}

	// build queue of filers that are not running
	targets := make([]netbox.Filer, 0)
	for _, filer := range n.filers {
		if !running[filer.Name] {
			targets = append(targets, filer)
			slog.Debug(fmt.Sprintf("added filer %s to queue", filer.Name))
		}
	}

	// close old queue before setting new one
	if n.queue != nil {
		close(n.queue)
	}
	n.queue = make(chan netbox.Filer, len(targets))
	for _, target := range targets {
		n.queue <- target
	}
}

// getPods is an example to get pods in a k8s namespace filtered by label
func (n *NetAppSD) getPods(label string) (*corev1.PodList, error) {
	return n.KubernetesClientset.CoreV1().Pods(n.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})

}

// setLabel sets the filer label on the pod
func (n *NetAppSD) setLabel(podName, key, value string) error {
	pod, err := n.KubernetesClientset.CoreV1().Pods(n.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	pod.Labels[key] = value
	_, err = n.KubernetesClientset.CoreV1().Pods(n.Namespace).Update(context.Background(), pod, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
