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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// EventsHandler handles GET /api/v1/events requests and returns Kubernetes events for Flux resources.
// Supports optional query parameters: kind, name, namespace
// Example: /api/v1/events?kind=FluxInstance&name=flux&namespace=flux-system
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

	// Build kinds array based on query parameter
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	} else {
		// Default kinds
		kinds = []string{
			// Appliers
			fluxcdv1.FluxInstanceKind,
			fluxcdv1.ResourceSetKind,
			fluxcdv1.FluxKustomizationKind,
			fluxcdv1.FluxHelmReleaseKind,
			// Sources
			fluxcdv1.FluxGitRepositoryKind,
			fluxcdv1.FluxOCIRepositoryKind,
			fluxcdv1.FluxHelmChartKind,
			fluxcdv1.FluxArtifactGeneratorKind,
		}
	}

	// Get events from the cluster using the request context
	events, err := r.GetEvents(req.Context(), kinds, name, namespace, "")
	if err != nil {
		r.log.Error(err, "failed to get events", "url", req.URL.String(),
			"kind", kind, "name", name, "namespace", namespace)
		// Return empty array instead of error for better UX
		events = []Event{}
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

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
func (r *Router) GetEvents(ctx context.Context, kinds []string, name, namespace string, excludeReason string) ([]Event, error) {
	var allEvents []corev1.Event

	if len(kinds) == 0 {
		return nil, errors.New("no resource kinds specified")
	}

	// Set limit based on number of kinds
	limit := 500
	if len(kinds) > 1 {
		limit = 100
	}

	// Query events for each kind in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(kinds))

	for _, kind := range kinds {
		wg.Add(1)
		go func(kind string) {
			defer wg.Done()

			el := &corev1.EventList{}

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

			listOpts := []client.ListOption{
				client.Limit(limit),
				client.MatchingFieldsSelector{
					Selector: fields.AndSelectors(selectors...),
				}}
			if namespace != "" {
				listOpts = append(listOpts, client.InNamespace(namespace))
			}

			if err := r.kubeReader.List(ctx, el, listOpts...); err != nil {
				errChan <- fmt.Errorf("unable to list events for kind %s: %w", kind, err)
				return
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

			mu.Lock()
			allEvents = append(allEvents, filteredEvents...)
			mu.Unlock()
		}(kind)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	// Sort all events by timestamp
	sort.Sort(SortableEvents(allEvents))

	// Convert to lighter Event representation
	events := make([]Event, 0, len(allEvents))
	for _, event := range allEvents {
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
