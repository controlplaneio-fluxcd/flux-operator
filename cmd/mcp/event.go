// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Event struct {
	LastTimestamp metav1.Time `json:"lastTimestamp"`
	Type          string      `json:"type"`
	Message       string      `json:"message"`
}

func getEvents(ctx context.Context, kubeClient client.Client, kind, name, namespace string) ([]Event, error) {
	el := &corev1.EventList{}
	listOpts := []client.ListOption{
		client.Limit(100),
		client.InNamespace(namespace),
		client.MatchingFieldsSelector{
			Selector: fields.AndSelectors(
				fields.OneTermNotEqualSelector("reason", "ReconciliationSucceeded"),
				fields.OneTermEqualSelector("involvedObject.kind", kind),
				fields.OneTermEqualSelector("involvedObject.name", name),
			),
		}}
	err := kubeClient.List(ctx, el, listOpts...)
	if err != nil {
		return nil, fmt.Errorf("unable to list events: %w", err)
	}

	sort.Sort(SortableEvents(el.Items))

	events := make([]Event, 0, len(el.Items))
	for _, event := range el.Items {
		events = append(events, Event{
			LastTimestamp: event.LastTimestamp,
			Type:          event.Type,
			Message:       event.Message,
		})
	}
	return events, nil
}

// SortableEvents implements sort.Interface for []api.Event by time
type SortableEvents []corev1.Event

func (list SortableEvents) Len() int {
	return len(list)
}

func (list SortableEvents) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// Return the time that should be used for sorting, which can come from
// various places in corev1.Event.
func eventTime(event corev1.Event) time.Time {
	if event.Series != nil {
		return event.Series.LastObservedTime.Time
	}
	if !event.LastTimestamp.Time.IsZero() {
		return event.LastTimestamp.Time
	}
	return event.EventTime.Time
}

func (list SortableEvents) Less(i, j int) bool {
	return eventTime(list[i]).Before(eventTime(list[j]))
}
