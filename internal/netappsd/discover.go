package netappsd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/sapcc/netappsd/internal/pkg/netapp"
	"github.com/sapcc/netappsd/internal/pkg/netbox"
	"github.com/sapcc/netappsd/internal/pkg/utils"
)

// filerLabelKey is the label key set on each managed Deployment holding the
// name of the filer it exports. It is used to map a Deployment back to its
// filer during reconciliation.
const filerLabelKey = "netappsd/filer"

// serviceLabelKey is the label key set on each managed Deployment holding the
// service (filer tag) it belongs to. It scopes reconciliation so that masters
// for different services (e.g. cinder, manila, apod) do not manage or delete
// each other's Deployments even though they share the managed-by label.
const serviceLabelKey = "netappsd/service"

// azLabelKey is the label key set on each managed worker pod holding the
// filer's availability zone (e.g. "eu-de-1a"). It lets a PodMonitor select
// pods by AZ so Prometheus can be functionally sharded per (service, AZ).
const azLabelKey = "netappsd/availability-zone"

// specHashKey is the annotation key holding a hash of the rendered Deployment
// spec. It lets reconciliation detect when the template (ConfigMap) changed and
// update existing Deployments accordingly.
const specHashKey = "netappsd/spec-hash"

// maxUpdatesPerReconcile caps how many existing Deployments are updated in a
// single reconcile cycle. A template change would otherwise roll every worker
// at once; capping staggers the rollout across cycles (reconcile runs every
// 30s), so at most this many workers restart simultaneously.
const maxUpdatesPerReconcile = 10

// discoveryInterval is how often filers are re-discovered from Netbox and
// re-probed, refreshing each filer's last-probe timestamp.
const discoveryInterval = 5 * time.Minute

// filerStaleAfter is how long a filer may go without a successful probe before
// it is considered stale and its deployment removed. It must be comfortably
// larger than discoveryInterval: if the two were equal, a reconcile firing just
// before the next discovery tick would see timestamps at exactly the boundary
// and wrongly delete every deployment moments before discovery refreshes them.
// Using a multiple means a filer is only dropped after discovery has genuinely
// failed to refresh it for several cycles.
const filerStaleAfter = 3 * discoveryInterval

type Filer netbox.Filer

type NetAppSD struct {
	NetboxHost         string
	NetboxToken        string
	Namespace          string
	Region             string
	FilerTag           string
	ManagedLabel       string
	DeploymentTemplate string
	NetAppUsername     string
	NetAppPassword     string

	filerList        map[string]Filer
	lastProbeError   error
	lastProbeFilerTs SyncMapTimestamp
	inactiveFilers   map[string]struct{}
	deploymentTpl    *template.Template

	netboxClient  *netbox.Client
	kubeClientset kubernetes.Interface
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
// the filers every 5 minutes and a goroutine that reconciles one Deployment
// per discovered filer.
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

	// Load and parse the per-filer Deployment template. It is reloaded on every
	// reconcile so changes to the mounted ConfigMap are picked up without a
	// restart; loading here as well lets us fail fast on a broken template.
	if err := n.loadTemplate(); err != nil {
		return err
	}

	n.lastProbeFilerTs = SyncMapTimestamp{}
	n.filerList = make(map[string]Filer)
	discoveryDone := make(chan struct{})
	n.inactiveFilers = make(map[string]struct{})

	go func() {
		defer close(discoveryDone)
		tick := new(utils.TickTick)

		for {
			select {
			case <-tick.Every(discoveryInterval):
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
			case <-discoveryDone: // reconcile after filer discovery
			case <-time.After(30 * time.Second): // reconcile every 30 seconds to correct drift
			case <-ctx.Done():
				return
			}
			if err := n.reconcileDeployments(ctx); err != nil {
				slog.Error("reconcile deployments failed", "error", err)
			}
		}
	}()

	return nil
}

// GetFiler returns a copy of the filer with the given name from the filer list.
func (n *NetAppSD) GetFiler(name string) (*Filer, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	filer, found := n.filerList[name]
	if !found {
		return nil, false
	}
	return &filer, true
}

func (n *NetAppSD) IsReady() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.filerList) > 0
}

// discoverFilers queries netbox for filers, probes them in parallel, and
// updates the filer list. Filers no longer present in the discovery result are
// pruned so that their deployments are removed on the next reconcile.
func (n *NetAppSD) discoverFilers(ctx context.Context) (int, int, error) {
	filers, err := n.netboxClient.GetFilers(ctx, n.Region, n.FilerTag)
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

	// track the filers seen in this discovery cycle so we can prune the ones
	// that disappeared from Netbox.
	seen := make(map[string]struct{}, len(filers))
	var seenMu sync.Mutex

	for _, f := range filers {
		seen[f.Name] = struct{}{}
		if f.Status != "active" {
			n.inactiveFilers[f.Name] = struct{}{}
			slog.Info("filer's status is not active in Netbox", "filer", f.Name, "status", f.Status)
			continue
		} else {
			delete(n.inactiveFilers, f.Name)
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

			seenMu.Lock()
			// initialize filer list if not exists
			if _, found := n.filerList[filer.Name]; !found {
				slog.Info("new filer discovered", "filer", filer.Name)
			}
			n.filerList[filer.Name] = filer
			seenMu.Unlock()

			// update filer probing timestamp
			n.lastProbeFilerTs.Store(filer.Name, time.Now().Unix())
		}(Filer(f))
	}

	wg.Wait()

	// prune filers that are no longer discovered from Netbox.
	for name := range n.filerList {
		if _, ok := seen[name]; !ok {
			slog.Info("filer no longer discovered, pruning", "filer", name)
			delete(n.filerList, name)
			n.lastProbeFilerTs.Delete(name)
		}
	}

	return int(successCounter.Load()), int(failedCounter.Load()), nil
}

func (n *NetAppSD) probeFiler(ctx context.Context, filer Filer) error {
	filerAddress := filer.Ip
	if filer.Ip == "" {
		filerAddress = filer.Host
	}
	slog.Debug("probing filer", "filer", filer.Name, "addr", filerAddress, "facility", filer.Facility)
	c := netapp.NewFilerClient(filerAddress, n.NetAppUsername, n.NetAppPassword)
	return c.Probe(ctx)
}

// reconcileDeployments converges the set of managed Deployments to match the
// set of discovered, recently-probed, active filers. It creates a Deployment
// for each new filer, updates existing ones whose rendered spec changed (e.g.
// the template ConfigMap was edited), and deletes the Deployment of any filer
// that is no longer desired.
func (n *NetAppSD) reconcileDeployments(ctx context.Context) error {
	desired := n.desiredFilers()

	// reload the template so edits to the mounted ConfigMap are picked up.
	if err := n.loadTemplate(); err != nil {
		return err
	}

	// list existing managed deployments and map them back to filer names.
	deployments, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.labelSelector(),
	})
	if err != nil {
		return fmt.Errorf("failed to list managed deployments: %w", err)
	}
	existing := make(map[string]*appsv1.Deployment, len(deployments.Items)) // filerName -> deployment
	for i := range deployments.Items {
		dep := &deployments.Items[i]
		if filerName, ok := dep.Labels[filerLabelKey]; ok {
			existing[filerName] = dep
		}
	}

	// create desired deployments that do not exist; update those whose rendered
	// spec changed.
	created, updated, deleted := 0, 0, 0
	pendingUpdates := 0
	for name, filer := range desired {
		want, err := n.buildDeployment(filer)
		if err != nil {
			slog.Error("failed to build deployment", "filer", name, "error", err)
			continue
		}

		cur, ok := existing[name]
		if !ok {
			if _, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Create(ctx, want, metav1.CreateOptions{}); err != nil {
				if apierrors.IsAlreadyExists(err) {
					continue
				}
				slog.Error("failed to create deployment", "filer", name, "error", err)
				continue
			}
			slog.Info("created deployment for filer", "filer", name)
			created++
			continue
		}

		// exists: update only if the desired spec hash differs from the live one.
		if cur.Annotations[specHashKey] == want.Annotations[specHashKey] {
			continue
		}
		// cap updates per cycle so a template change does not roll every worker
		// at once; the rest are updated on subsequent reconciles.
		if updated >= maxUpdatesPerReconcile {
			pendingUpdates++
			continue
		}
		want.ResourceVersion = cur.ResourceVersion // required for update
		if _, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Update(ctx, want, metav1.UpdateOptions{}); err != nil {
			slog.Error("failed to update deployment", "filer", name, "error", err)
			continue
		}
		slog.Info("updated deployment for filer (spec changed)", "filer", name)
		updated++
	}

	// delete deployments whose filer is no longer desired.
	foreground := metav1.DeletePropagationForeground
	for name, dep := range existing {
		if _, ok := desired[name]; ok {
			continue
		}
		slog.Info("deleting deployment for retired filer", "filer", name, "deployment", dep.Name)
		err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Delete(ctx, dep.Name, metav1.DeleteOptions{
			PropagationPolicy: &foreground,
		})
		if err != nil && !apierrors.IsNotFound(err) {
			slog.Error("failed to delete deployment", "filer", name, "deployment", dep.Name, "error", err)
			continue
		}
		deleted++
	}

	// update metrics
	managedDeployments.Reset()
	for _, filer := range desired {
		managedDeployments.WithLabelValues(filer.Name, filer.Host, filer.Ip).Set(1)
	}

	// summarize the reconcile outcome. When nothing changed, the desired set is
	// fully deployed and in sync.
	if created == 0 && updated == 0 && deleted == 0 && pendingUpdates == 0 {
		slog.Info("reconcile in sync", "service", n.FilerTag, "deployments", len(desired))
	} else {
		slog.Info("reconcile done", "service", n.FilerTag, "desired", len(desired),
			"existing", len(existing), "created", created, "updated", updated, "deleted", deleted,
			"pendingUpdates", pendingUpdates)
	}

	return nil
}

// desiredFilers returns the set of filers that should have a running
// deployment: active filers probed within the last 5 minutes. It takes a
// snapshot under the lock so that Kube I/O happens without holding it.
func (n *NetAppSD) desiredFilers() map[string]Filer {
	n.mu.Lock()
	defer n.mu.Unlock()

	desired := make(map[string]Filer, len(n.filerList))
	for name, filer := range n.filerList {
		if _, inactive := n.inactiveFilers[name]; inactive {
			continue
		}
		// Skip filers that have not been probed recently to avoid deploying
		// exporters for filers that are not reachable. filerStaleAfter is a
		// multiple of the discovery interval so a filer is only dropped after
		// discovery has failed to refresh it for several cycles, not merely
		// because the next discovery tick has not fired yet.
		lastProbeTime := n.lastProbeFilerTs.LoadTime(name)
		if time.Since(lastProbeTime) > filerStaleAfter {
			slog.Info("skip filer", "filer", name, "lastProbeTime", lastProbeTime)
			continue
		}
		desired[name] = filer
	}
	return desired
}

// loadTemplate reads and parses the per-filer Deployment template from disk and
// stores it on the receiver. It is called at startup and on every reconcile so
// that changes to the mounted ConfigMap are picked up without a restart.
func (n *NetAppSD) loadTemplate() error {
	tplBytes, err := os.ReadFile(n.DeploymentTemplate)
	if err != nil {
		return fmt.Errorf("failed to read deployment template %q: %w", n.DeploymentTemplate, err)
	}
	tpl, err := template.New("deployment").Parse(string(tplBytes))
	if err != nil {
		return fmt.Errorf("failed to parse deployment template: %w", err)
	}
	n.deploymentTpl = tpl
	return nil
}

// buildDeployment renders the Deployment template for the given filer and
// returns the resulting object. Identity (name, namespace, labels, selector)
// and the FILER_NAME env var are enforced in code so that a template mistake
// cannot break reconciliation. A hash of the rendered spec is stored in the
// spec-hash annotation so reconciliation can detect template changes.
func (n *NetAppSD) buildDeployment(filer Filer) (*appsv1.Deployment, error) {
	if errs := validation.IsDNS1123Subdomain(filer.Name); len(errs) > 0 {
		return nil, fmt.Errorf("filer name %q is not a valid deployment name: %v", filer.Name, errs)
	}

	var buf bytes.Buffer
	data := struct {
		Name             string
		Host             string
		AvailabilityZone string
		Service          string
		Namespace        string
	}{
		Name:             filer.Name,
		Host:             filer.Host,
		AvailabilityZone: filer.AvailabilityZone,
		Service:          filer.Service,
		Namespace:        n.Namespace,
	}
	if err := n.deploymentTpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to render deployment template: %w", err)
	}

	dep := &appsv1.Deployment{}
	if err := yaml.Unmarshal(buf.Bytes(), dep); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rendered deployment: %w", err)
	}

	// enforce identity in code.
	managedKey, managedVal := parseLabel(n.ManagedLabel)
	dep.Name = filer.Name
	dep.Namespace = n.Namespace
	setLabels(&dep.Labels, managedKey, managedVal, filer.Name, n.FilerTag)
	setLabels(&dep.Spec.Template.Labels, managedKey, managedVal, filer.Name, n.FilerTag)
	if dep.Spec.Selector == nil {
		dep.Spec.Selector = &metav1.LabelSelector{}
	}
	setLabels(&dep.Spec.Selector.MatchLabels, managedKey, managedVal, filer.Name, n.FilerTag)

	// AZ is a pod-template + metadata label only, deliberately NOT part of the
	// Deployment selector: selectors are immutable, and a filer's AZ can change.
	if az := filer.AvailabilityZone; az != "" {
		setAZLabel(&dep.Labels, az)
		setAZLabel(&dep.Spec.Template.Labels, az)
	}

	// inject FILER_NAME into the worker container.
	injected := false
	for i := range dep.Spec.Template.Spec.Containers {
		c := &dep.Spec.Template.Spec.Containers[i]
		if c.Name == "netappsd-worker" {
			c.Env = append(c.Env, corev1.EnvVar{Name: "FILER_NAME", Value: filer.Name})
			injected = true
			break
		}
	}
	if !injected {
		slog.Warn("no netappsd-worker container found in template, FILER_NAME not injected", "filer", filer.Name)
	}

	// record a hash of the desired spec so reconciliation can detect changes.
	hash, err := hashDeploymentSpec(dep)
	if err != nil {
		return nil, err
	}
	if dep.Annotations == nil {
		dep.Annotations = make(map[string]string)
	}
	dep.Annotations[specHashKey] = hash

	return dep, nil
}

// hashDeploymentSpec returns a stable hash of the parts of the Deployment that
// netappsd manages: the pod template and replica count. The hash is computed
// before the spec-hash annotation is set, so it is stable across reconciles.
func hashDeploymentSpec(dep *appsv1.Deployment) (string, error) {
	payload := struct {
		Labels   map[string]string      `json:"labels"`
		Replicas *int32                 `json:"replicas"`
		Template corev1.PodTemplateSpec `json:"template"`
	}{
		Labels:   dep.Labels,
		Replicas: dep.Spec.Replicas,
		Template: dep.Spec.Template,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to hash deployment spec: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// parseLabel splits a "key=value" label selector into its key and value. If no
// "=" is present, the whole string is treated as the key with an empty value.
func parseLabel(label string) (key, value string) {
	if i := strings.Index(label, "="); i >= 0 {
		return label[:i], label[i+1:]
	}
	return label, ""
}

// setLabels ensures the managed label, the filer label, and the service label
// are present in the given label map, initializing it if nil.
func setLabels(labels *map[string]string, managedKey, managedVal, filerName, service string) {
	if *labels == nil {
		*labels = make(map[string]string)
	}
	(*labels)[managedKey] = managedVal
	(*labels)[filerLabelKey] = filerName
	(*labels)[serviceLabelKey] = service
}

// setAZLabel sets the availability-zone label, initializing the map if nil.
func setAZLabel(labels *map[string]string, az string) {
	if *labels == nil {
		*labels = make(map[string]string)
	}
	(*labels)[azLabelKey] = az
}

// labelSelector returns the label selector used to list the Deployments this
// master owns. It is scoped by service (filer tag) so that masters for
// different services do not manage each other's Deployments.
func (n *NetAppSD) labelSelector() string {
	return fmt.Sprintf("%s,%s=%s", n.ManagedLabel, serviceLabelKey, n.FilerTag)
}
