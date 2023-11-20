package netappsd

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rs/zerolog/log"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
)

type NetAppSD struct {
	AppLabel            string
	Namespace           string
	NetboxClient        *netbox.Client
	KubernetesClientset *kubernetes.Clientset
	mu                  sync.Mutex
	queue               chan netbox.Filer
}

// Discover starts a discovery loop that runs every 5 minutes. It can be
// canceled by sending a signal to the cancel channel.
func (n *NetAppSD) Discover(cancel <-chan struct{}) {
	n.discover()
	for {
		select {
		case <-cancel:
			return
		case <-time.After(300 * time.Second):
			n.discover()
		}
	}
}

// NextItem returns the next item from the queue
func (n *NetAppSD) Next() (netbox.Filer, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.queue) == 0 {
		err := fmt.Errorf("discovery queue is empty")
		return netbox.Filer{}, err
	}
	return <-n.queue, nil
}

func (n *NetAppSD) discover() {
	n.mu.Lock()
	defer n.mu.Unlock()

	// close old queue before discovering new targets
	if n.queue != nil {
		close(n.queue)
	}
	targets := n.queryNetbox()
	n.queue = make(chan netbox.Filer, len(targets))
	for _, target := range targets {
		n.queue <- target
	}
}

// queryNetbox queries netbox for targets and queries the pods for running
// tasks and returns the diff
func (n *NetAppSD) queryNetbox() []netbox.Filer {
	filers, err := n.NetboxClient.GetFilers("qa-de-1", "manila")
	if err != nil {
		log.Warn().Err(err).Send()
	}
	return filers
	// return []string{"hello", "world"}
}

// getPods is an example to get pods in a k8s namespace filtered by label
func (n *NetAppSD) getPods() {
	for {
		pods, err := n.KubernetesClientset.CoreV1().Pods(n.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: n.AppLabel,
		})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
		time.Sleep(10 * time.Second)
	}
}
