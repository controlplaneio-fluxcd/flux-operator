// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	goruntime "runtime"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// ReportHandler handles GET /api/v1/report requests and returns the FluxReport from the cluster.
func (h *Handler) ReportHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the FluxReport from the cluster using the request context
	report, err := h.GetReport(req.Context())
	if err != nil {
		log.FromContext(req.Context()).Error(err, "cluster query failed")
		report = uninitialisedReport()
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Encode and send the response
	if err := json.NewEncoder(w).Encode(report); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetReport returns the cached FluxReport. If the cache is empty, it falls back to
// building a fresh report (this should only happen during initial startup).
func (h *Handler) GetReport(ctx context.Context) (*unstructured.Unstructured, error) {
	var report *unstructured.Unstructured
	var statsByNamespace []reporter.ReconcilerStatsByNamespace
	if cached, cachedStats := h.getCachedReport(); cached != nil {
		report, statsByNamespace = cached, cachedStats
	} else {
		r, computeResult, _, err := h.buildReport(ctx)
		if err != nil {
			return nil, err
		}
		report, statsByNamespace = r, computeResult.StatsByNamespace
	}

	// Get and modify the report spec
	spec, found := report.Object["spec"].(map[string]any)
	if !found {
		return nil, fmt.Errorf("report spec not found")
	}

	// Inject user info
	userInfo := map[string]any{
		"username": user.Username(ctx),
	}
	if imp := user.Permissions(ctx); !imp.IsEmpty() {
		userInfo["impersonation"] = imp
	}
	if p := user.Provider(ctx); len(p) > 0 {
		userInfo["provider"] = p
	}
	if s := user.SessionStart(ctx); s != nil {
		userInfo["sessionStart"] = s.Format(time.RFC3339)
	}
	// An access check failure hides the nodes dashboard link
	// and never fails the report.
	canViewNodes, err := h.kubeClient.CanListNodes(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to check the user access to nodes")
	}
	userInfo["canViewNodes"] = canViewNodes
	spec["userInfo"] = userInfo

	// Inject user-visible namespaces
	namespaces, _, err := h.kubeClient.ListUserNamespaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list user namespaces: %w", err)
	}
	spec["namespaces"] = namespaces

	// Compute stats filtered by user-visible namespaces and inject into report
	filteredStats := reporter.FilterReconcilerStatsByNamespaces(statsByNamespace, namespaces)
	spec["reconcilers"] = filteredStats

	return report, nil
}

// startReportCache starts a background goroutine that periodically refreshes the
// report cache. It returns a channel that is closed when the goroutine stops,
// which happens when the provided context is done.
func (h *Handler) startReportCache(ctx context.Context, reportInterval time.Duration) <-chan struct{} {
	// Build initial report synchronously
	h.refreshReportCache(ctx)

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)

		ticker := time.NewTicker(reportInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.refreshReportCache(ctx)
			}
		}
	}()

	return stopped
}

// refreshReportCache builds a fresh report and nodes snapshot
// and updates the cache.
func (h *Handler) refreshReportCache(ctx context.Context) {
	report, computeResult, nodes, err := h.buildReport(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) || ctx.Err() == nil {
			log.FromContext(ctx).Error(err, "failed to refresh report cache")
		}
		return
	}

	h.reportCacheMu.Lock()
	h.reportCache = report
	h.reportCacheStatsByNamespace = computeResult.StatsByNamespace
	// The snapshot is always from the same refresh as the report, so a
	// failed snapshot is dropped along with the report spec.nodes.
	h.nodesSnapshot = nodes
	h.reportCacheMu.Unlock()

	// Update the search index from the reporter's resource statuses.
	h.searchIndex.Update(computeResult.Resources)

	// Update the workload index from the reporter's inventory-derived workloads.
	h.workloadIndex.Update(computeResult.Workloads)
}

// getCachedReport returns the cached report if available.
func (h *Handler) getCachedReport() (*unstructured.Unstructured, []reporter.ReconcilerStatsByNamespace) {
	h.reportCacheMu.RLock()
	if h.reportCache == nil {
		h.reportCacheMu.RUnlock()
		return nil, nil
	}
	b, _ := json.Marshal(h.reportCache)
	statsByNamespace := h.reportCacheStatsByNamespace
	h.reportCacheMu.RUnlock()

	var obj unstructured.Unstructured
	_ = json.Unmarshal(b, &obj)
	return &obj, statsByNamespace
}

// getCachedNodesSnapshot returns the cached nodes snapshot if available.
// The snapshot is replaced, never mutated, so it is safe to share.
func (h *Handler) getCachedNodesSnapshot() *NodesSnapshot {
	h.reportCacheMu.RLock()
	defer h.reportCacheMu.RUnlock()
	return h.nodesSnapshot
}

// buildReport builds the FluxReport directly using the reporter package,
// injects pod metrics into the report spec and builds the nodes snapshot,
// whose name-less summary is injected as spec.nodes. The snapshot is nil
// and spec.nodes is left out when it fails to build.
func (h *Handler) buildReport(ctx context.Context) (*unstructured.Unstructured, *reporter.FluxStatusReport, *NodesSnapshot, error) {
	// The report client needs privileged access as it needs to access all
	// resources in the cluster to build the report. The report information,
	// however, is crunched in a way that does not expose sensitive information.
	// This allows us to keep a good UX for unprivileged users while
	// ensuring security boundaries are respected.
	// Note: The report is built on a background goroutine periodically anyway,
	// so there's no user session available to use for impersonation.
	repClient := h.kubeClient.GetClient(ctx, kubeclient.WithPrivileges())
	rep := reporter.NewFluxStatusReporter(repClient, fluxcdv1.DefaultInstanceName, h.statusManager, h.namespace)
	computeResult, err := rep.Compute(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "report computed with errors")
	}

	// Set the operator info
	computeResult.Spec.Operator = &fluxcdv1.OperatorInfo{
		APIVersion: fluxcdv1.GroupVersion.String(),
		Version:    h.version,
		Platform:   fmt.Sprintf("%s/%s", goruntime.GOOS, goruntime.GOARCH),
	}

	// Build the FluxReport object
	obj := &fluxcdv1.FluxReport{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fluxcdv1.GroupVersion.String(),
			Kind:       fluxcdv1.FluxReportKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluxcdv1.DefaultInstanceName,
			Namespace: h.namespace,
		},
		Spec: computeResult.Spec,
	}

	// Convert to unstructured
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to convert report to unstructured: %w", err)
	}
	report := &unstructured.Unstructured{Object: rawMap}

	// Enrich the report with the Flux controllers usage.
	if metrics := h.controllerMetrics(ctx); len(metrics) > 0 {
		if spec, found := report.Object["spec"].(map[string]any); found {
			spec["metrics"] = metrics
		}
	}

	// Build the nodes snapshot and enrich the report with its summary.
	var serverVersion string
	if computeResult.Spec.Cluster != nil {
		serverVersion = computeResult.Spec.Cluster.ServerVersion
	}
	// The bursting workloads are matched against the Flux-managed workloads
	// of the same report computation.
	nodes, err := h.buildNodesSnapshot(ctx, serverVersion, computeResult.Workloads)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to build the nodes snapshot")
	} else if spec, found := report.Object["spec"].(map[string]any); found {
		spec["nodes"] = nodes.summary
	}

	return report, computeResult, nodes, nil
}

// ControllerMetrics holds the current CPU/Memory usage of a Flux
// controller pod along with the resource requests and limits summed
// from its spec (zero means not set).
type ControllerMetrics struct {
	// Pod is the name of the controller pod.
	Pod string `json:"pod"`

	// Namespace is the namespace of the controller pod.
	Namespace string `json:"namespace"`

	// CPU is the current usage in cores.
	CPU float64 `json:"cpu"`

	// Memory is the current usage in bytes.
	Memory int64 `json:"memory"`

	// CPURequests and CPULimits are expressed in cores.
	CPURequests float64 `json:"cpuRequests"`
	CPULimits   float64 `json:"cpuLimits"`

	// MemoryRequests and MemoryLimits are expressed in bytes.
	MemoryRequests int64 `json:"memoryRequests"`
	MemoryLimits   int64 `json:"memoryLimits"`
}

// controllerMetrics returns the current usage of the Flux controller pods
// in the operator namespace, read from the pod metrics collector with the
// requests/limits taken from the pod specs. When the pod list fails, the
// usage of the buffered controller pods is returned without the
// requests/limits. Returns nil when pod metrics collection is disabled
// or the Metrics API is unavailable.
func (h *Handler) controllerMetrics(ctx context.Context) []ControllerMetrics {
	if h.metrics == nil || !h.metrics.Available() {
		return nil
	}

	fluxSelector := labels.Set{"app.kubernetes.io/part-of": "flux"}

	var result []ControllerMetrics
	var podList corev1.PodList
	if err := h.kubeClient.GetClient(ctx, kubeclient.WithPrivileges()).List(ctx, &podList,
		client.InNamespace(h.namespace),
		client.MatchingLabelsSelector{Selector: fluxSelector.AsSelector()}); err != nil {
		log.FromContext(ctx).Error(err, "failed to list Flux controller pods for metrics")
		for _, name := range h.metrics.LabeledPods(h.namespace, fluxSelector.AsSelector()) {
			if latest := h.currentPodMetrics(h.namespace, name); latest != nil {
				result = append(result, ControllerMetrics{
					Pod:       name,
					Namespace: h.namespace,
					CPU:       latest.CPU,
					Memory:    latest.Memory,
				})
			}
		}
	} else {
		missing := false
		for i := range podList.Items {
			pod := &podList.Items[i]
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}
			latest := h.currentPodMetrics(pod.Namespace, pod.Name)
			if latest == nil {
				missing = true
				continue
			}
			res := sumPodResources(pod)
			result = append(result, ControllerMetrics{
				Pod:            pod.Name,
				Namespace:      pod.Namespace,
				CPU:            latest.CPU,
				Memory:         latest.Memory,
				CPURequests:    res.CPURequests,
				CPULimits:      res.CPULimits,
				MemoryRequests: res.MemoryRequests,
				MemoryLimits:   res.MemoryLimits,
			})
		}
		// Freshly rolled-out pods have no usage sample yet: request a
		// catch-up scrape so the next report refresh picks them up.
		if missing {
			h.metrics.RequestScrape()
		}
	}

	slices.SortFunc(result, func(a, b ControllerMetrics) int {
		return strings.Compare(a.Pod, b.Pod)
	})
	return result
}

func uninitialisedReport() *unstructured.Unstructured {
	obj := &fluxcdv1.FluxReport{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fluxcdv1.GroupVersion.String(),
			Kind:       fluxcdv1.FluxReportKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
		},
		Spec: fluxcdv1.FluxReportSpec{
			Distribution: fluxcdv1.FluxDistributionStatus{
				Entitlement: "Unknown",
				Status:      "Unknown",
			},
		},
	}

	rawMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	return &unstructured.Unstructured{Object: rawMap}
}
