// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	maxLogBytes int64 = 512 * 1024 // 512 KiB
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
// that does not end with a newline has been truncated mid-line because the
// container was writing the line when the logs were read (common while
// following). Returning that fragment as the latest entry shows only a few
// characters, so it is dropped; the complete line reappears on the next fetch.
// A payload with no newline at all is returned unchanged so a single short line
// is not lost.
func trimPartialLogLine(logs string) string {
	if logs == "" || logs[len(logs)-1] == '\n' {
		return logs
	}
	if i := strings.LastIndexByte(logs, '\n'); i >= 0 {
		return logs[:i+1]
	}
	return logs
}

// tailLogBytes reads r to EOF but retains only the most recent limit bytes, so
// the response stays small while returning the newest log lines.
//
// The kubelet's PodLogOptions.LimitBytes keeps the *oldest* bytes of the range
// it reads, which for a tail window drops the most recent lines. Reading the
// whole window (bounded by TailLines) and keeping the trailing bytes here
// returns the newest lines instead. Memory use is bounded to ~limit regardless
// of the stream length.
//
// The returned bool is partialFirst: true when the retained slice begins
// mid-line (the byte dropped just before it was not a newline), so the caller
// should drop that leading fragment. A cut that lands exactly on a line boundary
// keeps a complete first line and reports false.
func tailLogBytes(r io.Reader, limit int) ([]byte, bool, error) {
	const chunkSize = 32 * 1024
	buf := make([]byte, 0, limit+chunkSize)
	chunk := make([]byte, chunkSize)
	partialFirst := false
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if len(buf) > limit {
				// The byte dropped just before the retained window decides
				// whether the first kept line is complete (preceded by '\n') or
				// a mid-line fragment.
				partialFirst = buf[len(buf)-limit-1] != '\n'
				// Shift the trailing limit bytes (the newest output) to the
				// front in place so the backing array, and thus memory, stays
				// bounded.
				copy(buf, buf[len(buf)-limit:])
				buf = buf[:limit]
			}
		}
		if err == io.EOF {
			return buf, partialFirst, nil
		}
		if err != nil {
			return nil, partialFirst, err
		}
	}
}

// trimPartialFirstLine drops a leading partial log line from the payload. It is
// applied only when the payload was byte-truncated from the front (see
// tailLogBytes), where the first line is a mid-line fragment. A payload with no
// newline is returned unchanged so a single short line is not lost.
func trimPartialFirstLine(logs string) string {
	if _, rest, found := strings.Cut(logs, "\n"); found {
		return rest
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
// By default the last tailLines entries are returned. When the sinceTime query
// parameter is set (an RFC3339 timestamp), only entries newer than that time are
// returned instead, so a following client can incrementally append new lines
// rather than re-fetching and replacing the whole tail window on every poll.
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

	// Parse the optional sinceTime parameter (RFC3339). When set, only entries
	// newer than this time are returned, letting a following client append new
	// lines instead of replacing the whole tail on every poll.
	var sinceTime *metav1.Time
	if v := req.URL.Query().Get("sinceTime"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			http.Error(w, "Invalid sinceTime parameter", http.StatusBadRequest)
			return
		}
		t := metav1.NewTime(parsed)
		sinceTime = &t
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

	// TailLines is always set so the kubelet returns the newest lines (and
	// bounds the read to at most tailLines lines). When following, SinceTime
	// additionally restricts the range to entries after the last seen line, so
	// the response is the newest lines since then. LimitBytes is deliberately
	// not used: it keeps the oldest bytes of the range, dropping the most recent
	// lines; the byte cap is enforced below by keeping the trailing bytes.
	opts := &corev1.PodLogOptions{
		Container:  container,
		TailLines:  &tailLines,
		Previous:   previous,
		Timestamps: true,
	}
	if sinceTime != nil {
		opts.SinceTime = sinceTime
	}
	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(name, opts).Stream(ctx)
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

	// Read the stream, keeping only the newest maxLogBytes so the payload stays
	// bounded while returning the most recent lines.
	data, partialFirst, err := tailLogBytes(stream, int(maxLogBytes))
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to read pod logs stream",
			"pod", name, "namespace", namespace, "container", container)
		http.Error(w, "Failed to read logs", http.StatusInternalServerError)
		return
	}

	// Drop the partial first line only when the byte cap cut mid-line (a cut on
	// a line boundary keeps a complete first line), and the partial last line
	// when the container was mid-write.
	logs := trimPartialLogLine(string(data))
	if partialFirst {
		logs = trimPartialFirstLine(logs)
	}

	// Return the logs as a JSON object so the frontend fetch utility can
	// decode it consistently with the other API endpoints.
	w.Header().Set("Content-Type", "application/json")
	resp := WorkloadLogsResponse{
		Pod:       name,
		Container: container,
		Logs:      logs,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
