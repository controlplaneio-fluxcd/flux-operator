// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

const (
	// defaultLogTailLines is the number of log lines returned when the
	// tailLines query parameter is not set.
	defaultLogTailLines int64 = 1000

	// maxLogTailLines caps the number of log lines a client can request.
	maxLogTailLines int64 = 5000

	// maxLogBytes caps the size of the log payload returned to the client.
	maxLogBytes int64 = 256 * 1024 // 256 KiB
)

// WorkloadLogsResponse represents the response body for GET /api/v1/workload/logs.
type WorkloadLogsResponse struct {
	// Pod is the name of the pod the logs belong to.
	Pod string `json:"pod"`

	// Container is the name of the container the logs belong to.
	Container string `json:"container"`

	// Logs is the plain-text log output of the container.
	Logs string `json:"logs"`
}

// trimPartialLogLine drops a trailing partial log line from the payload.
//
// Container runtimes newline-terminate every emitted log line, so a payload
// that does not end with a newline has been truncated mid-line, either because
// the container was writing the line when the logs were read (common while
// following) or because the LimitBytes cap cut the newest line. Returning that
// fragment as the latest entry shows only a few characters, so it is dropped;
// the complete line reappears on the next fetch. A payload with no newline at
// all is returned unchanged so a single short line is not lost.
func trimPartialLogLine(logs string) string {
	if logs == "" || logs[len(logs)-1] == '\n' {
		return logs
	}
	if i := strings.LastIndexByte(logs, '\n'); i >= 0 {
		return logs[:i+1]
	}
	return logs
}

// WorkloadLogsHandler handles GET /api/v1/workload/logs requests and returns the
// logs of a pod container managed by a Flux workload.
//
// Logs are read using the impersonated user's identity, so Kubernetes enforces
// the native "get" verb on the "pods/log" subresource: a user can only view the
// logs of pods they are explicitly granted access to. This is a read-only
// endpoint and is therefore not gated by UserActionsEnabled; access is governed
// entirely by RBAC.
//
// Example: /api/v1/workload/logs?namespace=flux-system&name=source-controller-xx-yy&container=manager
func (h *Handler) WorkloadLogsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters.
	namespace := req.URL.Query().Get("namespace")
	name := req.URL.Query().Get("name")
	container := req.URL.Query().Get("container")

	// Validate required fields.
	if namespace == "" || name == "" {
		http.Error(w, "Missing required query parameters: namespace, name", http.StatusBadRequest)
		return
	}

	// Parse the optional tailLines parameter, clamping it to a sane range.
	tailLines := defaultLogTailLines
	if v := req.URL.Query().Get("tailLines"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil || parsed <= 0 {
			http.Error(w, "Invalid tailLines parameter", http.StatusBadRequest)
			return
		}
		tailLines = min(parsed, maxLogTailLines)
	}

	// Parse the optional previous parameter to fetch logs from the
	// previous container instance (e.g. after a crash/restart).
	var previous bool
	if v := req.URL.Query().Get("previous"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			http.Error(w, "Invalid previous parameter", http.StatusBadRequest)
			return
		}
		previous = parsed
	}

	ctx := req.Context()

	// Build a typed clientset using the impersonated user's REST config so
	// that Kubernetes enforces the user's RBAC on the pods/log subresource.
	// The controller-runtime client cannot read the logs subresource, hence
	// the dedicated clientset (see GetMetrics for the same pattern).
	clientset, err := kubernetes.NewForConfig(h.kubeClient.GetConfig(ctx))
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to create clientset for pod logs",
			"pod", name, "namespace", namespace)
		http.Error(w, "Unable to read pod logs", http.StatusInternalServerError)
		return
	}

	logBytes := maxLogBytes
	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
		Container:  container,
		TailLines:  &tailLines,
		LimitBytes: &logBytes,
		Previous:   previous,
		Timestamps: true,
	}).Stream(ctx)
	if err != nil {
		switch {
		case errors.IsForbidden(err):
			perms := user.Permissions(ctx)
			http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to read logs for pod %s/%s",
				perms.Username, namespace, name), http.StatusForbidden)
		case errors.IsNotFound(err):
			http.Error(w, fmt.Sprintf("Pod %s/%s not found", namespace, name), http.StatusNotFound)
		default:
			log.FromContext(ctx).Error(err, "failed to stream pod logs",
				"pod", name, "namespace", namespace, "container", container)
			http.Error(w, fmt.Sprintf("Failed to read logs: %v", err), http.StatusInternalServerError)
		}
		return
	}
	defer stream.Close()

	// Read the log stream, enforcing the byte cap as a safety net in case
	// LimitBytes is not honored by the API server.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(stream, maxLogBytes)); err != nil {
		log.FromContext(ctx).Error(err, "failed to read pod logs stream",
			"pod", name, "namespace", namespace, "container", container)
		http.Error(w, "Failed to read logs", http.StatusInternalServerError)
		return
	}

	// Return the logs as a JSON object so the frontend fetch utility can
	// decode it consistently with the other API endpoints.
	w.Header().Set("Content-Type", "application/json")
	resp := WorkloadLogsResponse{
		Pod:       name,
		Container: container,
		Logs:      trimPartialLogLine(buf.String()),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
