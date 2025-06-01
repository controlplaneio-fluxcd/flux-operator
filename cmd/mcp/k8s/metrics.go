// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// GetMetrics retrieves the CPU and Memory metrics for a list of pods in the given namespace.
func (k *Client) GetMetrics(ctx context.Context, pod, namespace, labelSelector string, limit int) (*unstructured.Unstructured, error) {
	clientset, err := metricsclientset.NewForConfig(k.cfg)
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

	metrics := make([]map[string]interface{}, 0, len(versionedMetrics.Items))
	for _, item := range versionedMetrics.Items {
		for _, container := range item.Containers {
			metrics = append(metrics, map[string]interface{}{
				"pod":       item.Name,
				"namespace": item.Namespace,
				"container": container.Name,
				"cpuUsage":  container.Usage[corev1.ResourceCPU],
				"memUsage":  container.Usage[corev1.ResourceMemory],
			})
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PodMetricsList",
			"metadata": map[string]interface{}{
				"name":      pod,
				"namespace": namespace,
			},
			"items": metrics,
		},
	}, nil
}
