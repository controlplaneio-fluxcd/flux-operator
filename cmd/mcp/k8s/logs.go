// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

// GetLogs retrieves the logs for a specific pod container in the given namespace.
func (k *Client) GetLogs(ctx context.Context, pod, container, namespace string, limit int64) (*unstructured.Unstructured, error) {
	limitBytes := int64(64 * 1024)
	podLogOpts := corev1.PodLogOptions{
		Container:  container,
		TailLines:  &limit,
		LimitBytes: &limitBytes,
	}

	clientset, err := kubernetes.NewForConfig(k.cfg)
	if err != nil {
		return nil, err
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(pod, &podLogOpts)
	logs, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get logs for pod %s/%s: %w", namespace, pod, err)
	}
	defer logs.Close()

	logsBuffer := new(bytes.Buffer)
	_, err = io.Copy(logsBuffer, logs)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs for pod %s/%s: %w", namespace, pod, err)
	}

	logsContent := logsBuffer.String()

	if logsContent == "" {
		logsContent = fmt.Sprintf("no logs found for container %s", container)
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      pod,
				"namespace": namespace,
			},
			"container": container,
			"logs":      logsContent,
		},
	}, nil
}
