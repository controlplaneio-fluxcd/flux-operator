// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package podlogs provides helpers for resolving pod selectors and processing
// Kubernetes container log streams.
package podlogs

import (
	"context"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// LogTarget is one pod and container stream to read. An empty Container lets
// the kubelet select the pod's default container.
type LogTarget struct {
	Pod       string
	Container string
}

// LogStream is the timestamped payload fetched from one log target.
type LogStream struct {
	Pod       string
	Container string
	Blob      string
}

// logEntry is one timestamped record and any attached continuation lines.
type logEntry struct {
	ts        time.Time
	text      string
	pod       string
	container string
}

// MergeOption configures MergeLogStreams filtering and output rendering.
type MergeOption func(*mergeOptions)

// mergeOptions contains optional MergeLogStreams behavior.
type mergeOptions struct {
	secondTimestamps bool
	grep             *regexp.Regexp
}

// WithSecondTimestamps formats timestamped entries at RFC 3339 seconds precision.
func WithSecondTimestamps() MergeOption {
	return func(opts *mergeOptions) {
		opts.secondTimestamps = true
	}
}

// WithGrep keeps only the entries whose rendered text, including
// continuation lines, matches the expression.
func WithGrep(expression *regexp.Regexp) MergeOption {
	return func(opts *mergeOptions) {
		opts.grep = expression
	}
}

// MergeResult is the merged log payload and its entry statistics.
type MergeResult struct {
	// Logs is the merged payload.
	Logs string
	// Entries is the number of entries kept after filtering and before tailing.
	Entries int
	// Truncated reports whether the tail or the byte limit dropped entries.
	Truncated bool
}

// trimPartialLogLine drops a trailing partial log line from the payload.
//
// Container runtimes newline-terminate every emitted log line, so a payload
// that does not end with a newline has been truncated mid-line because the
// container was writing the line when the logs were read. A payload with no
// newline at all is returned unchanged so a single short line is not lost.
func trimPartialLogLine(logs string) string {
	if logs == "" || logs[len(logs)-1] == '\n' {
		return logs
	}
	if i := strings.LastIndexByte(logs, '\n'); i >= 0 {
		return logs[:i+1]
	}
	return logs
}

// tailLogBytes reads r to EOF but retains only the most recent limit bytes.
//
// partialFirst is true when the retained slice begins mid-line, so the caller
// should drop that leading fragment; dropped is true when older bytes were
// discarded to honor the limit.
func tailLogBytes(r io.Reader, limit int) (data []byte, partialFirst, dropped bool, err error) {
	const chunkSize = 32 * 1024
	buf := make([]byte, 0, limit+chunkSize)
	chunk := make([]byte, chunkSize)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if len(buf) > limit {
				partialFirst = buf[len(buf)-limit-1] != '\n'
				dropped = true
				copy(buf, buf[len(buf)-limit:])
				buf = buf[:limit]
			}
		}
		if err == io.EOF {
			return buf, partialFirst, dropped, nil
		}
		if err != nil {
			return nil, partialFirst, dropped, err
		}
	}
}

// trimPartialFirstLine drops a leading partial log line from a byte-capped
// payload. A payload with no newline is returned unchanged.
func trimPartialFirstLine(logs string) string {
	if _, rest, found := strings.Cut(logs, "\n"); found {
		return rest
	}
	return logs
}

// FetchContainerLog streams one container's logs and keeps only the newest
// byteLimit bytes, trimming partial first and last lines.
func FetchContainerLog(ctx context.Context, clientset kubernetes.Interface, namespace, name string, opts *corev1.PodLogOptions, byteLimit int) (string, error) {
	logs, _, err := FetchContainerLogWithStats(ctx, clientset, namespace, name, opts, byteLimit)
	return logs, err
}

// FetchContainerLogWithStats is FetchContainerLog that also reports whether
// the byte limit dropped older log lines.
func FetchContainerLogWithStats(ctx context.Context, clientset kubernetes.Interface, namespace, name string, opts *corev1.PodLogOptions, byteLimit int) (string, bool, error) {
	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(name, opts).Stream(ctx)
	if err != nil {
		return "", false, err
	}
	defer stream.Close()

	data, partialFirst, dropped, err := tailLogBytes(stream, byteLimit)
	if err != nil {
		return "", false, err
	}

	logs := trimPartialLogLine(string(data))
	if partialFirst {
		logs = trimPartialFirstLine(logs)
	}
	return logs, dropped, nil
}

// parseLogTimestamp parses the leading RFC3339 timestamp token added by the
// kubelet when PodLogOptions.Timestamps is enabled.
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

// parseLogEntries splits a timestamped payload into entries and attaches
// continuation lines, such as stack frames, to the preceding entry.
func parseLogEntries(blob, pod, container string) []logEntry {
	var entries []logEntry
	for line := range strings.SplitSeq(blob, "\n") {
		if line == "" {
			continue
		}
		if ts, ok := parseLogTimestamp(line); ok {
			entries = append(entries, logEntry{ts: ts, text: line, pod: pod, container: container})
			continue
		}
		if n := len(entries); n > 0 {
			entries[n-1].text += "\n" + line
		} else {
			entries = append(entries, logEntry{text: line, pod: pod, container: container})
		}
	}
	return entries
}

// MergeLogStreams interleaves streams chronologically, keeps the newest
// tailLines entries, and caps the result to byteLimit bytes on a line boundary.
// Timestamped lines are optionally tagged in pod-then-container order;
// continuation lines remain attached and untagged. Merge options filter the
// entries and shape the rendered lines without changing chronological ordering.
func MergeLogStreams(streams []LogStream, tailLines int, tagPod, tagContainer bool, byteLimit int, options ...MergeOption) string {
	return MergeLogStreamsWithStats(streams, tailLines, tagPod, tagContainer, byteLimit, options...).Logs
}

// MergeLogStreamsWithStats is MergeLogStreams with entry statistics.
func MergeLogStreamsWithStats(streams []LogStream, tailLines int, tagPod, tagContainer bool, byteLimit int, options ...MergeOption) MergeResult {
	opts := mergeOptions{}
	for _, option := range options {
		option(&opts)
	}

	var entries []logEntry
	for _, stream := range streams {
		for _, entry := range parseLogEntries(stream.Blob, stream.Pod, stream.Container) {
			if opts.grep != nil {
				entry.text = renderLogEntry(entry, opts, tagPod, tagContainer)
				if !opts.grep.MatchString(entry.text) {
					continue
				}
			}
			entries = append(entries, entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].ts.Before(entries[j].ts)
	})

	result := MergeResult{Entries: len(entries)}
	if tailLines > 0 && len(entries) > tailLines {
		entries = entries[len(entries)-tailLines:]
		result.Truncated = true
	}

	var sb strings.Builder
	for _, entry := range entries {
		text := entry.text
		if opts.grep == nil {
			text = renderLogEntry(entry, opts, tagPod, tagContainer)
		}
		sb.WriteString(text)
		sb.WriteByte('\n')
	}
	result.Logs = capLogBytes(sb.String(), byteLimit)
	if len(result.Logs) < sb.Len() {
		result.Truncated = true
	}
	return result
}

// renderLogEntry returns the entry text as emitted in the merged payload.
func renderLogEntry(entry logEntry, opts mergeOptions, tagPod, tagContainer bool) string {
	if entry.ts.IsZero() {
		return entry.text
	}

	var sb strings.Builder
	if tagPod {
		sb.WriteString(entry.pod)
		sb.WriteByte(' ')
	}
	if tagContainer {
		sb.WriteString(entry.container)
		sb.WriteByte(' ')
	}
	text := entry.text
	if opts.secondTimestamps {
		if _, rest, found := strings.Cut(text, " "); found {
			text = entry.ts.UTC().Format(time.RFC3339) + " " + rest
		}
	}
	sb.WriteString(text)
	return sb.String()
}

// DedupeNames removes duplicate names while preserving order and caps the
// result to limit. It reports whether the cap dropped a name.
func DedupeNames(names []string, limit int) ([]string, bool) {
	if len(names) == 0 {
		return names, false
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	truncated := false
	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if len(out) == limit {
			truncated = true
			break
		}
		out = append(out, name)
	}
	return out, truncated
}

// BuildLogTargets expands pod and container names into their product, capped
// to limit streams. An empty container list creates one default-container
// target per pod. It reports whether the cap dropped a target.
func BuildLogTargets(pods, containers []string, limit int) ([]LogTarget, bool) {
	var targets []LogTarget
	for _, pod := range pods {
		if len(containers) == 0 {
			if len(targets) == limit {
				return targets, true
			}
			targets = append(targets, LogTarget{Pod: pod})
			continue
		}
		for _, container := range containers {
			if len(targets) == limit {
				return targets, true
			}
			targets = append(targets, LogTarget{Pod: pod, Container: container})
		}
	}
	return targets, false
}

// capLogBytes keeps only the newest limit bytes of a newline-terminated log
// payload, trimming a partial leading line.
func capLogBytes(logs string, limit int) string {
	if limit <= 0 || len(logs) <= limit {
		return logs
	}
	cut := len(logs) - limit
	if logs[cut-1] == '\n' {
		return logs[cut:]
	}
	if i := strings.IndexByte(logs[cut:], '\n'); i >= 0 {
		return logs[cut+i+1:]
	}
	return logs[cut:]
}

// WorkloadPodSelector extracts a workload's full pod selector, including
// matchLabels and matchExpressions. It returns nil if the selector is absent
// or invalid.
func WorkloadPodSelector(obj *unstructured.Unstructured) labels.Selector {
	sel, found, _ := unstructured.NestedMap(obj.Object, "spec", "selector")
	if !found {
		return nil
	}
	labelSelector := &metav1.LabelSelector{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(sel, labelSelector); err != nil {
		return nil
	}
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil
	}
	return selector
}
