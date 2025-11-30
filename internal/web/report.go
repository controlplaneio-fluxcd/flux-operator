// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	goruntime "runtime"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// ReportHandler handles GET /api/v1/report requests and returns the FluxReport from the cluster.
func (r *Router) ReportHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the FluxReport from the cluster using the request context
	report, err := r.GetReport(req.Context())
	if err != nil {
		r.log.Error(err, "cluster query failed", "url", req.URL.String())
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
func (r *Router) GetReport(ctx context.Context) (*unstructured.Unstructured, error) {
	report := &unstructured.Unstructured{}
	if cached := r.getCachedReport(); cached != nil {
		report = cached
	} else {
		if r, err := r.buildReport(ctx); err != nil {
			return nil, err
		} else {
			report = r
		}
	}

	// Inject user info
	// TODO: Replace with real user info from auth context when available
	if spec, found := report.Object["spec"].(map[string]any); found {
		username := os.Getenv("HOSTNAME")
		if username == "" {
			username = "flux-user"
		}
		spec["userInfo"] = map[string]any{
			"username": username,
			"role":     "cluster:view",
		}
	}

	return report, nil
}

// buildReport builds the FluxReport directly using the reporter package
// and injects pod metrics into the report spec.
func (r *Router) buildReport(ctx context.Context) (*unstructured.Unstructured, error) {
	rep := reporter.NewFluxStatusReporter(r.kubeClient, fluxcdv1.DefaultInstanceName, r.statusManager, r.namespace)
	reportSpec, err := rep.Compute(ctx)
	if err != nil {
		r.log.Error(err, "report computed with errors")
	}

	// Set the operator info
	reportSpec.Operator = &fluxcdv1.OperatorInfo{
		APIVersion: fluxcdv1.GroupVersion.String(),
		Version:    r.version,
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
			Namespace: r.namespace,
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
	if metrics, err := r.GetMetrics(ctx, "", r.namespace, "app.kubernetes.io/part-of=flux", 100); err == nil {
		// Extract the items array from metrics
		if items, found := metrics.Object["items"]; found {
			// Add metrics to the result under spec.metrics
			if spec, found := result.Object["spec"].(map[string]any); found {
				spec["metrics"] = items
			}
		}
	}

	// Fetch namespaces from the cluster (non-fatal if it fails)
	if namespaces, err := r.GetNamespaces(ctx); err == nil {
		// Extract the items array from namespaces
		if items, found := namespaces.Object["items"]; found {
			// Add namespaces to the result under spec.namespaces
			if spec, found := result.Object["spec"].(map[string]any); found {
				spec["namespaces"] = items
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

// GetNamespaces retrieves all namespace names from the cluster sorted alphabetically.
func (r *Router) GetNamespaces(ctx context.Context) (*unstructured.Unstructured, error) {
	var namespaceList corev1.NamespaceList
	if err := r.kubeClient.List(ctx, &namespaceList); err != nil {
		return nil, err
	}

	if len(namespaceList.Items) == 0 {
		return nil, fmt.Errorf("no namespaces found")
	}

	names := make([]string, 0, len(namespaceList.Items))
	for _, ns := range namespaceList.Items {
		names = append(names, ns.Name)
	}

	sort.Strings(names)

	return &unstructured.Unstructured{
		Object: map[string]any{
			"items": names,
		},
	}, nil
}

// GetMetrics retrieves the CPU and Memory metrics for a list of pods in the given namespace.
func (r *Router) GetMetrics(ctx context.Context, pod, namespace, labelSelector string, limit int) (*unstructured.Unstructured, error) {
	clientset, err := metricsclientset.NewForConfig(r.kubeConfig)
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
	var podList corev1.PodList
	if pod != "" {
		p := &corev1.Pod{}
		if err := r.kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: pod}, p); err == nil {
			podList.Items = []corev1.Pod{*p}
		}
	} else {
		listOpts := []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabelsSelector{Selector: ls},
			client.Limit(int64(limit)),
		}
		if err := r.kubeClient.List(ctx, &podList, listOpts...); err != nil {
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
