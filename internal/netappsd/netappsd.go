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

	"github.com/rs/zerolog/log"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
)

type NetAppSD struct {
	AppLabel            string
	Namespace           string
	Region              string
	ServiceType         string
	NetboxClient        *netbox.Client
	KubernetesClientset *kubernetes.Clientset
	mu                  sync.Mutex
	newreplicaset       string
	queue               chan netbox.Filer
}

// Discover starts a discovery loop that runs every 5 minutes. It can be
// canceled by sending a signal to the cancel channel.
func (n *NetAppSD) Discover(cancel <-chan struct{}) {
	for ; true; <-time.After(5 * time.Minute) {
		select {
		case <-cancel:
			return
		default:
			func() {
				n.discover(true)
			}()
		}
	}
}

// NextItem returns the next item from the queue
func (n *NetAppSD) Next(podName string) (netbox.Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// discover new targets if request is from new replicaset
	r := strings.Split(podName, "-")
	replicaset := strings.Join(r[:len(r)-1], "-")
	if replicaset != n.newreplicaset {
		slog.Info("new replicaset detected, discovering new targets", "replicaSet", replicaset)
		n.newreplicaset = replicaset
		n.discover(false)
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

func (n *NetAppSD) discover(lockq bool) {
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

	// query netbox for filers and add to queue if not already harvested
	discovered := n.queryNetbox()
	targets := make([]netbox.Filer, 0)
	for _, filer := range discovered {
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

// queryNetbox queries netbox for targets and queries the pods for running
// tasks and returns the diff
func (n *NetAppSD) queryNetbox() []netbox.Filer {
	filers, err := n.NetboxClient.GetFilers(n.Region, n.ServiceType)
	slog.Debug(fmt.Sprintf("fetched %d filers from netbox", len(filers)))
	if err != nil {
		log.Warn().Err(err).Send()
	}
	return filers
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
