package netboxsd

import (
	"sync"
	"time"

	"github.com/sapcc/netappsd/internal/pkg/netbox"
)

type NetboxSD struct {
	netbox *netbox.Client
	queue  chan string
	mu     sync.Mutex
}

func NewNetAppSD(netboxURL, netboxToken string) *NetboxSD {
	netboxClient, err := netbox.NewClient(netboxURL, netboxToken)
	if err != nil {
		panic(err)
	}
	return &NetboxSD{
		netbox: &netboxClient,
		queue:  make(chan string),
	}
}

// Discover starts the discovery process. It queries netbox for targets and
// sends them to the queue every 5 minutes. The discovery process can be
// stopped by sending a signal to the cancel channel.
func (n *NetboxSD) Discover(cancel <-chan struct{}) {
	n.discover()

	go func() {
		for {
			select {
			case <-cancel:
				return
			case <-time.After(300 * time.Second):
				n.discover()
			}
		}
	}()
}

// NextItem returns the next item from the queue
func (n *NetboxSD) NextItem() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return <-n.queue
}

func (n *NetboxSD) discover() {
	n.mu.Lock()
	defer n.mu.Unlock()

	// close old queue before discovering new targets
	close(n.queue)
	targets := n.queryNetbox()
	n.queue = make(chan string, len(targets))
	for _, target := range targets {
		n.queue <- target
	}
}

// queryNetbox queries netbox for targets and queries the pods for running
// tasks and returns the diff
func (n *NetboxSD) queryNetbox() []string {
	return []string{"hello", "world"}
}
