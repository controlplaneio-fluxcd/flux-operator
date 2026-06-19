// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestTrimPartialLogLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "newline terminated", in: "line one\nline two\n", want: "line one\nline two\n"},
		{name: "drops partial trailing line", in: "line one\nline two\npar", want: "line one\nline two\n"},
		{name: "single partial line kept", in: "partial", want: "partial"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(trimPartialLogLine(tt.in)).To(Equal(tt.want))
		})
	}
}

func TestTrimPartialFirstLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "drops partial leading fragment", in: "tial\nline two\nline three\n", want: "line two\nline three\n"},
		{name: "single line with no newline kept", in: "only-line", want: "only-line"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(trimPartialFirstLine(tt.in)).To(Equal(tt.want))
		})
	}
}

func TestTailLogBytes(t *testing.T) {
	tests := []struct {
		name             string
		in               string
		limit            int
		want             string
		wantPartialFirst bool
	}{
		{name: "under limit returns all", in: "line1\nline2\n", limit: 100, want: "line1\nline2\n", wantPartialFirst: false},
		{name: "exact limit not truncated", in: "abcd", limit: 4, want: "abcd", wantPartialFirst: false},
		// The cut lands exactly after "line1\n", so the first retained line is
		// complete and must NOT be reported as partial (or the caller drops it).
		{name: "over limit cut on line boundary keeps complete first line", in: "line1\nline2\nline3\n", limit: 12, want: "line2\nline3\n", wantPartialFirst: false},
		{name: "over limit cutting mid-line", in: "line1\nline2\n", limit: 8, want: "1\nline2\n", wantPartialFirst: true},
		{name: "empty stream", in: "", limit: 16, want: "", wantPartialFirst: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			// strings.Reader hands out small reads, exercising the chunked loop.
			got, partialFirst, err := tailLogBytes(strings.NewReader(tt.in), tt.limit)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(got)).To(Equal(tt.want))
			g.Expect(partialFirst).To(Equal(tt.wantPartialFirst))
		})
	}
}

func TestParseLogEntries(t *testing.T) {
	t.Run("groups continuation lines with their timestamped entry and stamps the pod", func(t *testing.T) {
		g := NewWithT(t)

		blob := "2026-01-01T00:00:00Z panic: boom\ngoroutine 1 [running]:\nmain.main()\n" +
			"2026-01-01T00:00:01Z next line\n"
		entries := parseLogEntries(blob, "pod-a")

		g.Expect(entries).To(HaveLen(2))
		// The two non-timestamped lines stay attached to the first entry.
		g.Expect(entries[0].text).To(Equal("2026-01-01T00:00:00Z panic: boom\ngoroutine 1 [running]:\nmain.main()"))
		g.Expect(entries[0].pod).To(Equal("pod-a"))
		g.Expect(entries[1].text).To(Equal("2026-01-01T00:00:01Z next line"))
		g.Expect(entries[1].pod).To(Equal("pod-a"))
	})

	t.Run("a leading continuation with no preceding entry becomes its own entry", func(t *testing.T) {
		g := NewWithT(t)

		entries := parseLogEntries("orphan continuation\n2026-01-01T00:00:00Z line\n", "pod-a")
		g.Expect(entries).To(HaveLen(2))
		g.Expect(entries[0].ts.IsZero()).To(BeTrue())
		g.Expect(entries[0].text).To(Equal("orphan continuation"))
		g.Expect(entries[1].text).To(Equal("2026-01-01T00:00:00Z line"))
	})

	t.Run("empty payload yields no entries", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(parseLogEntries("", "pod-a")).To(BeEmpty())
	})
}

func TestMergeLogStreams(t *testing.T) {
	t.Run("interleaves two streams in chronological order untagged", func(t *testing.T) {
		g := NewWithT(t)

		app := logStream{pod: "pod-a", blob: "2026-01-01T00:00:00Z app a\n2026-01-01T00:00:02Z app b\n"}
		sidecar := logStream{pod: "pod-a", blob: "2026-01-01T00:00:01Z side a\n2026-01-01T00:00:03Z side b\n"}

		got := mergeLogStreams([]logStream{app, sidecar}, 0, false)
		g.Expect(got).To(Equal(
			"2026-01-01T00:00:00Z app a\n" +
				"2026-01-01T00:00:01Z side a\n" +
				"2026-01-01T00:00:02Z app b\n" +
				"2026-01-01T00:00:03Z side b\n"))
	})

	t.Run("orders fractional timestamps numerically, not lexically", func(t *testing.T) {
		g := NewWithT(t)

		// "0.12" is numerically after "0.1" but sorts before it lexically; the
		// merge must parse the timestamps to get this right.
		a := logStream{pod: "pod-a", blob: "2026-01-01T00:00:00.1Z first\n"}
		b := logStream{pod: "pod-a", blob: "2026-01-01T00:00:00.12Z second\n"}

		got := mergeLogStreams([]logStream{b, a}, 0, false)
		g.Expect(got).To(Equal("2026-01-01T00:00:00.1Z first\n2026-01-01T00:00:00.12Z second\n"))
	})

	t.Run("keeps a multi-line entry attached after sorting", func(t *testing.T) {
		g := NewWithT(t)

		app := logStream{pod: "pod-a", blob: "2026-01-01T00:00:00Z panic\nstack frame\n"}
		sidecar := logStream{pod: "pod-a", blob: "2026-01-01T00:00:01Z side\n"}

		got := mergeLogStreams([]logStream{app, sidecar}, 0, false)
		g.Expect(got).To(Equal("2026-01-01T00:00:00Z panic\nstack frame\n2026-01-01T00:00:01Z side\n"))
	})

	t.Run("caps to the newest tailLines entries across all streams", func(t *testing.T) {
		g := NewWithT(t)

		app := logStream{pod: "pod-a", blob: "2026-01-01T00:00:00Z app a\n2026-01-01T00:00:02Z app b\n"}
		sidecar := logStream{pod: "pod-a", blob: "2026-01-01T00:00:01Z side a\n2026-01-01T00:00:03Z side b\n"}

		got := mergeLogStreams([]logStream{app, sidecar}, 2, false)
		g.Expect(got).To(Equal("2026-01-01T00:00:02Z app b\n2026-01-01T00:00:03Z side b\n"))
	})

	t.Run("empty streams yield an empty payload", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(mergeLogStreams(nil, 100, false)).To(BeEmpty())
		g.Expect(mergeLogStreams([]logStream{{pod: "pod-a"}, {pod: "pod-b"}}, 100, false)).To(BeEmpty())
	})

	t.Run("tagged mode prefixes each timestamped line with its pod, interleaved", func(t *testing.T) {
		g := NewWithT(t)

		a := logStream{pod: "frontend-abc-gqh2x", blob: "2026-01-01T00:00:00Z a one\n2026-01-01T00:00:02Z a two\n"}
		b := logStream{pod: "frontend-abc-p8x2k", blob: "2026-01-01T00:00:01Z b one\n"}

		got := mergeLogStreams([]logStream{a, b}, 0, true)
		g.Expect(got).To(Equal(
			"frontend-abc-gqh2x 2026-01-01T00:00:00Z a one\n" +
				"frontend-abc-p8x2k 2026-01-01T00:00:01Z b one\n" +
				"frontend-abc-gqh2x 2026-01-01T00:00:02Z a two\n"))
	})

	t.Run("tagged mode tags only timestamped lines, not continuation or orphan lines", func(t *testing.T) {
		g := NewWithT(t)

		// A multi-line entry: only its timestamped first line is tagged; the
		// stack-trace continuation stays untagged so every tagged line is
		// uniformly "<pod> <timestamp> ...".
		a := logStream{pod: "pod-a", blob: "2026-01-01T00:00:01Z panic\nstack frame\n"}
		// A leading orphan continuation (zero ts) is left untagged too.
		b := logStream{pod: "pod-b", blob: "orphan line\n"}

		got := mergeLogStreams([]logStream{a, b}, 0, true)
		g.Expect(got).To(Equal(
			"orphan line\n" +
				"pod-a 2026-01-01T00:00:01Z panic\nstack frame\n"))
	})
}

func TestCapLogBytes(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{name: "under limit returns all", in: "line1\nline2\n", limit: 100, want: "line1\nline2\n"},
		{name: "trims partial leading line on a mid-line cut", in: "line1\nline2\nline3\n", limit: 13, want: "line2\nline3\n"},
		// The cut lands exactly after "line1\n", so the window already starts with
		// a complete line and the leading line must NOT be dropped.
		{name: "keeps complete first line when cut lands on a boundary", in: "line1\nline2\nline3\n", limit: 12, want: "line2\nline3\n"},
		{name: "no newline in window keeps tail bytes", in: "abcdef", limit: 3, want: "def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(capLogBytes(tt.in, tt.limit)).To(Equal(tt.want))
		})
	}
}

func TestDedupeNames(t *testing.T) {
	tests := []struct {
		name          string
		in            []string
		limit         int
		want          []string
		wantTruncated bool
	}{
		{name: "nil stays nil", in: nil, limit: 8, want: nil},
		{name: "preserves order without duplicates", in: []string{"app", "sidecar"}, limit: 8, want: []string{"app", "sidecar"}},
		{name: "drops duplicates keeping first occurrence", in: []string{"app", "app", "sidecar", "app"}, limit: 8, want: []string{"app", "sidecar"}},
		// De-duplication that leaves the result within the limit is not truncation.
		{name: "dedup within limit is not truncation", in: []string{"a", "a", "b", "b"}, limit: 2, want: []string{"a", "b"}},
		{name: "caps to the limit and reports truncation", in: []string{"a", "b", "c", "d"}, limit: 2, want: []string{"a", "b"}, wantTruncated: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, truncated := dedupeNames(tt.in, tt.limit)
			g.Expect(got).To(Equal(tt.want))
			g.Expect(truncated).To(Equal(tt.wantTruncated))
		})
	}
}

func TestBuildLogTargets(t *testing.T) {
	t.Run("product of pods and containers", func(t *testing.T) {
		g := NewWithT(t)
		targets, truncated := buildLogTargets([]string{"p1", "p2"}, []string{"app", "side"}, 64)
		g.Expect(truncated).To(BeFalse())
		g.Expect(targets).To(Equal([]logTarget{
			{pod: "p1", container: "app"}, {pod: "p1", container: "side"},
			{pod: "p2", container: "app"}, {pod: "p2", container: "side"},
		}))
	})

	t.Run("empty containers yield one default-container target per pod", func(t *testing.T) {
		g := NewWithT(t)
		targets, truncated := buildLogTargets([]string{"p1", "p2"}, nil, 64)
		g.Expect(truncated).To(BeFalse())
		g.Expect(targets).To(Equal([]logTarget{{pod: "p1"}, {pod: "p2"}}))
	})

	t.Run("caps to the stream limit and reports truncation", func(t *testing.T) {
		g := NewWithT(t)
		targets, truncated := buildLogTargets([]string{"p1", "p2", "p3"}, []string{"app", "side"}, 3)
		g.Expect(truncated).To(BeTrue())
		g.Expect(targets).To(HaveLen(3))
		g.Expect(targets).To(Equal([]logTarget{
			{pod: "p1", container: "app"}, {pod: "p1", container: "side"},
			{pod: "p2", container: "app"},
		}))
	})
}

func TestCollectLogStreams(t *testing.T) {
	errWaiting := errors.New("waiting to start")
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "pods/log"}, "p2", errors.New("nope"))

	t.Run("all succeed across two pods", func(t *testing.T) {
		g := NewWithT(t)
		targets := []logTarget{{pod: "p1", container: "app"}, {pod: "p2", container: "app"}}
		out := collectLogStreams(targets, []string{"x", "y"}, []error{nil, nil})
		g.Expect(out.streams).To(Equal([]logStream{{pod: "p1", blob: "x"}, {pod: "p2", blob: "y"}}))
		g.Expect(out.streamedSet).To(HaveLen(2))
		g.Expect(out.forbidden).To(Equal(0))
		g.Expect(out.firstErr).NotTo(HaveOccurred())
	})

	t.Run("partial failure skips the failed target and counts forbidden", func(t *testing.T) {
		g := NewWithT(t)
		targets := []logTarget{{pod: "p1", container: "app"}, {pod: "p2", container: "app"}}
		out := collectLogStreams(targets, []string{"x", ""}, []error{nil, forbidden})
		g.Expect(out.streams).To(Equal([]logStream{{pod: "p1", blob: "x"}}))
		g.Expect(out.streamedSet).To(HaveLen(1))
		g.Expect(out.forbidden).To(Equal(1))
		g.Expect(out.firstErr).To(MatchError(forbidden))
	})

	t.Run("all fail returns no streams and the first error", func(t *testing.T) {
		g := NewWithT(t)
		targets := []logTarget{{pod: "p1", container: "app"}, {pod: "p2", container: "app"}}
		out := collectLogStreams(targets, []string{"", ""}, []error{errWaiting, forbidden})
		g.Expect(out.streams).To(BeEmpty())
		g.Expect(out.streamedSet).To(BeEmpty())
		g.Expect(out.forbidden).To(Equal(1))
		g.Expect(out.firstErr).To(MatchError(errWaiting))
	})

	t.Run("forbidden is counted per pod, not per container target", func(t *testing.T) {
		g := NewWithT(t)
		// One forbidden pod with two containers must count once, not twice.
		targets := []logTarget{{pod: "p1", container: "app"}, {pod: "p1", container: "side"}}
		out := collectLogStreams(targets, []string{"", ""}, []error{forbidden, forbidden})
		g.Expect(out.forbidden).To(Equal(1))
	})
}

func TestBuildWorkloadLogsResponse(t *testing.T) {
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "pods/log"}, "p2", errors.New("nope"))

	t.Run("partial coverage when one requested pod is forbidden", func(t *testing.T) {
		g := NewWithT(t)
		// Two pods requested, only p1 streams; p2 is forbidden. The response must
		// report 200-style partial coverage with the per-pod counts the UI shows.
		targets := []logTarget{{pod: "p1", container: "app"}, {pod: "p2", container: "app"}}
		fanOut := collectLogStreams(targets, []string{"2026-01-01T00:00:00Z hi\n", ""}, []error{nil, forbidden})
		resp := buildWorkloadLogsResponse([]string{"p1", "p2"}, []string{"app"}, fanOut, 2, 1000, true, false)

		g.Expect(resp.Tagged).To(BeTrue())
		g.Expect(resp.Total).To(Equal(2))
		g.Expect(resp.Streamed).To(Equal(1))
		g.Expect(resp.Partial).To(BeTrue())
		g.Expect(resp.Forbidden).To(Equal(1))
		g.Expect(resp.Pod).To(Equal("p1,p2"))
		g.Expect(resp.Logs).To(ContainSubstring("hi"))
	})

	t.Run("full coverage is not partial", func(t *testing.T) {
		g := NewWithT(t)
		targets := []logTarget{{pod: "p1", container: "app"}, {pod: "p2", container: "app"}}
		fanOut := collectLogStreams(targets, []string{"2026-01-01T00:00:00Z a\n", "2026-01-01T00:00:01Z b\n"}, []error{nil, nil})
		resp := buildWorkloadLogsResponse([]string{"p1", "p2"}, []string{"app"}, fanOut, 2, 1000, true, false)

		g.Expect(resp.Streamed).To(Equal(2))
		g.Expect(resp.Partial).To(BeFalse())
		g.Expect(resp.Forbidden).To(Equal(0))
	})

	t.Run("truncation forces partial even when every requested pod streamed", func(t *testing.T) {
		g := NewWithT(t)
		// total counts requested pods (1) and that pod streamed, but the fan-out
		// was capped, so the response must still flag the result as partial.
		targets := []logTarget{{pod: "p1", container: "app"}}
		fanOut := collectLogStreams(targets, []string{"2026-01-01T00:00:00Z x\n"}, []error{nil})
		resp := buildWorkloadLogsResponse([]string{"p1"}, []string{"app"}, fanOut, 1, 1000, true, true)

		g.Expect(resp.Streamed).To(Equal(1))
		g.Expect(resp.Total).To(Equal(1))
		g.Expect(resp.Partial).To(BeTrue())
	})
}

func TestWorkloadLogsHandler_MethodNotAllowed(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/logs", nil)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("Method not allowed"))
}

func TestWorkloadLogsHandler_MissingParams(t *testing.T) {
	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name  string
		query string
	}{
		{name: "missing both", query: ""},
		{name: "missing name", query: "namespace=default"},
		{name: "missing namespace", query: "name=test-pod"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/workload/logs?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.WorkloadLogsHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("Missing required query parameters"))
		})
	}
}

func TestWorkloadLogsHandler_InvalidParams(t *testing.T) {
	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name    string
		query   string
		message string
	}{
		{name: "invalid tailLines", query: "namespace=default&name=test-pod&tailLines=abc", message: "Invalid tailLines parameter"},
		{name: "negative tailLines", query: "namespace=default&name=test-pod&tailLines=-5", message: "Invalid tailLines parameter"},
		{name: "invalid previous", query: "namespace=default&name=test-pod&previous=maybe", message: "Invalid previous parameter"},
		{name: "invalid sinceTime", query: "namespace=default&name=test-pod&sinceTime=not-a-time", message: "Invalid sinceTime parameter"},
		{name: "since without separator", query: "namespace=default&name=test-pod&pod=other&since=pod-a", message: "Invalid since parameter"},
		{name: "since with empty pod", query: "namespace=default&name=test-pod&pod=other&since==2026-01-01T00:00:00Z", message: "Invalid since parameter"},
		{name: "since with bad timestamp", query: "namespace=default&name=test-pod&pod=other&since=pod-a=not-a-time", message: "Invalid since parameter"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/workload/logs?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.WorkloadLogsHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring(tc.message))
		})
	}
}

func TestWorkloadLogsHandler_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// A user without the pods/log permission must be rejected with 403 by the
	// API server when the impersonated client attempts to read the logs.
	username := "logs-forbidden-user"
	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Logs Forbidden User"},
		Impersonation: imp,
	}, userClient)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/workload/logs?namespace=default&name=test-pod&container=app", nil).WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
}

func TestWorkloadLogsHandler_AllContainersForbidden(t *testing.T) {
	g := NewWithT(t)

	// The all-containers path (repeated container params) is governed by the same
	// pods/log RBAC as the single-container path: a user without it gets 403 once
	// every container stream fails.
	username := "logs-forbidden-multi-user"
	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Logs Forbidden Multi User"},
		Impersonation: imp,
	}, userClient)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/workload/logs?namespace=default&name=test-pod&container=app&container=sidecar", nil).WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
}

func TestWorkloadLogsHandler_AllPodsForbidden(t *testing.T) {
	g := NewWithT(t)

	// The all-pods path (the primary name plus repeated pod params) is governed by
	// the same pods/log RBAC. With more than one pod the failure is workload-scoped
	// (no single pod is named), so the 403 message does not pin to one pod.
	username := "logs-forbidden-allpods-user"
	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Logs Forbidden All Pods User"},
		Impersonation: imp,
	}, userClient)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/workload/logs?namespace=default&name=pod-a&pod=pod-b&container=app", nil).WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
	// Workload-scoped message: it does not name a single pod.
	g.Expect(rec.Body.String()).To(ContainSubstring("workload pod logs"))
}

func TestGetWorkloadStatus_ViewLogsCapability(t *testing.T) {
	g := NewWithT(t)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-logs",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-logs"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-logs"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:1.25"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// baseRules grant enough access to read the workload and list its pods,
	// but deliberately omit the pods/log subresource.
	baseRules := []rbacv1.PolicyRule{
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "list"}},
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}},
	}

	t.Run("with pods/log permission", func(t *testing.T) {
		g := NewWithT(t)

		rules := append([]rbacv1.PolicyRule{}, baseRules...)
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""}, Resources: []string{"pods/log"}, Verbs: []string{"get"},
		})
		userCtx := bindWorkloadLogsUser(t, g, "logs-reader-user", rules)

		workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-logs", "default", false)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionViewLogs))
	})

	t.Run("without pods/log permission", func(t *testing.T) {
		g := NewWithT(t)

		userCtx := bindWorkloadLogsUser(t, g, "logs-noreader-user", baseRules)

		workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-logs", "default", false)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload.UserActions).NotTo(ContainElement(fluxcdv1.UserActionViewLogs))
	})
}

// bindWorkloadLogsUser creates a namespaced Role with the given rules in the
// default namespace, binds it to username, and returns an impersonated user
// context for use with the handler.
func bindWorkloadLogsUser(t *testing.T, g *WithT, username string, rules []rbacv1.PolicyRule) context.Context {
	t.Helper()

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: username + "-role", Namespace: "default"},
		Rules:      rules,
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(ctx, role) })

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: username + "-binding", Namespace: "default"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{{Kind: "User", Name: username}},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(ctx, roleBinding) })

	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	return user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: username},
		Impersonation: imp,
	}, userClient)
}
