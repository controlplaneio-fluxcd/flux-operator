// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWebConfigSpec_MetricsEnabled(t *testing.T) {
	g := NewWithT(t)

	// Metrics are collected by default.
	g.Expect((*WebConfigSpec)(nil).MetricsEnabled()).To(BeTrue())
	g.Expect((&WebConfigSpec{}).MetricsEnabled()).To(BeTrue())
	g.Expect((&WebConfigSpec{Metrics: &MetricsSpec{}}).MetricsEnabled()).To(BeTrue())

	// Collection can be turned off.
	g.Expect((&WebConfigSpec{Metrics: &MetricsSpec{Disabled: true}}).MetricsEnabled()).To(BeFalse())
}

func TestWebConfigSpec_MetricsScrapeInterval(t *testing.T) {
	g := NewWithT(t)

	interval := func(d time.Duration) *WebConfigSpec {
		return &WebConfigSpec{Metrics: &MetricsSpec{ScrapeInterval: &metav1.Duration{Duration: d}}}
	}

	// Defaults to one minute.
	g.Expect((*WebConfigSpec)(nil).MetricsScrapeInterval()).To(Equal(time.Minute))
	g.Expect((&WebConfigSpec{}).MetricsScrapeInterval()).To(Equal(time.Minute))
	g.Expect((&WebConfigSpec{Metrics: &MetricsSpec{}}).MetricsScrapeInterval()).To(Equal(time.Minute))
	g.Expect(interval(0).MetricsScrapeInterval()).To(Equal(time.Minute))

	// Configured values are honored and clamped to [15s, 10m].
	g.Expect(interval(30 * time.Second).MetricsScrapeInterval()).To(Equal(30 * time.Second))
	g.Expect(interval(5 * time.Second).MetricsScrapeInterval()).To(Equal(15 * time.Second))
	g.Expect(interval(time.Hour).MetricsScrapeInterval()).To(Equal(10 * time.Minute))
}
