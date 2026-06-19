// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
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

	// maxLogContainers caps the number of containers streamed for the
	// all-containers view, bounding the per-request log-stream fan-out. Real
	// pods have only a handful of containers; this is a defensive ceiling for a
	// request that names many directly.
	maxLogContainers = 64
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

// fetchContainerLog streams the logs of a single container with the given
// options and returns the payload trimmed of any partial first/last line. It is
// shared by the single-container and all-containers paths of the handler.
func fetchContainerLog(ctx context.Context, clientset kubernetes.Interface, namespace, name string, opts *corev1.PodLogOptions) (string, error) {
	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(name, opts).Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	// Read the stream, keeping only the newest maxLogBytes so the payload stays
	// bounded while returning the most recent lines.
	data, partialFirst, err := tailLogBytes(stream, int(maxLogBytes))
	if err != nil {
		return "", err
	}

	// Drop the partial first line only when the byte cap cut mid-line (a cut on
	// a line boundary keeps a complete first line), and the partial last line
	// when the container was mid-write.
	logs := trimPartialLogLine(string(data))
	if partialFirst {
		logs = trimPartialFirstLine(logs)
	}
	return logs, nil
}

// writeLogStreamError maps a GetLogs stream error to an HTTP response: 403 for a
// forbidden user, 404 for a missing pod, and 500 otherwise (logged).
func writeLogStreamError(ctx context.Context, w http.ResponseWriter, err error, namespace, name, container string) {
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
}

// writeWorkloadLogs encodes the log payload as the JSON WorkloadLogsResponse.
func writeWorkloadLogs(w http.ResponseWriter, pod, container, logs string) {
	w.Header().Set("Content-Type", "application/json")
	resp := WorkloadLogsResponse{
		Pod:       pod,
		Container: container,
		Logs:      logs,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// logEntry is one record used when interleaving multiple container streams: a
// line carrying a leading RFC3339 timestamp plus any immediately following
// continuation lines that lack their own timestamp (e.g. stack-trace frames).
// The parsed timestamp is the sort key; text retains the original line(s),
// including the timestamp prefix, since the frontend strips it for display.
type logEntry struct {
	ts   time.Time
	text string
}

// parseLogTimestamp parses the leading RFC3339 timestamp token kubelet prepends
// to each line when PodLogOptions.Timestamps is set. It reports false when the
// line does not start with such a token (e.g. a stack-trace continuation).
func parseLogTimestamp(line string) (time.Time, bool) {
	tsStr, _, found := strings.Cut(line, " ")
	if !found {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// parseLogEntries splits a timestamped log payload into entries. Each entry
// begins at a line whose first token parses as an RFC3339 timestamp and absorbs
// subsequent lines without such a prefix, so multi-line records (stack traces)
// stay intact and ordered. A continuation line with no preceding entry becomes
// its own zero-timestamped entry, which sorts to the front.
func parseLogEntries(blob string) []logEntry {
	var entries []logEntry
	for line := range strings.SplitSeq(blob, "\n") {
		if line == "" {
			continue
		}
		if ts, ok := parseLogTimestamp(line); ok {
			entries = append(entries, logEntry{ts: ts, text: line})
			continue
		}
		if n := len(entries); n > 0 {
			entries[n-1].text += "\n" + line
		} else {
			entries = append(entries, logEntry{text: line})
		}
	}
	return entries
}

// mergeLogStreams interleaves the timestamped log payloads of multiple
// containers into a single chronological stream. Entries are stable-sorted by
// their timestamp (so records logged at the same instant keep a deterministic
// order), capped to the newest tailLines entries, and finally to the newest
// maxLogBytes bytes on a line boundary so the payload stays bounded.
func mergeLogStreams(blobs []string, tailLines int) string {
	var entries []logEntry
	for _, b := range blobs {
		entries = append(entries, parseLogEntries(b)...)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].ts.Before(entries[j].ts)
	})
	if tailLines > 0 && len(entries) > tailLines {
		entries = entries[len(entries)-tailLines:]
	}
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.text)
		sb.WriteByte('\n')
	}
	return capLogBytes(sb.String(), int(maxLogBytes))
}

// dedupeContainers removes duplicate container names (keeping first occurrence)
// and caps the result to limit, so a request naming the same container twice
// does not stream it twice and a request naming many containers cannot fan out
// without bound. Order is preserved so the merge input stays deterministic.
func dedupeContainers(names []string, limit int) []string {
	if len(names) == 0 {
		return names
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
		if len(out) == limit {
			break
		}
	}
	return out
}

// collectContainerLogs partitions the per-container fan-out results into the
// successful log payloads and the first error encountered. It implements the
// best-effort policy of the all-containers view: a container that failed (e.g.
// is still waiting to start) is skipped as long as another succeeded, so the
// returned error is only meaningful to the caller when no blob was collected.
func collectContainerLogs(logs []string, errs []error) ([]string, error) {
	var blobs []string
	var firstErr error
	for i := range logs {
		if errs[i] != nil {
			if firstErr == nil {
				firstErr = errs[i]
			}
			continue
		}
		blobs = append(blobs, logs[i])
	}
	return blobs, firstErr
}

// capLogBytes keeps only the newest limit bytes of a newline-terminated log
// payload, trimming any partial leading line so the result starts on a line
// boundary.
func capLogBytes(logs string, limit int) string {
	if len(logs) <= limit {
		return logs
	}
	cut := len(logs) - limit
	// When the cut lands exactly on a line boundary (the preceding byte is a
	// newline), the window already starts with a complete line and must not be
	// trimmed further; otherwise drop the leading partial fragment.
	if logs[cut-1] == '\n' {
		return logs[cut:]
	}
	if i := strings.IndexByte(logs[cut:], '\n'); i >= 0 {
		return logs[cut+i+1:]
	}
	return logs[cut:]
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
// The container query parameter may be repeated to request the "All containers"
// view: each named container's logs are streamed and interleaved chronologically
// into a single payload (the previous-instance option is not offered there). A
// single or absent container keeps the per-container behavior. The frontend
// supplies the container names, so no pod read is required and access stays
// governed solely by the pods/log RBAC, which is enforced per pod.
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

	// Collect the requested container names. Repeated container params request
	// the "All containers" view; a single or absent container keeps the original
	// per-container behavior. Empty values are dropped so an explicit container=
	// does not select a phantom container, and the list is de-duplicated and
	// capped so a direct caller cannot stream a container twice or fan out
	// without bound.
	var containers []string
	for _, c := range req.URL.Query()["container"] {
		if c != "" {
			containers = append(containers, c)
		}
	}
	containers = dedupeContainers(containers, maxLogContainers)

	// TailLines is always set so the kubelet returns the newest lines (and
	// bounds the read to at most tailLines lines). When following, SinceTime
	// additionally restricts the range to entries after the last seen line, so
	// the response is the newest lines since then. LimitBytes is deliberately
	// not used: it keeps the oldest bytes of the range, dropping the most recent
	// lines; the byte cap is enforced by keeping the trailing bytes.

	// Single-container path (one or no container): preserves the previous-instance
	// and follow semantics unchanged. An absent container lets the kubelet pick
	// the pod's default container.
	if len(containers) <= 1 {
		container := ""
		if len(containers) == 1 {
			container = containers[0]
		}
		opts := &corev1.PodLogOptions{
			Container:  container,
			TailLines:  &tailLines,
			Previous:   previous,
			Timestamps: true,
		}
		if sinceTime != nil {
			opts.SinceTime = sinceTime
		}
		logs, err := fetchContainerLog(ctx, clientset, namespace, name, opts)
		if err != nil {
			writeLogStreamError(ctx, w, err, namespace, name, container)
			return
		}
		writeWorkloadLogs(w, name, container, logs)
		return
	}

	// All-containers path: stream every named container concurrently and merge
	// the results by timestamp. Previous-instance logs are not offered here, so
	// Previous is always false. The fetch is best-effort: a container with no
	// readable logs yet (e.g. waiting to start) is skipped, and an error is only
	// returned to the client when every container fails.
	logsByContainer := make([]string, len(containers))
	errsByContainer := make([]error, len(containers))
	var wg sync.WaitGroup
	for i, c := range containers {
		wg.Add(1)
		go func(i int, c string) {
			defer wg.Done()
			opts := &corev1.PodLogOptions{
				Container:  c,
				TailLines:  &tailLines,
				Timestamps: true,
			}
			if sinceTime != nil {
				opts.SinceTime = sinceTime
			}
			logsByContainer[i], errsByContainer[i] = fetchContainerLog(ctx, clientset, namespace, name, opts)
		}(i, c)
	}
	wg.Wait()

	blobs, firstErr := collectContainerLogs(logsByContainer, errsByContainer)
	// Only fail when no container produced logs; otherwise return what we have.
	if len(blobs) == 0 && firstErr != nil {
		writeLogStreamError(ctx, w, firstErr, namespace, name, strings.Join(containers, ","))
		return
	}

	merged := mergeLogStreams(blobs, int(tailLines))
	writeWorkloadLogs(w, name, strings.Join(containers, ","), merged)
}
