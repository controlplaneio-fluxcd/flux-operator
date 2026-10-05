// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	toolscache "k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
)

// readyTransitionsWindow is the span over which the Ready
// transitions of a node are counted for flapping detection.
const readyTransitionsWindow = 15 * time.Minute

// ReadyTransitionTracker records the Ready condition transitions of every
// node from the Node informer events. It works independently of the metrics
// collector, so flapping is detected with metrics disabled, and it sees every
// transition instead of sampling the node status on a scrape tick. Each node
// reports counts only after it has been observed for a full window.
// Safe for concurrent use.
type ReadyTransitionTracker struct {
	mu sync.Mutex

	// firstSeen holds the time each node was first observed.
	firstSeen map[string]time.Time

	// transitions holds the observed transition times per node.
	transitions map[string][]time.Time

	// now returns the current time, overridable in tests.
	now func() time.Time
}

// NewReadyTransitionTracker returns an empty tracker.
func NewReadyTransitionTracker() *ReadyTransitionTracker {
	return &ReadyTransitionTracker{
		firstSeen:   make(map[string]time.Time),
		transitions: make(map[string][]time.Time),
		now:         time.Now,
	}
}

// StartReadyTransitionTracker registers a ReadyTransitionTracker as an event
// handler on the Node informer of the given cache.
func StartReadyTransitionTracker(ctx context.Context, c ctrlcache.Cache) (*ReadyTransitionTracker, error) {
	informer, err := c.GetInformer(ctx, &corev1.Node{})
	if err != nil {
		return nil, err
	}
	t := NewReadyTransitionTracker()
	if _, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if node, ok := obj.(*corev1.Node); ok {
				t.Seen(node.Name)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldNode, ok1 := oldObj.(*corev1.Node)
			newNode, ok2 := newObj.(*corev1.Node)
			if ok1 && ok2 {
				t.Observe(oldNode, newNode)
			}
		},
		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			if node, ok := obj.(*corev1.Node); ok {
				t.Forget(node.Name)
			}
		},
	}); err != nil {
		return nil, err
	}
	return t, nil
}

// Seen records the first time a node is observed, which starts its window.
func (t *ReadyTransitionTracker) Seen(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seenLocked(name)
}

// seenLocked implements Seen; the caller must hold t.mu.
func (t *ReadyTransitionTracker) seenLocked(name string) {
	if _, ok := t.firstSeen[name]; !ok {
		t.firstSeen[name] = t.now()
	}
}

// Observe records the Ready transitions between two versions of a node.
// A status change counts as one transition. When the status is unchanged
// but the lastTransitionTime moved, the node went away and back between
// the two versions (e.g. events coalesced by a relist), which counts as two.
func (t *ReadyTransitionTracker) Observe(oldNode, newNode *corev1.Node) {
	oldReady := nodeCondition(oldNode, corev1.NodeReady)
	newReady := nodeCondition(newNode, corev1.NodeReady)
	if newReady == nil {
		return
	}

	var count int
	switch {
	case oldReady == nil:
		return
	case oldReady.Status != newReady.Status:
		count = 1
	case !oldReady.LastTransitionTime.Equal(&newReady.LastTransitionTime):
		count = 2
	default:
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.seenLocked(newNode.Name)
	now := t.now()
	for range count {
		t.transitions[newNode.Name] = append(t.transitions[newNode.Name], now)
	}
	t.pruneLocked(newNode.Name, now)
}

// Forget drops the history of a deleted node.
func (t *ReadyTransitionTracker) Forget(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.transitions, name)
	delete(t.firstSeen, name)
}

// Transitions returns the number of Ready transitions of the node in the
// last 15 minutes, or nil until the node has been observed for a full
// window or when the tracker is nil.
func (t *ReadyTransitionTracker) Transitions(name string) *int {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	first, ok := t.firstSeen[name]
	if !ok || now.Sub(first) < readyTransitionsWindow {
		return nil
	}
	t.pruneLocked(name, now)
	n := len(t.transitions[name])
	return &n
}

// pruneLocked drops the transitions older than the window;
// the caller must hold t.mu.
func (t *ReadyTransitionTracker) pruneLocked(name string, now time.Time) {
	list := t.transitions[name]
	cutoff := now.Add(-readyTransitionsWindow)
	i := 0
	for i < len(list) && list[i].Before(cutoff) {
		i++
	}
	if i == len(list) {
		delete(t.transitions, name)
		return
	}
	t.transitions[name] = list[i:]
}
