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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
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
	if cached := h.getCachedReport(); cached != nil {
		report = cached
	} else {
		if r, err := h.buildReport(ctx); err != nil {
			return nil, err
		} else {
			report = r
		}
	}

	// Inject user info
	if spec, found := report.Object["spec"].(map[string]any); found {
		username, role := user.UsernameAndRole(ctx)
		userInfo := make(map[string]any)
		if username != "" {
			userInfo["username"] = username
		}
		if role != "" {
			userInfo["role"] = role
		}
		spec["userInfo"] = userInfo

		// Inject user-visible namespaces (non-fatal if it fails)
		namespaces, _, err := h.kubeClient.ListUserNamespaces(ctx)
		switch {
		case err != nil:
			log.FromContext(ctx).Error(err, "failed to list user namespaces for report injection")
		case len(namespaces) > 0:
			spec["namespaces"] = namespaces
		}
	}

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

// refreshReportCache builds a fresh report and updates the cache.
func (h *Handler) refreshReportCache(ctx context.Context) {
	report, err := h.buildReport(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) || ctx.Err() == nil {
			log.FromContext(ctx).Error(err, "failed to refresh report cache")
		}
		return
	}

	h.reportCacheMu.Lock()
	h.reportCache = report
	h.reportCacheMu.Unlock()
}

// getCachedReport returns the cached report if available.
func (h *Handler) getCachedReport() *unstructured.Unstructured {
	h.reportCacheMu.RLock()
	if h.reportCache == nil {
		h.reportCacheMu.RUnlock()
		return nil
	}
	b, _ := json.Marshal(h.reportCache)
	h.reportCacheMu.RUnlock()

	var obj unstructured.Unstructured
	_ = json.Unmarshal(b, &obj)
	return &obj
}

// buildReport builds the FluxReport directly using the reporter package
// and injects pod metrics into the report spec.
func (h *Handler) buildReport(ctx context.Context) (*unstructured.Unstructured, error) {
	// The report client needs privileged access as it needs to access all
	// resources in the cluster to build the report. The report information,
	// however, is crunched in a way that does not expose sensitive information.
	// This allows us to keep a good UX for unprivileged users while
	// ensuring security boundaries are respected.
	// Note: The report is built on a background goroutine periodically anyway,
	// so there's no user session available to use for impersonation.
	repClient := h.kubeClient.GetClient(ctx, kubeclient.WithPrivileges())
	rep := reporter.NewFluxStatusReporter(repClient, fluxcdv1.DefaultInstanceName, h.statusManager, h.namespace)
	reportSpec, err := rep.Compute(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "report computed with errors")
	}

	// Set the operator info
	reportSpec.Operator = &fluxcdv1.OperatorInfo{
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
		Spec: reportSpec,
	}

	// Convert to unstructured
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert report to unstructured: %w", err)
	}
	result := &unstructured.Unstructured{Object: rawMap}

	// Fetch metrics for Flux components (non-fatal if it fails)
	// We pass WithPrivileges() here to ensure we can read metrics from all Flux controllers,
	// even if the user querying the report has limited RBAC permissions. Our decision here
	// is based on the same reasoning explained above for building the report.
	if metrics, err := h.GetMetrics(ctx, "", h.namespace, "app.kubernetes.io/part-of=flux", 100, kubeclient.WithPrivileges()); err == nil {
		// Extract the items array from metrics
		if items, found := metrics.Object["items"]; found {
			// Add metrics to the result under spec.metrics
			if spec, found := result.Object["spec"].(map[string]any); found {
				spec["metrics"] = items
			}
		}
	}

	return result, nil
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

// cleanObjectForExport removes fields that shouldn't be included in exports
func cleanObjectForExport(obj *unstructured.Unstructured, keepStatus bool) {
	// Remove status subresource
	if !keepStatus {
		unstructured.RemoveNestedField(obj.Object, "status")
	}

	// Remove runtime metadata - keep only name, namespace, labels, and annotations
	metadata := obj.Object["metadata"].(map[string]any)
	cleanMetadata := make(map[string]any)

	// Preserve essential fields
	if name, exists := metadata["name"]; exists {
		cleanMetadata["name"] = name
	}
	if namespace, exists := metadata["namespace"]; exists {
		cleanMetadata["namespace"] = namespace
	}
	if lb, exists := metadata["labels"]; exists {
		cleanMetadata["labels"] = lb
	}

	if annotations, exists := metadata["annotations"]; exists {
		cleanMetadata["annotations"] = annotations
	}

	// Remove Flux ownership labels
	if lb, exists := cleanMetadata["labels"]; exists {
		if labelMap, ok := lb.(map[string]any); ok {
			for key := range labelMap {
				if fluxcdv1.IsFluxAPI(key) &&
					(strings.HasSuffix(key, "/name") || strings.HasSuffix(key, "/namespace")) {
					delete(labelMap, key)
				}
			}
			// Remove labels map if empty after cleanup
			if len(labelMap) == 0 {
				delete(cleanMetadata, "labels")
			}
		}
	}

	// Remove kubectl and Flux CLI annotations from clean metadata
	if annotations, exists := cleanMetadata["annotations"]; exists {
		if annotationMap, ok := annotations.(map[string]any); ok {
			delete(annotationMap, "kubectl.kubernetes.io/last-applied-configuration")
			delete(annotationMap, "reconcile.fluxcd.io/requestedAt")
			delete(annotationMap, "reconcile.fluxcd.io/forceAt")
			// Remove annotations map if empty after cleanup
			if len(annotationMap) == 0 {
				delete(cleanMetadata, "annotations")
			}
		}
	}

	// Replace metadata with the clean version
	obj.Object["metadata"] = cleanMetadata
}

// GetMetrics retrieves the CPU and Memory metrics for a list of pods in the given namespace.
func (h *Handler) GetMetrics(ctx context.Context, pod, namespace, labelSelector string, limit int, opts ...kubeclient.Option) (*unstructured.Unstructured, error) {
	clientset, err := metricsclientset.NewForConfig(h.kubeClient.GetConfig(ctx, opts...))
	if err != nil {
		return nil, err
	}

	ls := labels.Everything()
	if len(labelSelector) > 0 {
		ls, err = labels.Parse(labelSelector)
		if err != nil {
			return nil, err
		}
	}

	versionedMetrics := &metricsv1beta1api.PodMetricsList{}
	if pod != "" {
		m, err := clientset.MetricsV1beta1().
			PodMetricses(namespace).
			Get(ctx, pod, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		versionedMetrics.Items = []metricsv1beta1api.PodMetrics{*m}
	} else {
		versionedMetrics, err = clientset.MetricsV1beta1().
			PodMetricses(namespace).
			List(ctx, metav1.ListOptions{LabelSelector: ls.String(), Limit: int64(limit)})
		if err != nil {
			return nil, err
		}
	}

	if len(versionedMetrics.Items) == 0 {
		return nil, fmt.Errorf("no metrics found for pods in namespace %s", namespace)
	}

	// Fetch pod specs to get resource limits
	kubeClient := h.kubeClient.GetClient(ctx, opts...)
	var podList corev1.PodList
	if pod != "" {
		p := &corev1.Pod{}
		if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: pod}, p); err == nil {
			podList.Items = []corev1.Pod{*p}
		}
	} else {
		listOpts := []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabelsSelector{Selector: ls},
			client.Limit(int64(limit)),
		}
		if err := kubeClient.List(ctx, &podList, listOpts...); err != nil {
			// Non-fatal: continue without limits if pod list fails
			podList.Items = nil
		}
	}

	// Create a map for quick pod lookup
	podMap := make(map[string]*corev1.Pod)
	for i := range podList.Items {
		podMap[podList.Items[i].Name] = &podList.Items[i]
	}

	metrics := make([]map[string]any, 0, len(versionedMetrics.Items))
	for _, item := range versionedMetrics.Items {
		for _, container := range item.Containers {
			if len(container.Usage) == 0 {
				continue
			}
			memQuantity, ok := container.Usage[corev1.ResourceMemory]
			if !ok || memQuantity.IsZero() {
				continue
			}

			cpuQuantity := container.Usage[corev1.ResourceCPU]

			// Convert CPU from millicores to cores (float) for easier UI parsing
			cpuCores := float64(cpuQuantity.MilliValue()) / 1000.0

			// Convert Memory to bytes (int64) for easier UI parsing
			memBytes := memQuantity.Value()

			metricEntry := map[string]any{
				"pod":       item.Name,
				"namespace": item.Namespace,
				"container": container.Name,
				"cpu":       cpuCores,
				"memory":    memBytes,
			}

			// Default limits: 2x actual usage (fallback if no limits are set)
			cpuLimit := cpuCores * 2.0
			memLimit := memBytes * 2

			// Try to get actual limits from pod spec
			if podSpec, found := podMap[item.Name]; found {
				for _, c := range podSpec.Spec.Containers {
					if c.Name == container.Name {
						if limit, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
							cpuLimit = float64(limit.MilliValue()) / 1000.0
						}
						if limit, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
							memLimit = limit.Value()
						}
						break
					}
				}
			}

			metricEntry["cpuLimit"] = cpuLimit
			metricEntry["memoryLimit"] = memLimit

			metrics = append(metrics, metricEntry)
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"items": metrics,
		},
	}, nil
}
