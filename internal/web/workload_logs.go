// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/podlogs"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

const (
	// defaultLogTailLines is the number of log lines returned when the
	// tailLines query parameter is not set.
	defaultLogTailLines = 1000

	// maxLogTailLines caps the number of log lines a client can request.
	maxLogTailLines = 5000

	// maxLogBytes caps the size of the log payload returned to the client.
	maxLogBytes int64 = 512 * 1024 // 512 KiB

	// maxLogContainers caps the number of containers streamed per pod for the
	// all-containers view. Real pods have only a handful of containers; this is a
	// defensive ceiling for a request that names many directly.
	maxLogContainers = 50

	// maxLogPods caps the number of pods streamed for the all-pods view, bounding
	// the pod dimension of the fan-out independently of the container dimension.
	maxLogPods = 100

	// maxLogStreams caps the total number of concurrent log streams (pods ×
	// containers) a single request can open, bounding the overall fan-out.
	maxLogStreams = 50

	// maxLogStreamConcurrency caps how many of a request's streams are read at
	// once, so the all-pods view does not open every stream simultaneously.
	maxLogStreamConcurrency = 10

	// minPerStreamLogBytes floors the per-stream byte budget when the byte cap is
	// divided across a multi-stream fan-out, so each stream still returns a useful
	// tail even when many streams share the overall maxLogBytes budget.
	minPerStreamLogBytes = 64 * 1024 // 64 KiB
)

// WorkloadLogsResponse represents the response body for GET /api/v1/workload/logs.
type WorkloadLogsResponse struct {
	// Pod is the name of the pod the logs belong to (or a comma-joined list of
	// pods for the all-pods view).
	Pod string `json:"pod"`

	// Container is the name of the container the logs belong to (or a comma-joined
	// list for the all-containers view).
	Container string `json:"container"`

	// Logs is the plain-text log output.
	Logs string `json:"logs"`

	// Tagged reports whether each timestamped line in Logs is prefixed with its
	// pod of origin ("<pod> <timestamp> <message>"). Set only for the all-pods
	// view (more than one pod streamed); absent for single-pod responses.
	Tagged bool `json:"tagged,omitempty"`

	// ContainerTagged reports whether each timestamped line in Logs is prefixed
	// with its container of origin, after the optional pod tag ("<pod> <container>
	// <timestamp> <message>", or "<container> <timestamp> <message>" for a single
	// pod). Set only for the all-containers view (more than one container
	// streamed), so the client can scope multi-line folding per container.
	ContainerTagged bool `json:"containerTagged,omitempty"`

	// Total is the number of distinct pods the client requested. Set only for the
	// multi-stream (all-pods/all-containers) path.
	Total int `json:"total,omitempty"`

	// Streamed is the number of those pods that produced logs. When it is less
	// than Total some pods were skipped (forbidden, missing, or capped).
	Streamed int `json:"streamed,omitempty"`

	// Partial reports that the response does not cover every requested pod
	// (Streamed < Total) or that the fan-out was truncated by a cap.
	Partial bool `json:"partial,omitempty"`

	// Forbidden is the number of pods skipped because the user is not allowed to
	// read their logs, so the UI can explain a partial result.
	Forbidden int `json:"forbidden,omitempty"`
}

// writeLogStreamError maps a GetLogs stream error to an HTTP response: 403 for a
// forbidden user, 404 for a missing pod, and 500 otherwise (logged). An empty
// name produces a workload-scoped message for the all-pods fan-out, where the
// failure is not attributable to one named pod.
func writeLogStreamError(ctx context.Context, w http.ResponseWriter, err error, namespace, name, container string) {
	switch {
	case errors.IsForbidden(err):
		perms := user.Permissions(ctx)
		if name == "" {
			http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to read the workload pod logs in namespace %s",
				perms.Username, namespace), http.StatusForbidden)
			return
		}
		http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to read logs for pod %s/%s",
			perms.Username, namespace, name), http.StatusForbidden)
	case errors.IsNotFound(err):
		if name == "" {
			http.Error(w, fmt.Sprintf("No readable pods found in namespace %s", namespace), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Pod %s/%s not found", namespace, name), http.StatusNotFound)
	default:
		log.FromContext(ctx).Error(err, "failed to stream pod logs",
			"pod", name, "namespace", namespace, "container", container)
		http.Error(w, fmt.Sprintf("Failed to read logs: %v", err), http.StatusInternalServerError)
	}
}

// writeWorkloadLogs encodes a single-stream log payload as the JSON
// WorkloadLogsResponse, leaving the multi-stream metadata fields unset.
func writeWorkloadLogs(w http.ResponseWriter, pod, container, logs string) {
	writeWorkloadLogsResponse(w, WorkloadLogsResponse{
		Pod:       pod,
		Container: container,
		Logs:      logs,
	})
}

// writeWorkloadLogsResponse encodes the given response as JSON.
func writeWorkloadLogsResponse(w http.ResponseWriter, resp WorkloadLogsResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// logFanOut is the outcome of a per-target log fan-out: the successful streams
// (tagged with their pod), the set of distinct pods that produced logs, the
// number of distinct pods skipped because the user is forbidden to read them, and
// the first error encountered. It implements the best-effort policy of the
// all-pods/all-containers view: a target that failed (e.g. waiting to start,
// deleted, or forbidden) is skipped as long as another succeeded, so firstErr is
// only meaningful when no stream was collected.
type logFanOut struct {
	streams     []podlogs.LogStream
	streamedSet map[string]struct{}
	forbidden   int
	firstErr    error
}

// collectLogStreams partitions the per-target fan-out results into the
// successful streams, the distinct pods that produced logs, the count of distinct
// pods forbidden, and the first error. targets, logs, and errs are index-aligned.
// Forbidden is counted per pod (not per target) so a forbidden pod with several
// containers is reported once, matching the pod-based Total/Streamed fields.
func collectLogStreams(targets []podlogs.LogTarget, logs []string, errs []error) logFanOut {
	out := logFanOut{streamedSet: make(map[string]struct{})}
	forbiddenPods := make(map[string]struct{})
	for i := range targets {
		if errs[i] != nil {
			if out.firstErr == nil {
				out.firstErr = errs[i]
			}
			if errors.IsForbidden(errs[i]) {
				forbiddenPods[targets[i].Pod] = struct{}{}
			}
			continue
		}
		out.streams = append(out.streams, podlogs.LogStream{Pod: targets[i].Pod, Container: targets[i].Container, Blob: logs[i]})
		out.streamedSet[targets[i].Pod] = struct{}{}
	}
	out.forbidden = len(forbiddenPods)
	return out
}

// WorkloadLogsHandler handles GET /api/v1/workload/logs requests and returns the
// logs of a pod container managed by a Flux workload.
//
// Logs are only available when user actions are enabled. They are read using
// the impersonated user's identity, so Kubernetes enforces the native "get"
// verb on the "pods/log" subresource: a user can only view the logs of pods
// they are explicitly granted access to.
//
// By default the last tailLines entries are returned. When the sinceTime query
// parameter is set (an RFC3339 timestamp), only entries newer than that time are
// returned instead, so a following client can incrementally append new lines
// rather than re-fetching and replacing the whole tail window on every poll.
//
// The container query parameter may be repeated for the "All containers" view and
// the pod query parameter (in addition to the required name) for the "All pods"
// view: every (pod, container) target is streamed and interleaved chronologically
// into a single payload (the previous-instance option is not offered there). When
// more than one pod is requested, each timestamped line is prefixed with its pod
// of origin ("<pod> <timestamp> <message>") and the response sets tagged=true plus
// the total/streamed/partial/forbidden coverage fields. The frontend supplies the
// pod and container names, so no pod read is required and access stays governed
// solely by the pods/log RBAC, enforced per pod; a user who cannot read some pods
// simply gets a partial result.
//
// Following uses sinceTime for the single-stream path and a repeated since
// parameter ("<pod>=<rfc3339>") for the all-pods path, so each pod advances its
// own cursor independently of clock skew between nodes.
//
// Example: /api/v1/workload/logs?namespace=flux-system&name=source-controller-xx-yy&container=manager
func (h *Handler) WorkloadLogsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if actions are enabled before selecting a Kubernetes REST config.
	if !h.conf.UserActionsEnabled() {
		http.Error(w, "User actions are disabled", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters.
	namespace := req.URL.Query().Get("namespace")
	name := req.URL.Query().Get("name")

	// Validate required fields.
	if namespace == "" || name == "" {
		http.Error(w, "Missing required query parameters: namespace, name", http.StatusBadRequest)
		return
	}

	// Parse the optional tailLines parameter, clamping it to a sane range.
	tailLines := defaultLogTailLines
	if v := req.URL.Query().Get("tailLines"); v != "" {
		parsed, err := strconv.Atoi(v)
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

	// Parse the optional sinceTime parameter (RFC3339) for the single-stream path.
	// When set, only entries newer than this time are returned, letting a
	// following client append new lines instead of replacing the whole tail.
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

	// Parse the optional per-pod follow cursors for the all-pods view. Each
	// repeated "since" parameter carries "<pod>=<rfc3339>", so a following client
	// advances each pod independently (pods can sit on nodes with skewed clocks,
	// where a single shared cursor would drop a lagging pod's lines).
	sinceByPod := make(map[string]*metav1.Time)
	for _, v := range req.URL.Query()["since"] {
		pod, ts, found := strings.Cut(v, "=")
		if !found || pod == "" || ts == "" {
			http.Error(w, "Invalid since parameter", http.StatusBadRequest)
			return
		}
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			http.Error(w, "Invalid since parameter", http.StatusBadRequest)
			return
		}
		t := metav1.NewTime(parsed)
		sinceByPod[pod] = &t
	}

	ctx := req.Context()

	// Build a typed clientset using the impersonated user's REST config so
	// that Kubernetes enforces the user's RBAC on the pods/log subresource.
	// The controller-runtime client cannot read the logs subresource, hence
	// the dedicated clientset (the metrics collector uses the same pattern).
	clientset, err := kubernetes.NewForConfig(h.kubeClient.GetConfig(ctx))
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to create clientset for pod logs",
			"pod", name, "namespace", namespace)
		http.Error(w, "Unable to read pod logs", http.StatusInternalServerError)
		return
	}

	// Collect the requested pods: the required primary pod (name) plus any repeated
	// "pod" params for the all-pods view. Empty values are dropped, the list is
	// de-duplicated, and the pod and container dimensions are capped independently
	// so neither can fan out without bound.
	pods := []string{name}
	for _, p := range req.URL.Query()["pod"] {
		if p != "" {
			pods = append(pods, p)
		}
	}
	// total is the number of distinct pods the client asked for, measured before
	// the cap so a truncated all-pods view can be reported as partial.
	deduped, _ := podlogs.DedupeNames(pods, len(pods))
	total := len(deduped)
	pods, podsTruncated := podlogs.DedupeNames(pods, maxLogPods)

	// Collect the requested container names. An absent container lets the kubelet
	// pick the pod's default container.
	var containers []string
	for _, c := range req.URL.Query()["container"] {
		if c != "" {
			containers = append(containers, c)
		}
	}
	containers, containersTruncated := podlogs.DedupeNames(containers, maxLogContainers)

	// Tagging is decided from the client's request, before the stream cap, so the
	// client and server agree on the wire format regardless of which streams
	// survive truncation. Each fanned-out dimension is tagged: the pod when more
	// than one pod is streamed, the container when more than one container is, so
	// the client can scope multi-line folding per (pod, container).
	tagPod := total > 1
	tagContainer := len(containers) > 1

	// Build the (pod, container) targets as the product of the two dimensions,
	// capped to maxLogStreams overall. An empty container list yields one target
	// per pod (kubelet default container).
	targets, streamsTruncated := podlogs.BuildLogTargets(pods, containers, maxLogStreams)
	truncated := podsTruncated || containersTruncated || streamsTruncated

	// TailLines is always set so the kubelet returns the newest lines (and bounds
	// the read to at most tailLines lines). SinceTime narrows a follow poll to
	// entries after the last seen line. LimitBytes is deliberately not used: it
	// keeps the oldest bytes of the range, dropping the most recent lines; the
	// byte cap is enforced by keeping the trailing bytes.
	//
	// tailLines64 widens the clamped line count for the kubelet API, which takes
	// a *int64; the value is bounded by maxLogTailLines so the conversion is safe.
	tailLines64 := int64(tailLines)

	// Single-stream path (one pod, one or no container, no pod tagging): preserves
	// the previous-instance and follow semantics unchanged. An all-pods request
	// (tagPod) always uses the fan-out path so it carries the tagged/partial
	// metadata even if a cap collapsed it to a single surviving stream. An
	// all-containers request (tagContainer) cannot reach here: it has >1 container,
	// so buildLogTargets yields min(len(containers), maxLogStreams) > 1 targets
	// (maxLogStreams is 50). Were a future cap to collapse it to one target, the
	// response would report ContainerTagged=false, so client and server still agree.
	if !tagPod && len(targets) <= 1 {
		t := podlogs.LogTarget{Pod: name}
		if len(targets) == 1 {
			t = targets[0]
		}
		opts := &corev1.PodLogOptions{
			Container:  t.Container,
			TailLines:  &tailLines64,
			Previous:   previous,
			Timestamps: true,
		}
		if sinceTime != nil {
			opts.SinceTime = sinceTime
		}
		logs, err := podlogs.FetchContainerLog(ctx, clientset, namespace, t.Pod, opts, int(maxLogBytes))
		if err != nil {
			writeLogStreamError(ctx, w, err, namespace, t.Pod, t.Container)
			return
		}
		writeWorkloadLogs(w, t.Pod, t.Container, logs)
		return
	}

	// Fan-out path (all-pods and/or all-containers): stream every target
	// concurrently (bounded by a semaphore) and merge by timestamp. Previous logs
	// are not offered here. The per-stream byte budget is the overall cap divided
	// across the streams (floored) so transient memory stays bounded. The fetch is
	// best-effort: a target with no readable logs yet (waiting, deleted, or
	// forbidden) is skipped as long as another succeeds, and an error is returned
	// only when every target fails.
	perStreamBytes := max(int(maxLogBytes)/len(targets), minPerStreamLogBytes)
	logsByTarget := make([]string, len(targets))
	errsByTarget := make([]error, len(targets))
	sem := make(chan struct{}, maxLogStreamConcurrency)
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t podlogs.LogTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			opts := &corev1.PodLogOptions{
				Container:  t.Container,
				TailLines:  &tailLines64,
				Timestamps: true,
			}
			// Follow cursor: a per-pod `since` for the all-pods view, falling back
			// to the single global sinceTime (used by a single-pod, multi-container
			// follow, which routes here yet sends one cursor).
			if st := sinceByPod[t.Pod]; st != nil {
				opts.SinceTime = st
			} else if sinceTime != nil {
				opts.SinceTime = sinceTime
			}
			logsByTarget[i], errsByTarget[i] = podlogs.FetchContainerLog(ctx, clientset, namespace, t.Pod, opts, perStreamBytes)
		}(i, t)
	}
	wg.Wait()

	fanOut := collectLogStreams(targets, logsByTarget, errsByTarget)
	if len(fanOut.streams) == 0 && fanOut.firstErr != nil {
		// Attribute the error to the single pod when only one was requested;
		// otherwise keep it workload-scoped.
		errName := ""
		if total == 1 {
			errName = name
		}
		writeLogStreamError(ctx, w, fanOut.firstErr, namespace, errName, strings.Join(containers, ","))
		return
	}

	writeWorkloadLogsResponse(w, buildWorkloadLogsResponse(pods, containers, fanOut, total, tailLines, tagPod, tagContainer, truncated))
}

// buildWorkloadLogsResponse assembles the multi-stream (all-pods/all-containers)
// response from a completed fan-out: it merges the collected streams and reports
// the coverage so the UI can explain a partial view. Streamed is the number of
// requested pods that produced logs; the result is Partial when fewer pods
// streamed than were requested (some forbidden, missing, or still starting) or
// when a cap truncated the fan-out. Forbidden carries the count of pods skipped
// for lack of pods/log access.
func buildWorkloadLogsResponse(pods, containers []string, fanOut logFanOut, total, tailLines int, tagPod, tagContainer, truncated bool) WorkloadLogsResponse {
	streamed := len(fanOut.streamedSet)
	return WorkloadLogsResponse{
		Pod:             strings.Join(pods, ","),
		Container:       strings.Join(containers, ","),
		Logs:            podlogs.MergeLogStreams(fanOut.streams, tailLines, tagPod, tagContainer, int(maxLogBytes)),
		Tagged:          tagPod,
		ContainerTagged: tagContainer,
		Total:           total,
		Streamed:        streamed,
		Partial:         streamed < total || truncated,
		Forbidden:       fanOut.forbidden,
	}
}
