package netappsd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// testTemplate renders a minimal per-filer Deployment. %s is the container
// image, which the tests vary to simulate a template (ConfigMap) change.
const testTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: netappsd-worker
          image: %s
`

// newTestNetAppSD builds a NetAppSD wired to a fake clientset with the given
// filers already discovered and probed "now". The Deployment template is
// written to a temp file (reconcile reloads it from disk each cycle); the
// returned path can be rewritten to simulate a ConfigMap change.
func newTestNetAppSD(t *testing.T, filers ...string) (*NetAppSD, string) {
	t.Helper()
	tplPath := filepath.Join(t.TempDir(), "deployment.yaml.tpl")
	writeTemplate(t, tplPath, "netappsd:v1")

	n := &NetAppSD{
		Namespace:          "test-ns",
		FilerTag:           "test",
		ManagedLabel:       "app.kubernetes.io/managed-by=netappsd",
		DeploymentTemplate: tplPath,
		kubeClientset:      fake.NewSimpleClientset(),
		filerList:          make(map[string]Filer),
		inactiveFilers:     make(map[string]struct{}),
	}
	n.lastProbeFilerTs = SyncMapTimestamp{}
	if err := n.loadTemplate(); err != nil {
		t.Fatalf("load template: %v", err)
	}
	for _, f := range filers {
		n.filerList[f] = Filer{Name: f, Host: f + ".example", Service: "test"}
		n.lastProbeFilerTs.Store(f, time.Now().Unix())
	}
	return n, tplPath
}

// TestBuildDeploymentAZLabel verifies that a filer with an availability zone
// produces a Deployment whose metadata and pod-template carry the AZ label, but
// whose (immutable) selector does NOT — so a later AZ change cannot break the
// reconcile Update. A filer without an AZ must carry no AZ label at all.
func TestBuildDeploymentAZLabel(t *testing.T) {
	n, _ := newTestNetAppSD(t)

	t.Run("with AZ", func(t *testing.T) {
		dep, err := n.buildDeployment(Filer{Name: "filer-a", Host: "filer-a.example", Service: "test", AvailabilityZone: "eu-de-1a"})
		if err != nil {
			t.Fatalf("buildDeployment: %v", err)
		}
		if got := dep.Labels[azLabelKey]; got != "eu-de-1a" {
			t.Fatalf("deployment metadata AZ label = %q, want eu-de-1a", got)
		}
		if got := dep.Spec.Template.Labels[azLabelKey]; got != "eu-de-1a" {
			t.Fatalf("pod template AZ label = %q, want eu-de-1a", got)
		}
		if _, ok := dep.Spec.Selector.MatchLabels[azLabelKey]; ok {
			t.Fatalf("AZ label must not be in the immutable selector, but it is present")
		}
	})

	t.Run("without AZ", func(t *testing.T) {
		dep, err := n.buildDeployment(Filer{Name: "filer-b", Host: "filer-b.example", Service: "test"})
		if err != nil {
			t.Fatalf("buildDeployment: %v", err)
		}
		if _, ok := dep.Spec.Template.Labels[azLabelKey]; ok {
			t.Fatalf("expected no AZ label when filer AZ is empty, but it is present")
		}
	})
}

func writeTemplate(t *testing.T, path, image string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(fmt.Sprintf(testTemplate, image)), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}
}

func managedCount(t *testing.T, n *NetAppSD) int {
	t.Helper()
	list, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: n.labelSelector(),
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return len(list.Items)
}

// TestReconcileCreatesDeployments verifies a first reconcile creates one
// deployment per desired filer and a second no-op reconcile changes nothing.
func TestReconcileCreatesDeployments(t *testing.T) {
	n, _ := newTestNetAppSD(t, "filer-a", "filer-b", "filer-c")
	if err := n.reconcileDeployments(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := managedCount(t, n); got != 3 {
		t.Fatalf("expected 3 deployments, got %d", got)
	}
	if err := n.reconcileDeployments(context.Background()); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	if got := managedCount(t, n); got != 3 {
		t.Fatalf("expected 3 deployments after no-op reconcile, got %d", got)
	}
}

// TestReconcileUpdatesOnTemplateChange verifies that editing the template file
// (as a ConfigMap change would) updates the existing deployment's spec-hash.
func TestReconcileUpdatesOnTemplateChange(t *testing.T) {
	ctx := context.Background()
	n, tplPath := newTestNetAppSD(t, "filer-a")
	if err := n.reconcileDeployments(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	before, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, "filer-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	oldHash := before.Annotations[specHashKey]

	// simulate a ConfigMap edit: change the rendered image, then reconcile.
	writeTemplate(t, tplPath, "netappsd:v2")
	if err := n.reconcileDeployments(ctx); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	after, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).Get(ctx, "filer-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get 2: %v", err)
	}
	if after.Annotations[specHashKey] == oldHash {
		t.Fatalf("expected spec-hash to change after template change, still %s", oldHash)
	}
	if got := after.Spec.Template.Spec.Containers[0].Image; got != "netappsd:v2" {
		t.Fatalf("expected image netappsd:v2, got %s", got)
	}
}

// TestReconcileCapsUpdates verifies that no more than maxUpdatesPerReconcile
// deployments are updated in a single cycle after a template change.
func TestReconcileCapsUpdates(t *testing.T) {
	ctx := context.Background()
	total := maxUpdatesPerReconcile + 5
	names := make([]string, total)
	for i := range names {
		names[i] = fmt.Sprintf("filer-%02d", i)
	}
	n, tplPath := newTestNetAppSD(t, names...)
	if err := n.reconcileDeployments(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// change the template so every deployment's desired spec differs.
	writeTemplate(t, tplPath, "netappsd:v2")
	if err := n.reconcileDeployments(ctx); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}

	// count how many now carry the new image (i.e. were updated this cycle).
	list, err := n.kubeClientset.AppsV1().Deployments(n.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: n.labelSelector(),
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	updatedNow := 0
	for i := range list.Items {
		if list.Items[i].Spec.Template.Spec.Containers[0].Image == "netappsd:v2" {
			updatedNow++
		}
	}
	if updatedNow != maxUpdatesPerReconcile {
		t.Fatalf("expected exactly %d updated in one cycle, got %d", maxUpdatesPerReconcile, updatedNow)
	}

	// a second reconcile updates the remaining ones (still capped, but only 5 left).
	if err := n.reconcileDeployments(ctx); err != nil {
		t.Fatalf("reconcile 3: %v", err)
	}
	if got := managedCount(t, n); got != total {
		t.Fatalf("expected %d deployments, got %d", total, got)
	}
	list, _ = n.kubeClientset.AppsV1().Deployments(n.Namespace).List(ctx, metav1.ListOptions{LabelSelector: n.labelSelector()})
	allUpdated := 0
	for i := range list.Items {
		if list.Items[i].Spec.Template.Spec.Containers[0].Image == "netappsd:v2" {
			allUpdated++
		}
	}
	if allUpdated != total {
		t.Fatalf("expected all %d updated after two cycles, got %d", total, allUpdated)
	}
}
