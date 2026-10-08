// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReadyTransitionTracker(t *testing.T) {
	g := NewWithT(t)

	start := time.Now()
	clock := start
	tracker := NewReadyTransitionTracker()
	tracker.now = func() time.Time { return clock }

	node := func(name string, status corev1.ConditionStatus, since time.Time) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
				Type: corev1.NodeReady, Status: status, LastTransitionTime: metav1.NewTime(since),
			}}},
		}
	}

	// node-1 is seen at start, node-2 five minutes later.
	tracker.Seen("node-1")

	// Two observed transitions before the window is complete.
	ready := node("node-1", corev1.ConditionTrue, start.Add(-time.Hour))
	notReady := node("node-1", corev1.ConditionFalse, start)
	tracker.Observe(ready, notReady)
	clock = start.Add(time.Minute)
	readyAgain := node("node-1", corev1.ConditionTrue, clock)
	tracker.Observe(notReady, readyAgain)

	// Unchanged status and transition time is not a transition.
	tracker.Observe(readyAgain, readyAgain)

	clock = start.Add(5 * time.Minute)
	tracker.Seen("node-2")
	tracker.Seen("node-1") // already seen, the window start is kept

	// Nothing is reported until the node was observed for 15m.
	g.Expect(tracker.Transitions("node-1")).To(BeNil())

	clock = start.Add(14 * time.Minute)
	// The status is unchanged but the transition time moved:
	// the node went away and back in between, two transitions.
	tracker.Observe(readyAgain, node("node-1", corev1.ConditionTrue, clock))

	clock = start.Add(15 * time.Minute)
	g.Expect(*tracker.Transitions("node-1")).To(Equal(4))
	g.Expect(tracker.Transitions("node-2")).To(BeNil())
	g.Expect(tracker.Transitions("unseen")).To(BeNil())

	// Transitions older than 15m are pruned.
	clock = start.Add(16*time.Minute + time.Second)
	g.Expect(*tracker.Transitions("node-1")).To(Equal(2))

	clock = start.Add(20 * time.Minute)
	g.Expect(*tracker.Transitions("node-2")).To(BeZero())

	// A deleted node starts over.
	tracker.Forget("node-1")
	g.Expect(tracker.Transitions("node-1")).To(BeNil())

	// A nil tracker reports nothing.
	var nilTracker *ReadyTransitionTracker
	g.Expect(nilTracker.Transitions("node-1")).To(BeNil())
}

func TestStartReadyTransitionTracker(t *testing.T) {
	g := NewWithT(t)

	tracker, err := StartReadyTransitionTracker(ctx, testCluster.GetCache())
	g.Expect(err).NotTo(HaveOccurred())

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "tracker-node"}}
	g.Expect(testClient.Create(ctx, node)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(ctx, node) })

	setReady := func(status corev1.ConditionStatus) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		node.Status.Conditions = []corev1.NodeCondition{{
			Type: corev1.NodeReady, Status: status,
			LastTransitionTime: metav1.Now(), LastHeartbeatTime: metav1.Now(),
		}}
		g.Expect(testClient.Status().Update(ctx, node)).To(Succeed())
	}
	setReady(corev1.ConditionTrue)
	g.Eventually(func() bool {
		var cached corev1.Node
		err := testCluster.GetClient().Get(ctx, client.ObjectKeyFromObject(node), &cached)
		return err == nil && len(cached.Status.Conditions) > 0
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
	setReady(corev1.ConditionFalse)

	g.Eventually(func() bool {
		tracker.mu.Lock()
		defer tracker.mu.Unlock()
		return len(tracker.transitions["tracker-node"]) == 1
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
	g.Expect(tracker.Transitions("tracker-node")).To(BeNil())

	// The node is reported once observed for 15m.
	tracker.mu.Lock()
	tracker.firstSeen["tracker-node"] = time.Now().Add(-readyTransitionsWindow)
	tracker.mu.Unlock()
	g.Expect(*tracker.Transitions("tracker-node")).To(Equal(1))
}
