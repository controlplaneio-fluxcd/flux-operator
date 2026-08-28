// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package podlogs

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
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
		{name: "under limit returns all", in: "line1\nline2\n", limit: 100, want: "line1\nline2\n"},
		{name: "exact limit not truncated", in: "abcd", limit: 4, want: "abcd"},
		{name: "over limit cut on line boundary keeps complete first line", in: "line1\nline2\nline3\n", limit: 12, want: "line2\nline3\n"},
		{name: "over limit cutting mid-line", in: "line1\nline2\n", limit: 8, want: "1\nline2\n", wantPartialFirst: true},
		{name: "empty stream", in: "", limit: 16, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, partialFirst, err := tailLogBytes(strings.NewReader(tt.in), tt.limit)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(got)).To(Equal(tt.want))
			g.Expect(partialFirst).To(Equal(tt.wantPartialFirst))
		})
	}
}

func TestParseLogEntries(t *testing.T) {
	t.Run("groups continuation lines with their timestamped entry", func(t *testing.T) {
		g := NewWithT(t)
		blob := "2026-01-01T00:00:00Z panic: boom\ngoroutine 1 [running]:\nmain.main()\n" +
			"2026-01-01T00:00:01Z next line\n"
		entries := parseLogEntries(blob, "pod-a", "app")
		g.Expect(entries).To(HaveLen(2))
		g.Expect(entries[0].text).To(Equal("2026-01-01T00:00:00Z panic: boom\ngoroutine 1 [running]:\nmain.main()"))
		g.Expect(entries[0].pod).To(Equal("pod-a"))
		g.Expect(entries[0].container).To(Equal("app"))
		g.Expect(entries[1].text).To(Equal("2026-01-01T00:00:01Z next line"))
	})

	t.Run("leading continuation becomes a zero timestamp entry", func(t *testing.T) {
		g := NewWithT(t)
		entries := parseLogEntries("orphan continuation\n2026-01-01T00:00:00Z line\n", "pod-a", "app")
		g.Expect(entries).To(HaveLen(2))
		g.Expect(entries[0].ts.IsZero()).To(BeTrue())
		g.Expect(entries[0].text).To(Equal("orphan continuation"))
		g.Expect(entries[1].text).To(Equal("2026-01-01T00:00:00Z line"))
	})

	t.Run("empty payload yields no entries", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(parseLogEntries("", "pod-a", "app")).To(BeEmpty())
	})
}

func TestMergeLogStreams(t *testing.T) {
	const byteLimit = 512 * 1024

	t.Run("interleaves streams in chronological order", func(t *testing.T) {
		g := NewWithT(t)
		app := LogStream{Pod: "pod-a", Blob: "2026-01-01T00:00:00Z app a\n2026-01-01T00:00:02Z app b\n"}
		sidecar := LogStream{Pod: "pod-a", Blob: "2026-01-01T00:00:01Z side a\n2026-01-01T00:00:03Z side b\n"}
		got := MergeLogStreams([]LogStream{app, sidecar}, 0, false, false, byteLimit)
		g.Expect(got).To(Equal("2026-01-01T00:00:00Z app a\n2026-01-01T00:00:01Z side a\n2026-01-01T00:00:02Z app b\n2026-01-01T00:00:03Z side b\n"))
	})

	t.Run("orders fractional timestamps numerically", func(t *testing.T) {
		g := NewWithT(t)
		a := LogStream{Blob: "2026-01-01T00:00:00.1Z first\n"}
		b := LogStream{Blob: "2026-01-01T00:00:00.12Z second\n"}
		g.Expect(MergeLogStreams([]LogStream{b, a}, 0, false, false, byteLimit)).To(Equal("2026-01-01T00:00:00.1Z first\n2026-01-01T00:00:00.12Z second\n"))
	})

	t.Run("keeps continuation lines attached", func(t *testing.T) {
		g := NewWithT(t)
		app := LogStream{Blob: "2026-01-01T00:00:00Z panic\nstack frame\n"}
		sidecar := LogStream{Blob: "2026-01-01T00:00:01Z side\n"}
		g.Expect(MergeLogStreams([]LogStream{app, sidecar}, 0, false, false, byteLimit)).To(Equal("2026-01-01T00:00:00Z panic\nstack frame\n2026-01-01T00:00:01Z side\n"))
	})

	t.Run("keeps newest entries across all streams", func(t *testing.T) {
		g := NewWithT(t)
		app := LogStream{Blob: "2026-01-01T00:00:00Z app a\n2026-01-01T00:00:02Z app b\n"}
		sidecar := LogStream{Blob: "2026-01-01T00:00:01Z side a\n2026-01-01T00:00:03Z side b\n"}
		g.Expect(MergeLogStreams([]LogStream{app, sidecar}, 2, false, false, byteLimit)).To(Equal("2026-01-01T00:00:02Z app b\n2026-01-01T00:00:03Z side b\n"))
	})

	t.Run("empty streams yield empty output", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(MergeLogStreams(nil, 100, false, false, byteLimit)).To(BeEmpty())
		g.Expect(MergeLogStreams([]LogStream{{Pod: "pod-a"}}, 100, false, false, byteLimit)).To(BeEmpty())
	})

	t.Run("tags pod only on timestamped lines", func(t *testing.T) {
		g := NewWithT(t)
		a := LogStream{Pod: "p1", Blob: "2026-01-01T00:00:01Z panic\nstack frame\n"}
		b := LogStream{Pod: "p2", Blob: "orphan line\n"}
		g.Expect(MergeLogStreams([]LogStream{a, b}, 0, true, false, byteLimit)).To(Equal("orphan line\np1 2026-01-01T00:00:01Z panic\nstack frame\n"))
	})

	t.Run("tags container for a single pod", func(t *testing.T) {
		g := NewWithT(t)
		app := LogStream{Pod: "p1", Container: "app", Blob: "2026-01-01T00:00:00Z a\n"}
		side := LogStream{Pod: "p1", Container: "side", Blob: "2026-01-01T00:00:01Z s\n"}
		g.Expect(MergeLogStreams([]LogStream{app, side}, 0, false, true, byteLimit)).To(Equal("app 2026-01-01T00:00:00Z a\nside 2026-01-01T00:00:01Z s\n"))
	})

	t.Run("tags pod then container", func(t *testing.T) {
		g := NewWithT(t)
		a := LogStream{Pod: "p1", Container: "app", Blob: "2026-01-01T00:00:00Z a\nframe\n"}
		b := LogStream{Pod: "p2", Container: "side", Blob: "2026-01-01T00:00:01Z b\n"}
		g.Expect(MergeLogStreams([]LogStream{a, b}, 0, true, true, byteLimit)).To(Equal("p1 app 2026-01-01T00:00:00Z a\nframe\np2 side 2026-01-01T00:00:01Z b\n"))
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
		{name: "trims partial leading line", in: "line1\nline2\nline3\n", limit: 13, want: "line2\nline3\n"},
		{name: "keeps boundary line", in: "line1\nline2\nline3\n", limit: 12, want: "line2\nline3\n"},
		{name: "no newline keeps tail bytes", in: "abcdef", limit: 3, want: "def"},
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
		{name: "preserves order", in: []string{"app", "sidecar"}, limit: 8, want: []string{"app", "sidecar"}},
		{name: "drops duplicates", in: []string{"app", "app", "sidecar"}, limit: 8, want: []string{"app", "sidecar"}},
		{name: "dedup within limit", in: []string{"a", "a", "b", "b"}, limit: 2, want: []string{"a", "b"}},
		{name: "caps names", in: []string{"a", "b", "c"}, limit: 2, want: []string{"a", "b"}, wantTruncated: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got, truncated := DedupeNames(tt.in, tt.limit)
			g.Expect(got).To(Equal(tt.want))
			g.Expect(truncated).To(Equal(tt.wantTruncated))
		})
	}
}

func TestBuildLogTargets(t *testing.T) {
	g := NewWithT(t)
	targets, truncated := BuildLogTargets([]string{"p1", "p2"}, []string{"app", "side"}, 3)
	g.Expect(truncated).To(BeTrue())
	g.Expect(targets).To(Equal([]LogTarget{
		{Pod: "p1", Container: "app"},
		{Pod: "p1", Container: "side"},
		{Pod: "p2", Container: "app"},
	}))

	targets, truncated = BuildLogTargets([]string{"p1", "p2"}, nil, 4)
	g.Expect(truncated).To(BeFalse())
	g.Expect(targets).To(Equal([]LogTarget{{Pod: "p1"}, {Pod: "p2"}}))
}
