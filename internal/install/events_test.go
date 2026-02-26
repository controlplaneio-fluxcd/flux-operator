// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"sort"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSortableEvents_SortByLastTimestamp(t *testing.T) {
	g := NewWithT(t)

	now := time.Now()
	events := SortableEvents{
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "third"},
			LastTimestamp: metav1.NewTime(now.Add(2 * time.Minute)),
		},
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "first"},
			LastTimestamp: metav1.NewTime(now),
		},
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "second"},
			LastTimestamp: metav1.NewTime(now.Add(1 * time.Minute)),
		},
	}

	sort.Sort(events)

	g.Expect(events[0].Name).To(Equal("first"))
	g.Expect(events[1].Name).To(Equal("second"))
	g.Expect(events[2].Name).To(Equal("third"))
}

func TestSortableEvents_SortByEventTime(t *testing.T) {
	g := NewWithT(t)

	now := time.Now()
	events := SortableEvents{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "second"},
			EventTime:  metav1.NewMicroTime(now.Add(1 * time.Minute)),
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "first"},
			EventTime:  metav1.NewMicroTime(now),
		},
	}

	sort.Sort(events)

	g.Expect(events[0].Name).To(Equal("first"))
	g.Expect(events[1].Name).To(Equal("second"))
}

func TestSortableEvents_SortBySeriesTime(t *testing.T) {
	g := NewWithT(t)

	now := time.Now()
	events := SortableEvents{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "second"},
			Series: &corev1.EventSeries{
				LastObservedTime: metav1.NewMicroTime(now.Add(1 * time.Minute)),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "first"},
			Series: &corev1.EventSeries{
				LastObservedTime: metav1.NewMicroTime(now),
			},
		},
	}

	sort.Sort(events)

	g.Expect(events[0].Name).To(Equal("first"))
	g.Expect(events[1].Name).To(Equal("second"))
}

func TestSortableEvents_SeriesPreferredOverLastTimestamp(t *testing.T) {
	g := NewWithT(t)

	now := time.Now()
	events := SortableEvents{
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "second"},
			LastTimestamp: metav1.NewTime(now),
			Series: &corev1.EventSeries{
				LastObservedTime: metav1.NewMicroTime(now.Add(2 * time.Minute)),
			},
		},
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "first"},
			LastTimestamp: metav1.NewTime(now.Add(5 * time.Minute)),
			Series: &corev1.EventSeries{
				LastObservedTime: metav1.NewMicroTime(now.Add(1 * time.Minute)),
			},
		},
	}

	sort.Sort(events)

	// Series time takes priority over LastTimestamp
	g.Expect(events[0].Name).To(Equal("first"))
	g.Expect(events[1].Name).To(Equal("second"))
}

func TestSortableEvents_EmptyList(t *testing.T) {
	g := NewWithT(t)

	events := SortableEvents{}
	sort.Sort(events)
	g.Expect(events).To(BeEmpty())
}

func TestSortableEvents_SingleEvent(t *testing.T) {
	g := NewWithT(t)

	events := SortableEvents{
		{ObjectMeta: metav1.ObjectMeta{Name: "only"}},
	}
	sort.Sort(events)
	g.Expect(events).To(HaveLen(1))
	g.Expect(events[0].Name).To(Equal("only"))
}
