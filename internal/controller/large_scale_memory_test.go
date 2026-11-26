// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

const (
	// largeScaleEnv gates the tests that create thousands of large objects.
	// They are opt-in because they take several minutes and use a lot of
	// memory in both the test process and the envtest API server.
	largeScaleEnv = "FLUX_OPERATOR_LARGE_SCALE_TESTS"

	largeScaleCount     = 5000
	largeScaleNamespace = "large-scale"
	largeScaleTimeout   = 45 * time.Minute

	// defaultMaxPeakMiB is a coarse guard against reintroducing the
	// unbounded list. With metadata-only migration and 500-object report
	// pages the measured peak is ~100-140 MiB; without those optimizations
	// it is ~1 GiB.
	defaultMaxPeakMiB = 512

	helmReleaseAPIVersion = fluxcdv1.FluxHelmGroup + "/v2"
)

var (
	largeScaleOnce       sync.Once
	largeScaleSetupErr   error
	largeScaleClientOnce sync.Once
	largeScaleClient     client.Client
	largeScaleClientErr  error

	heapProfileSeq atomic.Int64
)

// largeScaleObjects returns the object count, allowing a smaller dataset for
// quick local sanity checks.
func largeScaleObjects() int {
	if v := os.Getenv("FLUX_OPERATOR_LARGE_SCALE_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return largeScaleCount
}

// TestLargeScaleFluxReport measures the memory used by the FluxReport path,
// which lists every Flux custom resource across all namespaces in one call.
//
// Run it with:
//
//	FLUX_OPERATOR_LARGE_SCALE_TESTS=1 go test ./internal/controller \
//	  -run TestLargeScaleFluxReport -v -timeout 60m
func TestLargeScaleFluxReport(t *testing.T) {
	ensureLargeScaleDataset(t)
	count := largeScaleObjects()

	ctx, cancel := context.WithTimeout(testCtx, largeScaleTimeout)
	defer cancel()

	// The reporter resolves the cluster info through client-go's config
	// loader, so point it at the envtest kubeconfig.
	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(kubeconfigPath, testKubeConfig, 0600); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", kubeconfigPath)

	kc := maybeProfileClient(t, largeScaleKubeClient(t))
	rep := reporter.NewFluxStatusReporter(kc, fluxcdv1.DefaultInstanceName, controllerName, largeScaleNamespace)

	before := readMemory()
	stopSampler := startMemorySampler()
	result, err := rep.Compute(ctx)
	peak := stopSampler()
	after := readMemory()
	if err != nil {
		t.Fatalf("failed to compute FluxReport: %v", err)
	}

	var hrStatus *fluxcdv1.FluxReconcilerStatus
	for i := range result.Spec.ReconcilersStatus {
		if result.Spec.ReconcilersStatus[i].Kind == fluxcdv1.FluxHelmReleaseKind {
			hrStatus = &result.Spec.ReconcilersStatus[i]
			break
		}
	}
	if hrStatus == nil {
		t.Fatalf("FluxReport did not contain HelmRelease stats")
	}
	if hrStatus.Stats.Running != count {
		t.Fatalf("expected %d running HelmReleases, got %d", count, hrStatus.Stats.Running)
	}
	if got := len(result.Resources); got < count {
		t.Fatalf("expected at least %d resources, got %d", count, got)
	}

	logMemoryStats(t, "FluxReport", before, peak, after)
	assertPeakMemory(t, "FluxReport", peak)
	writeHeapProfile(t, "fluxreport")
}

// TestLargeScaleMigrateResources measures the memory used by the FluxInstance
// API version migration path, which lists the metadata of all resources of
// each CRD in one call and then patches them.
//
// Run it with:
//
//	FLUX_OPERATOR_LARGE_SCALE_TESTS=1 go test ./internal/controller \
//	  -run TestLargeScaleMigrateResources -v -timeout 60m
func TestLargeScaleMigrateResources(t *testing.T) {
	ensureLargeScaleDataset(t)
	count := largeScaleObjects()

	ctx, cancel := context.WithTimeout(testCtx, largeScaleTimeout)
	defer cancel()

	kc := maybeProfileClient(t, largeScaleKubeClient(t))
	recorder := &patchRecorderClient{Client: kc, patchTypes: make(map[types.PatchType]int)}
	r := &FluxInstanceReconciler{
		Client:        recorder,
		APIReader:     kc,
		Scheme:        NewTestScheme(),
		StatusManager: controllerName,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
	}

	before := readMemory()
	stopSampler := startMemorySampler()
	err := r.migrateResources(ctx, client.MatchingLabels{"app.kubernetes.io/part-of": fluxcdv1.DefaultInstanceName}, true)
	peak := stopSampler()
	after := readMemory()
	if err != nil {
		t.Fatalf("failed to migrate resources: %v", err)
	}

	// The migration lists all objects in one call and patches each of them.
	// Assert on the patches attempted instead of on managedFields, because
	// recent envtest API servers may ignore client-provided managedFields.
	if got := recorder.patchTypes[types.JSONPatchType]; got != count {
		t.Fatalf("expected %d JSON patches, got %d", count, got)
	}
	if got := len(recorder.patched); got != count {
		t.Fatalf("expected %d patched objects, got %d", count, got)
	}

	// Confirm the objects are still all present after the migration.
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(helmReleaseAPIVersion)
	list.SetKind(fluxcdv1.FluxHelmReleaseKind + "List")
	if err := kc.List(ctx, list, client.InNamespace(largeScaleNamespace)); err != nil {
		t.Fatalf("failed to list migrated HelmReleases: %v", err)
	}
	if len(list.Items) != count {
		t.Fatalf("expected %d HelmReleases, got %d", count, len(list.Items))
	}

	logMemoryStats(t, "MigrateResources", before, peak, after)
	assertPeakMemory(t, "MigrateResources", peak)
	writeHeapProfile(t, "migrate")
}

// ensureLargeScaleDataset creates the large HelmRelease dataset once per test
// binary. Both tests can be run individually or together.
func ensureLargeScaleDataset(t *testing.T) {
	t.Helper()
	if os.Getenv(largeScaleEnv) != "1" {
		t.Skipf("set %s=1 to run the large-scale memory tests", largeScaleEnv)
	}

	ctx, cancel := context.WithTimeout(testCtx, largeScaleTimeout)
	defer cancel()

	largeScaleOnce.Do(func() {
		largeScaleSetupErr = setupLargeScaleDataset(ctx, t)
	})
	if largeScaleSetupErr != nil {
		t.Fatalf("failed to set up large-scale dataset: %v", largeScaleSetupErr)
	}
}

func setupLargeScaleDataset(ctx context.Context, t *testing.T) error {
	t.Helper()

	// Replace the Flux CRDs used by the other tests so the HelmRelease CRD
	// has the served v2beta1 version that produces stale managedFields.
	crds := []string{
		filepath.Join("testdata", "large-scale", "helmrelease-crd.yaml"),
		filepath.Join("testdata", "large-scale", "kustomization-crd.yaml"),
	}
	for _, path := range crds {
		if err := installLargeScaleCRD(ctx, path); err != nil {
			return err
		}
	}

	// Create the namespace that holds the large objects.
	if err := createLargeScaleNamespace(ctx); err != nil {
		return err
	}

	template, err := loadLargeScaleHelmReleaseTemplate()
	if err != nil {
		return err
	}

	count := largeScaleObjects()
	t.Logf("creating %d HelmRelease objects in namespace %s", count, largeScaleNamespace)
	start := time.Now()

	kc := largeScaleKubeClient(t)
	var created atomic.Int32
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for i := 0; i < count; i++ {
		g.Go(func() error {
			obj := template.DeepCopy()
			obj.SetName(fmt.Sprintf("kube-prometheus-stack-%05d", i))
			obj.SetNamespace(largeScaleNamespace)
			if err := kc.Create(gctx, obj); err != nil && !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create object %d: %w", i, err)
			}
			n := created.Add(1)
			if n%500 == 0 {
				t.Logf("created %d/%d HelmReleases", n, count)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Make sure the API server has indexed all objects before measuring.
	if err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			list := &unstructured.UnstructuredList{}
			list.SetAPIVersion(helmReleaseAPIVersion)
			list.SetKind(fluxcdv1.FluxHelmReleaseKind + "List")
			if err := kc.List(ctx, list, client.InNamespace(largeScaleNamespace)); err != nil {
				return false, err
			}
			return len(list.Items) == count, nil
		}); err != nil {
		return fmt.Errorf("timed out waiting for %d HelmReleases: %w", count, err)
	}

	t.Logf("created %d HelmRelease objects in %s", count, time.Since(start).Round(time.Second))
	return nil
}

func createLargeScaleNamespace(ctx context.Context) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: largeScaleNamespace},
	}
	if err := largeScaleKubeClient(nil).Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace %s: %w", largeScaleNamespace, err)
	}
	return nil
}

func installLargeScaleCRD(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read CRD %s: %w", path, err)
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(data, crd); err != nil {
		return fmt.Errorf("failed to decode CRD %s: %w", path, err)
	}

	kc := largeScaleKubeClient(nil)
	existing := &apiextensionsv1.CustomResourceDefinition{}
	err = kc.Get(ctx, client.ObjectKey{Name: crd.Name}, existing)
	if err == nil {
		if err := kc.Delete(ctx, existing); err != nil {
			return fmt.Errorf("failed to delete existing CRD %s: %w", crd.Name, err)
		}
		if err := wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, time.Minute, true,
			func(ctx context.Context) (bool, error) {
				err := kc.Get(ctx, client.ObjectKey{Name: crd.Name}, &apiextensionsv1.CustomResourceDefinition{})
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				return false, err
			}); err != nil {
			return fmt.Errorf("timed out waiting for CRD %s deletion: %w", crd.Name, err)
		}
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get CRD %s: %w", crd.Name, err)
	}

	if err := kc.Create(ctx, crd); err != nil {
		return fmt.Errorf("failed to create CRD %s: %w", crd.Name, err)
	}

	return wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, time.Minute, true,
		func(ctx context.Context) (bool, error) {
			current := &apiextensionsv1.CustomResourceDefinition{}
			if err := kc.Get(ctx, client.ObjectKey{Name: crd.Name}, current); err != nil {
				return false, err
			}
			for _, cond := range current.Status.Conditions {
				if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
					return true, nil
				}
			}
			return false, nil
		})
}

func loadLargeScaleHelmReleaseTemplate() (*unstructured.Unstructured, error) {
	data, err := os.ReadFile(filepath.Join("testdata", "large-scale", "helmrelease.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read HelmRelease template: %w", err)
	}
	var obj map[string]any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("failed to decode HelmRelease template: %w", err)
	}
	return &unstructured.Unstructured{Object: obj}, nil
}

func largeScaleKubeClient(t *testing.T) client.Client {
	largeScaleClientOnce.Do(func() {
		cfg := rest.CopyConfig(testEnv.Config)
		cfg.QPS = 200
		cfg.Burst = 400
		largeScaleClient, largeScaleClientErr = client.New(cfg, client.Options{Scheme: NewTestScheme(), Cache: nil})
	})
	if largeScaleClientErr != nil {
		if t != nil {
			t.Fatalf("failed to create large-scale client: %v", largeScaleClientErr)
		}
		panic(largeScaleClientErr)
	}
	return largeScaleClient
}

// maybeProfileClient wraps the client with a heap profiler when
// FLUX_OPERATOR_HEAP_PROFILE_DIR is set. The profile is written right after a
// large list has been decoded, while the list is still live, which points
// directly at the objects holding memory.
func maybeProfileClient(t *testing.T, c client.Client) client.Client {
	dir := os.Getenv("FLUX_OPERATOR_HEAP_PROFILE_DIR")
	if dir == "" {
		return c
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create heap profile dir: %v", err)
	}
	return &heapProfileClient{Client: c, dir: dir}
}

type heapProfileClient struct {
	client.Client

	dir string
}

func (c *heapProfileClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.Client.List(ctx, list, opts...); err != nil {
		return err
	}

	ul, ok := list.(*unstructured.UnstructuredList)
	if !ok || len(ul.Items) < 100 {
		return nil
	}

	n := heapProfileSeq.Add(1)
	name := fmt.Sprintf("list-%02d-%s-%d.pb.gz", n, ul.GetKind(), len(ul.Items))
	f, err := os.Create(filepath.Join(c.dir, name))
	if err != nil {
		return nil
	}
	defer f.Close()

	runtime.GC()
	return pprof.WriteHeapProfile(f)
}

// patchRecorderClient records the patches sent by the migration path.
type patchRecorderClient struct {
	client.Client

	patched    []string
	patchTypes map[types.PatchType]int
}

func (c *patchRecorderClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patched = append(c.patched, obj.GetNamespace()+"/"+obj.GetName())
	c.patchTypes[patch.Type()]++
	return c.Client.Patch(ctx, obj, patch, opts...)
}

type memorySnapshot struct {
	heapAlloc   uint64
	heapInuse   uint64
	heapObjects uint64
	totalAlloc  uint64
	sys         uint64
}

func readMemory() memorySnapshot {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return memorySnapshot{
		heapAlloc:   ms.HeapAlloc,
		heapInuse:   ms.HeapInuse,
		heapObjects: ms.HeapObjects,
		totalAlloc:  ms.TotalAlloc,
		sys:         ms.Sys,
	}
}

// startMemorySampler records the highest HeapAlloc observed while the measured
// operation is running. The returned function stops the sampler and returns
// the peak.
func startMemorySampler() func() uint64 {
	var peak atomic.Uint64
	done := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		defer close(finished)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				updatePeak(&peak, ms.HeapAlloc)
			}
		}
	}()

	return func() uint64 {
		close(done)
		<-finished
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		updatePeak(&peak, ms.HeapAlloc)
		return peak.Load()
	}
}

func updatePeak(peak *atomic.Uint64, value uint64) {
	for {
		current := peak.Load()
		if value <= current || peak.CompareAndSwap(current, value) {
			return
		}
	}
}

// writeHeapProfile writes a profile after the measured operation, including
// the allocations that have already been freed. This is useful for finding
// which code paths allocate the most, not just what is retained.
func writeHeapProfile(t *testing.T, label string) {
	t.Helper()
	dir := os.Getenv("FLUX_OPERATOR_HEAP_PROFILE_DIR")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("after-%s.pb.gz", label)))
	if err != nil {
		t.Logf("failed to create heap profile: %v", err)
		return
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Logf("failed to write heap profile: %v", err)
	}
}

// assertPeakMemory fails when the operation used more peak heap than the
// configured limit. The limit can be overridden with
// FLUX_OPERATOR_LARGE_SCALE_MAX_PEAK_MIB.
func assertPeakMemory(t *testing.T, name string, peak uint64) {
	t.Helper()

	maxMiB := defaultMaxPeakMiB
	if v := os.Getenv("FLUX_OPERATOR_LARGE_SCALE_MAX_PEAK_MIB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxMiB = n
		}
	}

	peakMiB := peak / (1024 * 1024)
	if peakMiB > uint64(maxMiB) {
		t.Fatalf("%s peak heap %d MiB exceeds limit %d MiB; check that listing is still paginated",
			name, peakMiB, maxMiB)
	}
}

func logMemoryStats(t *testing.T, name string, before memorySnapshot, peak uint64, after memorySnapshot) {
	t.Helper()
	const mib = 1024 * 1024
	t.Logf("%s memory: heap before=%dMiB peak=%dMiB after=%dMiB delta=%dMiB allocated=%dMiB sys=%dMiB objects=%d",
		name,
		before.heapAlloc/mib,
		peak/mib,
		after.heapAlloc/mib,
		(peak-before.heapAlloc)/mib,
		(after.totalAlloc-before.totalAlloc)/mib,
		after.sys/mib,
		after.heapObjects,
	)
}
