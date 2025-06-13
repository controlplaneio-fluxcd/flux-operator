// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package schedule_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/schedule"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

func TestNewScheduler(t *testing.T) {
	for _, tt := range []struct {
		name      string
		schedules []fluxcdv1.Schedule
		timeout   time.Duration
		err       string
	}{
		{
			name: "no schedules",
		},
		{
			name: "one schedule",
			schedules: []fluxcdv1.Schedule{{
				Cron: "0 0 * * *",
			}},
		},
		{
			name: "two schedules",
			schedules: []fluxcdv1.Schedule{
				{Cron: "0 0 * * *"},
				{Cron: "0 8 * * *"},
			},
		},
		{
			name: "too short window",
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 0 * * *",
				Window: metav1.Duration{Duration: time.Minute},
			}},
			timeout: time.Hour,
			err:     "failed to validate schedule[0]: a non-zero window (1m0s) must always be at least twice the timeout (1h0m0s)",
		},
		{
			name: "negative window",
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 0 * * *",
				Window: metav1.Duration{Duration: -time.Hour},
			}},
			err: "failed to validate schedule[0]: negative window: -1h0m0s",
		},
		{
			name: "parse error",
			schedules: []fluxcdv1.Schedule{{
				Cron: "kljlkajs",
			}},
			err: "failed to validate schedule[0]: failed to parse cron spec 'kljlkajs':",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			s, err := schedule.NewScheduler(tt.schedules, tt.timeout)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(s).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				if len(tt.schedules) == 0 {
					g.Expect(s).To(BeNil())
				} else {
					g.Expect(s).NotTo(BeNil())
				}
			}
		})
	}
}

func TestScheduler_ShouldReconcile(t *testing.T) {
	for _, tt := range []struct {
		name      string
		schedules []fluxcdv1.Schedule
		timeout   time.Duration
		now       string
		expected  bool
	}{
		{
			name:     "no schedules",
			now:      "2025-01-01T12:00:00Z",
			expected: true,
		},
		{
			name: "for zero window we make the best effort, reconcile even long after the trigger",
			schedules: []fluxcdv1.Schedule{{
				Cron: "0 * * * *",
			}},
			now:      "2025-01-01T12:30:00Z",
			expected: true,
		},
		{
			name: "non-zero window, true",
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 * * * *",
				Window: metav1.Duration{Duration: time.Minute},
			}},
			timeout:  10 * time.Second,
			now:      "2025-01-01T12:00:00Z",
			expected: true,
		},
		{
			name: "non-zero window, false",
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 * * * *",
				Window: metav1.Duration{Duration: time.Minute},
			}},
			timeout:  10 * time.Second,
			now:      "2025-01-01T12:00:55Z",
			expected: false,
		},
		{
			name: "two windows, one says reconcile and the other not",
			schedules: []fluxcdv1.Schedule{
				{
					Cron:   "0 * * * *",
					Window: metav1.Duration{Duration: time.Minute},
				},
				{
					Cron:   "0 * * * *",
					Window: metav1.Duration{Duration: 2 * time.Minute},
				},
			},
			timeout:  10 * time.Second,
			now:      "2025-01-01T12:00:55Z",
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			s, err := schedule.NewScheduler(tt.schedules, tt.timeout)
			g.Expect(err).NotTo(HaveOccurred())

			now := testutils.ParseTime(t, tt.now)

			g.Expect(s.ShouldReconcile(now)).To(Equal(tt.expected))
		})
	}
}

func TestScheduler_ShouldScheduleInterval(t *testing.T) {
	for _, tt := range []struct {
		name         string
		schedules    []fluxcdv1.Schedule
		timeout      time.Duration
		nextInterval string
		expected     bool
	}{
		{
			name:         "no schedules",
			nextInterval: "2025-01-01T12:00:00Z",
			expected:     true,
		},
		{
			name: "a zero window does not reconcile on intervals",
			schedules: []fluxcdv1.Schedule{{
				Cron: "0 * * * *",
			}},
			nextInterval: "2025-01-01T12:30:00Z",
			expected:     false,
		},
		{
			name: "non-zero window, true",
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 * * * *",
				Window: metav1.Duration{Duration: time.Minute},
			}},
			timeout:      10 * time.Second,
			nextInterval: "2025-01-01T12:00:00Z",
			expected:     true,
		},
		{
			name: "non-zero window, false",
			schedules: []fluxcdv1.Schedule{{
				Cron:   "0 * * * *",
				Window: metav1.Duration{Duration: time.Minute},
			}},
			timeout:      10 * time.Second,
			nextInterval: "2025-01-01T12:00:55Z",
			expected:     false,
		},
		{
			name: "two windows, one says schedule and the other not",
			schedules: []fluxcdv1.Schedule{
				{
					Cron:   "0 * * * *",
					Window: metav1.Duration{Duration: time.Minute},
				},
				{
					Cron:   "0 * * * *",
					Window: metav1.Duration{Duration: 2 * time.Minute},
				},
			},
			timeout:      10 * time.Second,
			nextInterval: "2025-01-01T12:00:55Z",
			expected:     true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			s, err := schedule.NewScheduler(tt.schedules, tt.timeout)
			g.Expect(err).NotTo(HaveOccurred())

			nextInterval := testutils.ParseTime(t, tt.nextInterval)

			g.Expect(s.ShouldScheduleInterval(nextInterval)).To(Equal(tt.expected))
		})
	}
}

func TestScheduler_Next(t *testing.T) {
	for _, tt := range []struct {
		name      string
		schedules []fluxcdv1.Schedule
		now       string
		expected  *fluxcdv1.NextSchedule
	}{
		{
			name:     "no schedules",
			now:      "2025-01-01T12:00:00Z",
			expected: nil,
		},
		{
			name: "one schedule",
			schedules: []fluxcdv1.Schedule{{
				Cron: "0 3 * * *",
			}},
			now: "2025-01-01T12:00:00Z",
			expected: &fluxcdv1.NextSchedule{
				Schedule: fluxcdv1.Schedule{
					Cron: "0 3 * * *",
				},
				When: metav1.Time{Time: testutils.ParseTime(t, "2025-01-02T03:00:00Z")},
			},
		},
		{
			name: "three schedules, chooses earliest",
			schedules: []fluxcdv1.Schedule{
				{Cron: "0 5 * * *"},
				{Cron: "0 3 * * *"},
				{Cron: "0 7 * * *"},
			},
			now: "2025-01-01T12:00:00Z",
			expected: &fluxcdv1.NextSchedule{
				Schedule: fluxcdv1.Schedule{
					Cron: "0 3 * * *",
				},
				When: metav1.Time{Time: testutils.ParseTime(t, "2025-01-02T03:00:00Z")},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			s, err := schedule.NewScheduler(tt.schedules, time.Minute)
			g.Expect(err).NotTo(HaveOccurred())

			now := testutils.ParseTime(t, tt.now)

			g.Expect(s.Next(now)).To(Equal(tt.expected))
		})
	}
}

func TestParse(t *testing.T) {
	now := testutils.ParseTime(t, "2025-01-01T12:10:00Z")

	for _, tt := range []struct {
		spec     string
		timeZone string
		trigger  string
		err      string
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
		{
			spec:     "kljlkajs",
			timeZone: "",
			trigger:  "2025-01-02T08:00:00Z",
			err:      "failed to parse cron spec 'kljlkajs':",
		},
		{
			spec:     "kljlkajs",
			timeZone: "America/Sao_Paulo",
			trigger:  "2025-01-02T08:00:00Z",
			err:      "failed to parse cron spec 'kljlkajs' with timezone 'America/Sao_Paulo':",
		},
	} {
		tz := strings.ReplaceAll(tt.timeZone, "/", "_")
		name := fmt.Sprintf("spec=%s,timeZone=%s", tt.spec, tz)
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			s, err := schedule.Parse(tt.spec, tt.timeZone)
			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(s).NotTo(BeNil())

			trigger := s.Next(now)
			g.Expect(trigger).To(Equal(testutils.ParseTime(t, tt.trigger)))
		})
	}
}

func TestGetPrevAndNextTriggers(t *testing.T) {
	for _, tt := range []struct {
		name     string
		cron     string
		timeZone string
		now      string
		prev     string
		next     string
	}{
		{
			name: "feb 29, now far apart from prev and next",
			cron: "0 0 29 2 *",
			now:  "2025-01-01T12:00:00Z",
			prev: "2024-02-29T00:00:00Z",
			next: "2028-02-29T00:00:00Z",
		},
		{
			name: "feb 29, now equals prev",
			cron: "0 0 29 2 *",
			now:  "2024-02-29T00:00:00Z",
			prev: "2024-02-29T00:00:00Z",
			next: "2028-02-29T00:00:00Z",
		},
		{
			name: "feb 29, now right before next",
			cron: "0 0 29 2 *",
			now:  "2028-02-28T23:59:59Z",
			prev: "2024-02-29T00:00:00Z",
			next: "2028-02-29T00:00:00Z",
		},
		{
			name: "now equals prev",
			cron: "0 * * * *",
			now:  "2025-01-01T12:00:00Z",
			prev: "2025-01-01T12:00:00Z",
			next: "2025-01-01T13:00:00Z",
		},
		{
			name: "now right after prev",
			cron: "0 * * * *",
			now:  "2025-01-01T12:00:01Z",
			prev: "2025-01-01T12:00:00Z",
			next: "2025-01-01T13:00:00Z",
		},
		{
			name: "now right before next",
			cron: "0 * * * *",
			now:  "2025-01-01T11:59:59Z",
			prev: "2025-01-01T11:00:00Z",
			next: "2025-01-01T12:00:00Z",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			s, err := schedule.Parse(tt.cron, tt.timeZone)
			g.Expect(err).NotTo(HaveOccurred())

			now := testutils.ParseTime(t, tt.now)

			prev, next := schedule.GetPrevAndNextTriggers(s, now)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(prev).To(Equal(testutils.ParseTime(t, tt.prev)))
			g.Expect(next).To(Equal(testutils.ParseTime(t, tt.next)))
		})
	}
}
