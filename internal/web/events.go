// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// EventsHandler handles GET /api/v1/events requests and returns Kubernetes events for Flux resources.
// Supports optional query parameters: kind, name, namespace, type
// Example: /api/v1/events?kind=FluxInstance&name=flux&namespace=flux-system&type=Warning
func (r *Router) EventsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	queryParams := req.URL.Query()
	kind := queryParams.Get("kind")
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")
	eventType := queryParams.Get("type")

	// Get events from the cluster using the request context
	events, err := r.GetEvents(req.Context(), kind, name, namespace, "", eventType)
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to get events")
		// Return empty array instead of error for better UX
		events = []Event{}
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Encode and send the response
	response := map[string]any{"events": events}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Event is a lighter representation of a Kubernetes event.
type Event struct {
	LastTimestamp  metav1.Time `json:"lastTimestamp"`
	Type           string      `json:"type"`
	Message        string      `json:"message"`
	InvolvedObject string      `json:"involvedObject"`
	Namespace      string      `json:"namespace"`
}

// GetEvents retrieves events for the specified resource kinds.
// Returns at most 500 events per kind (100 if multiple kinds are specified), sorted by timestamp descending.
// Filters by eventType (Normal, Warning) if provided.
func (r *Router) GetEvents(ctx context.Context, kind, name, namespace, excludeReason, eventType string) ([]Event, error) {
	// Build kinds array based on query parameter
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	} else {
		// Default kinds
		kinds = []string{
			// Appliers
			fluxcdv1.ResourceSetKind,
			fluxcdv1.FluxKustomizationKind,
			fluxcdv1.FluxHelmReleaseKind,
			// Sources
			fluxcdv1.FluxGitRepositoryKind,
			fluxcdv1.FluxOCIRepositoryKind,
			fluxcdv1.FluxHelmChartKind,
			fluxcdv1.FluxHelmRepositoryKind,
			fluxcdv1.FluxBucketKind,
			fluxcdv1.FluxArtifactGeneratorKind,
			fluxcdv1.ResourceSetInputProviderKind,
		}
	}

	// Prepare list of namespaces to search in
	var namespaces []string
	if namespace != "" {
		namespaces = []string{namespace}
	} else {
		// Check if the user has access to all namespaces
		userNamespaces, all, err := r.kubeClient.ListUserNamespaces(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list user namespaces: %w", err)
		}

		// If the user has no access to any namespaces, return empty result
		if len(userNamespaces) == 0 {
			return []Event{}, nil
		}

		// If the user has cluster-wide access, we can add FluxInstance to kinds
		if all && kind == "" {
			kinds = append(kinds, fluxcdv1.FluxInstanceKind)
		}

		// If the user does not have access to all namespaces, limit search to their namespaces
		if !all {
			namespaces = userNamespaces
		}
	}

	var allEvents []corev1.Event

	if len(kinds) == 0 {
		return nil, errors.New("no resource kinds specified")
	}

	// Set limit based on number of kinds
	limit := 1000
	if len(kinds) > 1 {
		limit = 500
	}

	// Query events for each kind in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, kind := range kinds {
		wg.Add(1)
		go func(kind string) {
			defer wg.Done()

			selectors := []fields.Selector{
				fields.OneTermEqualSelector("involvedObject.kind", kind),
			}

			// Add name filter if provided and doesn't contain wildcards
			// For exact match, use field selector (faster)
			if name != "" && !hasWildcard(name) {
				selectors = append(selectors, fields.OneTermEqualSelector("involvedObject.name", name))
			}

			// Add excludeReason filter if provided
			if excludeReason != "" {
				selectors = append(selectors, fields.OneTermNotEqualSelector("reason", excludeReason))
			}

			// Determine which namespaces to query.
			// If namespaces is empty, query all namespaces (cluster-wide access).
			// Otherwise, query each namespace in the list.
			namespacesToQuery := namespaces
			if len(namespacesToQuery) == 0 {
				namespacesToQuery = []string{""}
			}

			var byKindEvents []corev1.Event
			for _, ns := range namespacesToQuery {
				el := &corev1.EventList{}

				listOpts := []client.ListOption{
					client.Limit(limit),
					client.MatchingFieldsSelector{
						Selector: fields.AndSelectors(selectors...),
					}}
				if ns != "" {
					listOpts = append(listOpts, client.InNamespace(ns))
				}

				if err := r.kubeClient.GetAPIReader(ctx).List(ctx, el, listOpts...); err != nil {
					if !apierrors.IsForbidden(err) {
						log.FromContext(ctx).Error(err, "failed to list events for user",
							"kind", kind,
							"namespace", ns)
					}
					continue
				}

				// Filter by name using wildcard matching if needed
				filteredEvents := el.Items
				if hasWildcard(name) {
					filteredEvents = []corev1.Event{}
					for _, event := range el.Items {
						if matchesWildcard(event.InvolvedObject.Name, name) {
							filteredEvents = append(filteredEvents, event)
						}
					}
				}

				byKindEvents = append(byKindEvents, filteredEvents...)
			}

			mu.Lock()
			allEvents = append(allEvents, byKindEvents...)
			mu.Unlock()
		}(kind)
	}

	wg.Wait()

	// Sort all events by timestamp
	sort.Sort(SortableEvents(allEvents))

	// Filter by event type if specified
	filteredEvents := allEvents
	if eventType != "" {
		filteredEvents = make([]corev1.Event, 0, len(allEvents))
		for _, event := range allEvents {
			if event.Type == eventType {
				filteredEvents = append(filteredEvents, event)
			}
		}
	}

	// Convert to lighter Event representation
	events := make([]Event, 0, len(filteredEvents))
	for _, event := range filteredEvents {
		events = append(events, Event{
			LastTimestamp: event.LastTimestamp,
			Type:          event.Type,
			Message:       event.Message,
			InvolvedObject: fmt.Sprintf("%s/%s",
				event.InvolvedObject.Kind,
				event.InvolvedObject.Name),
			Namespace: event.InvolvedObject.Namespace,
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
	return eventTime(list[i]).After(eventTime(list[j]))
}
