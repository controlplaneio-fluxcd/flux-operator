// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Event is a lighter representation of a Kubernetes event.
type Event struct {
	LastTimestamp metav1.Time `json:"lastTimestamp"`
	Type          string      `json:"type"`
	Message       string      `json:"message"`
}

// GetEvents retrieves events for a specific resource kind and name in the given namespace.
func (k *Client) GetEvents(ctx context.Context, kind, name, namespace string, excludeReason string) ([]Event, error) {
	el := &corev1.EventList{}

	selectors := []fields.Selector{
		fields.OneTermEqualSelector("involvedObject.kind", kind),
		fields.OneTermEqualSelector("involvedObject.name", name),
	}

	if excludeReason != "" {
		selectors = append(selectors, fields.OneTermNotEqualSelector("reason", excludeReason))
	}

	listOpts := []ctrlclient.ListOption{
		ctrlclient.Limit(100),
		ctrlclient.InNamespace(namespace),
		ctrlclient.MatchingFieldsSelector{
			Selector: fields.AndSelectors(selectors...),
		}}
	err := k.Client.List(ctx, el, listOpts...)
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

const (
	// eventListPageSize is the number of events requested from the API per page.
	eventListPageSize int64 = 500
	// eventListHardCap bounds the number of events inspected by a single request.
	eventListHardCap = 5000
	// defaultEventLimit is the number of matched events returned when no limit is specified.
	defaultEventLimit = 100
)

// ListEventsOptions defines filters and limits for listing Kubernetes events.
type ListEventsOptions struct {
	// Namespace restricts events to the namespace of their involved objects.
	Namespace string
	// APIVersion filters by the exact involved object API version.
	APIVersion string
	// Kind filters by the exact involved object kind.
	Kind string
	// Name filters by the exact involved object name.
	Name string
	// Type filters by the exact Kubernetes event type.
	Type string
	// Since keeps events newer than this duration when set.
	Since *time.Duration
	// Grep filters event reasons, messages, and object references.
	Grep *regexp.Regexp
	// Limit is the maximum number of matched events to return.
	Limit int
}

// ListEventsResult contains filtered Kubernetes events and truncation metadata.
type ListEventsResult struct {
	// Namespace is the requested namespace and is omitted for cluster-wide lists.
	Namespace string `json:"namespace,omitempty"`
	// Total is the number of matched events before applying Limit.
	Total int `json:"total"`
	// Truncated reports whether the hard cap or Limit dropped events.
	Truncated bool `json:"truncated"`
	// Events contains one line per matched event in newest-first order.
	Events string `json:"events"`
}

// eventWorkloadKinds are the kinds whose events are listed together with the
// events of the ReplicaSets, Jobs and pods they own.
var eventWorkloadKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"CronJob":     true,
	"Job":         true,
}

// eventTarget identifies one involved object to list events for.
type eventTarget struct {
	kind string
	name string
}

// ListEvents retrieves Kubernetes events across the cluster, in a namespace,
// or for a workload together with the objects it owns.
func (k *Client) ListEvents(ctx context.Context, opts ListEventsOptions) (*ListEventsResult, error) {
	var clientset kubernetes.Interface
	if isWorkloadEventQuery(opts) {
		cs, err := kubernetes.NewForConfig(k.cfg)
		if err != nil {
			return nil, err
		}
		clientset = cs
	}
	return listEvents(ctx, k.Client, clientset, opts)
}

// isWorkloadEventQuery reports whether the options name a workload whose
// owned objects must be resolved.
func isWorkloadEventQuery(opts ListEventsOptions) bool {
	return eventWorkloadKinds[opts.Kind] && opts.Name != "" && opts.Namespace != ""
}

// listEvents implements ListEvents with an injectable clientset for unit tests.
func listEvents(ctx context.Context, c ctrlclient.Client, clientset kubernetes.Interface, opts ListEventsOptions) (*ListEventsResult, error) {
	var typeSelector []fields.Selector
	if opts.Type != "" {
		typeSelector = append(typeSelector, fields.OneTermEqualSelector("type", opts.Type))
	}

	var allEvents []corev1.Event
	truncated := false
	if isWorkloadEventQuery(opts) {
		targets, podsTruncated, err := resolveEventTargets(ctx, clientset, opts.Kind, opts.Name, opts.Namespace)
		if err != nil {
			return nil, err
		}
		truncated = podsTruncated
		for i, target := range targets {
			selectors := append(slices.Clone(typeSelector),
				fields.OneTermEqualSelector("involvedObject.kind", target.kind),
				fields.OneTermEqualSelector("involvedObject.name", target.name))
			if i == 0 && opts.APIVersion != "" {
				selectors = append(selectors, fields.OneTermEqualSelector("involvedObject.apiVersion", opts.APIVersion))
			}
			events, capped, err := listEventPages(ctx, c, opts.Namespace, selectors, eventListHardCap-len(allEvents))
			if err != nil {
				return nil, err
			}
			allEvents = append(allEvents, events...)
			truncated = truncated || capped
			if len(allEvents) >= eventListHardCap {
				break
			}
		}
	} else {
		selectors := slices.Clone(typeSelector)
		if opts.APIVersion != "" {
			selectors = append(selectors, fields.OneTermEqualSelector("involvedObject.apiVersion", opts.APIVersion))
		}
		if opts.Kind != "" {
			selectors = append(selectors, fields.OneTermEqualSelector("involvedObject.kind", opts.Kind))
		}
		if opts.Name != "" {
			selectors = append(selectors, fields.OneTermEqualSelector("involvedObject.name", opts.Name))
		}
		var err error
		allEvents, truncated, err = listEventPages(ctx, c, opts.Namespace, selectors, eventListHardCap)
		if err != nil {
			return nil, err
		}
	}

	cutoff := time.Time{}
	if opts.Since != nil {
		cutoff = time.Now().Add(-*opts.Since)
	}
	filtered := make([]corev1.Event, 0, len(allEvents))
	for _, event := range allEvents {
		if !cutoff.IsZero() && !eventTime(event).After(cutoff) {
			continue
		}
		object := eventObjectRef(event)
		if opts.Grep != nil &&
			!opts.Grep.MatchString(event.Reason) &&
			!opts.Grep.MatchString(event.Message) &&
			!opts.Grep.MatchString(object) {
			continue
		}
		filtered = append(filtered, event)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return eventTime(filtered[i]).After(eventTime(filtered[j]))
	})

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultEventLimit
	}
	result := &ListEventsResult{
		Namespace: opts.Namespace,
		Total:     len(filtered),
		Truncated: truncated || len(filtered) > limit,
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	var events strings.Builder
	for _, event := range filtered {
		events.WriteString(renderEventLine(event))
		events.WriteByte('\n')
	}
	result.Events = events.String()

	return result, nil
}

// listEventPages lists the events matching the selectors, paging with Continue
// up to maxEvents. It reports whether the cap dropped events.
func listEventPages(ctx context.Context, c ctrlclient.Client, namespace string, selectors []fields.Selector, maxEvents int) ([]corev1.Event, bool, error) {
	events := make([]corev1.Event, 0, eventListPageSize)
	continueToken := ""
	for len(events) < maxEvents {
		remaining := maxEvents - len(events)
		pageLimit := min(eventListPageSize, int64(remaining))

		listOpts := []ctrlclient.ListOption{ctrlclient.Limit(pageLimit)}
		if namespace != "" {
			listOpts = append(listOpts, ctrlclient.InNamespace(namespace))
		}
		if len(selectors) > 0 {
			listOpts = append(listOpts, ctrlclient.MatchingFieldsSelector{
				Selector: fields.AndSelectors(selectors...),
			})
		}
		if continueToken != "" {
			listOpts = append(listOpts, ctrlclient.Continue(continueToken))
		}

		page := &corev1.EventList{}
		if err := c.List(ctx, page, listOpts...); err != nil {
			return nil, false, err
		}

		if len(page.Items) > remaining {
			events = append(events, page.Items[:remaining]...)
			return events, true, nil
		}
		events = append(events, page.Items...)
		continueToken = page.Continue
		if continueToken == "" {
			break
		}
	}
	return events, len(events) == maxEvents && continueToken != "", nil
}

// resolveEventTargets returns the workload, the ReplicaSets or Jobs it owns,
// and its newest pods capped at maxLogPods.
func resolveEventTargets(ctx context.Context, clientset kubernetes.Interface, kind, name, namespace string) ([]eventTarget, bool, error) {
	targets := []eventTarget{{kind: kind, name: name}}
	var pods []corev1.Pod

	switch kind {
	case "Deployment":
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, false, err
		}
		selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
		if err != nil {
			return nil, false, err
		}
		replicaSets, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, false, err
		}
		for _, rs := range replicaSets.Items {
			if hasOwnerUID(rs.OwnerReferences, deployment.UID) {
				targets = append(targets, eventTarget{kind: "ReplicaSet", name: rs.Name})
			}
		}
		if pods, err = resolveAppsWorkloadPods(ctx, clientset, deployment, namespace, name); err != nil {
			return nil, false, err
		}
	case "CronJob":
		cronJob, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, false, err
		}
		jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, false, err
		}
		for _, job := range jobs.Items {
			if hasOwnerUID(job.OwnerReferences, cronJob.UID) {
				targets = append(targets, eventTarget{kind: "Job", name: job.Name})
			}
		}
		if pods, err = resolveCronJobPods(ctx, clientset, cronJob); err != nil {
			return nil, false, err
		}
	default:
		var err error
		if pods, err = resolveLogPods(ctx, clientset, kind, name, namespace); err != nil {
			return nil, false, err
		}
	}

	sort.SliceStable(pods, func(i, j int) bool {
		if pods[i].CreationTimestamp.Equal(&pods[j].CreationTimestamp) {
			return pods[i].Name < pods[j].Name
		}
		return pods[i].CreationTimestamp.After(pods[j].CreationTimestamp.Time)
	})
	truncated := len(pods) > maxLogPods
	if truncated {
		pods = pods[:maxLogPods]
	}
	for _, pod := range pods {
		targets = append(targets, eventTarget{kind: "Pod", name: pod.Name})
	}
	return targets, truncated, nil
}

// eventObjectRef renders the involved object as Kind/namespace/name,
// or Kind/name for cluster-scoped objects.
func eventObjectRef(event corev1.Event) string {
	if event.InvolvedObject.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
	}
	return fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name)
}

// renderEventLine formats an event as a compact, single-line MCP result.
func renderEventLine(event corev1.Event) string {
	parts := []string{
		eventTime(event).UTC().Format(time.RFC3339),
		event.Type,
		event.Reason,
		eventObjectRef(event),
	}
	count := event.Count
	if event.Series != nil {
		count = event.Series.Count
	}
	if count > 1 {
		parts = append(parts, fmt.Sprintf("x%d", count))
	}

	if message := strings.Join(strings.Fields(event.Message), " "); message != "" {
		parts = append(parts, message)
	}
	return strings.Join(parts, " ")
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
