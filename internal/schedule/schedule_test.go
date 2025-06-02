// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package schedule_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/schedule"
)

func TestGetPrevAndNextTriggers(t *testing.T) {
	for _, tt := range []struct {
		cron     string
		timeZone string
		now      string
		next     string
		prev     string
		err      string
	}{
		{
			cron: "0 * * * *",
			now:  "2025-01-01T12:00:00Z",
			next: "2025-01-01T13:00:00Z",
			prev: "2025-01-01T12:00:00Z",
		},
		{
			cron: "0 * * * *",
			now:  "2025-01-01T12:10:00Z",
			next: "2025-01-01T13:00:00Z",
			prev: "2025-01-01T12:00:00Z",
		},
		{
			cron: "0 * * * *",
			now:  "2025-01-01T11:59:00Z",
			next: "2025-01-01T12:00:00Z",
			prev: "2025-01-01T11:00:00Z",
		},
		{
			cron: "lakfkjdf",
			now:  "2025-01-01T11:59:00Z",
			err:  "failed to parse cron spec 'lakfkjdf':",
		},
		{
			cron:     "lakfkjdf",
			timeZone: "UTC",
			now:      "2025-01-01T11:59:00Z",
			err:      "failed to parse cron spec 'lakfkjdf' with timezone 'UTC':",
		},
	} {
		name := fmt.Sprintf("cron=%s,timeZone=%s,now=%s", tt.cron, tt.timeZone, tt.now)
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			now := parseTime(t, tt.now)

			prev, next, err := schedule.GetPrevAndNextTriggers(tt.cron, tt.timeZone, now)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(next).To(Equal(parseTime(t, tt.next)))

			if tt.prev == "" {
				g.Expect(prev).To(BeZero())
			} else {
				g.Expect(prev).NotTo(BeNil())
				g.Expect(prev).To(Equal(parseTime(t, tt.prev)))
			}
		})
	}
}

func TestParse(t *testing.T) {
	now := parseTime(t, "2025-01-01T12:10:00Z")

	for _, tt := range []struct {
		spec     string
		timeZone string
		trigger  string
	}{
		{
			spec:     "0 3 * * *",
			timeZone: "",
			trigger:  "2025-01-02T03:00:00Z",
		},
		{
			spec:     "0 5 * * *",
			timeZone: "",
			trigger:  "2025-01-02T05:00:00Z",
		},
		{
			spec:     "0 5 * * *",
			timeZone: "UTC",
			trigger:  "2025-01-02T05:00:00Z",
		},
		{
			spec:     "0 5 * * *",
			timeZone: "Europe/Bucharest",
			trigger:  "2025-01-02T03:00:00Z",
		},
		{
			spec:     "0 5 * * *",
			timeZone: "Europe/London",
			trigger:  "2025-01-02T05:00:00Z",
		},
		{
			spec:     "0 5 * * *",
			timeZone: "Europe/Dublin",
			trigger:  "2025-01-02T05:00:00Z",
		},
		{
			spec:     "0 5 * * *",
			timeZone: "America/New_York",
			trigger:  "2025-01-02T10:00:00Z",
		},
		{
			spec:     "0 5 * * *",
			timeZone: "America/Sao_Paulo",
			trigger:  "2025-01-02T08:00:00Z",
		},
	} {
		tz := strings.ReplaceAll(tt.timeZone, "/", "_")
		name := fmt.Sprintf("spec=%s,timeZone=%s", tt.spec, tz)
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			s, err := schedule.Parse(tt.spec, tt.timeZone)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(s).NotTo(BeNil())

			trigger := s.Next(now)
			g.Expect(trigger).To(Equal(parseTime(t, tt.trigger)))
		})
	}
}

func parseTime(t *testing.T, s string) time.Time {
	t.Helper()
	g := NewWithT(t)
	tm, err := time.Parse(time.RFC3339, s)
	g.Expect(err).NotTo(HaveOccurred())
	return tm
}
