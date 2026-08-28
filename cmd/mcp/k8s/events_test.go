// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestGetEvents(t *testing.T) {
	mockEventList := &corev1.EventList{
		Items: []corev1.Event{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event1",
					Namespace: "flux-system",
				},
				InvolvedObject: corev1.ObjectReference{
					APIVersion: "fluxcd.controlplane.io/v1",
					Kind:       "FluxInstance",
					Name:       "flux",
					Namespace:  "flux-system",
				},
				Type:    corev1.EventTypeWarning,
				Reason:  "ReconciliationFailed",
				Message: "Reconciliation failed with unknown version",
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event2",
					Namespace: "flux-system",
				},
				InvolvedObject: corev1.ObjectReference{
					APIVersion: "fluxcd.controlplane.io/v1",
					Kind:       "FluxInstance",
					Name:       "flux",
					Namespace:  "flux-system",
				},
				Type:    corev1.EventTypeWarning,
				Reason:  "ReconciliationFailed",
				Message: "Reconciliation failed with unknown distribution",
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event3",
					Namespace: "flux-system",
				},
				InvolvedObject: corev1.ObjectReference{
					APIVersion: "fluxcd.controlplane.io/v1",
					Kind:       "ResourceSet",
					Name:       "infra",
					Namespace:  "flux-system",
				},
				Type:    corev1.EventTypeNormal,
				Reason:  "ReconciliationSucceeded",
				Message: "Reconciliation succeeded with version 1.0.0",
			},
		},
	}

	objKindIndexer := func(obj client.Object) []string {
		e, _ := obj.(*corev1.Event)
		return []string{e.InvolvedObject.Kind}
	}

	objNameIndexer := func(obj client.Object) []string {
		e, _ := obj.(*corev1.Event)
		return []string{e.InvolvedObject.Name}
	}

	reasonIndexer := func(obj client.Object) []string {
		e, _ := obj.(*corev1.Event)
		return []string{e.Reason}
	}

	kubeClient := Client{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithLists(mockEventList).
			WithIndex(&corev1.Event{}, "involvedObject.kind", objKindIndexer).
			WithIndex(&corev1.Event{}, "involvedObject.name", objNameIndexer).
			WithIndex(&corev1.Event{}, "reason", reasonIndexer).
			Build(),
	}

	tests := []struct {
		testName    string
		matchLen    int
		matchResult string

		kind      string
		name      string
		namespace string
	}{
		{
			testName:    "match single event",
			matchResult: "Reconciliation succeeded",
			matchLen:    1,

			kind:      "ResourceSet",
			name:      "infra",
			namespace: "flux-system",
		},
		{
			testName:    "match multiple events",
			matchResult: "Reconciliation failed",
			matchLen:    2,

			kind:      "FluxInstance",
			name:      "flux",
			namespace: "flux-system",
		},
		{
			testName: "match no events",
			matchLen: 0,

			kind:      "FluxInstance",
			name:      "flux1",
			namespace: "flux-system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			events, err := kubeClient.GetEvents(
				context.Background(),
				tt.kind,
				tt.name,
				tt.namespace,
				"",
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(events).To(HaveLen(tt.matchLen))
			for _, event := range events {
				g.Expect(event.Message).To(ContainSubstring(tt.matchResult))
			}
		})
	}
}

func TestRenderEventLine(t *testing.T) {
	timestamp := time.Date(2026, time.August, 28, 13, 15, 2, 987654321, time.FixedZone("UTC+3", 3*60*60))
	tests := []struct {
		name  string
		event corev1.Event
		want  string
	}{
		{
			name: "namespaced warning with repeated count and multiline message",
			event: corev1.Event{
				InvolvedObject: corev1.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "apps", Name: "backend"},
				Type:           corev1.EventTypeWarning,
				Reason:         "BackOff",
				Message:        "Back-off restarting failed container\nbackend in pod backend_apps   \t",
				Count:          17,
				LastTimestamp:  metav1.NewTime(timestamp),
			},
			want: "2026-08-28T10:15:02Z Warning BackOff Pod/apps/backend x17 Back-off restarting failed container backend in pod backend_apps",
		},
		{
			name: "cluster-scoped object with single count",
			event: corev1.Event{
				InvolvedObject: corev1.ObjectReference{APIVersion: "v1", Kind: "Node", Name: "worker-1"},
				Type:           corev1.EventTypeNormal,
				Reason:         "RegisteredNode",
				Message:        "Node registered",
				Count:          1,
				LastTimestamp:  metav1.NewTime(timestamp),
			},
			want: "2026-08-28T10:15:02Z Normal RegisteredNode Node/worker-1 Node registered",
		},
		{
			name: "series count and timestamp take precedence",
			event: corev1.Event{
				InvolvedObject: corev1.ObjectReference{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "apps", Name: "backend"},
				Type:           corev1.EventTypeNormal,
				Reason:         "ScalingReplicaSet",
				Message:        "Scaled up replica set",
				Count:          1,
				Series: &corev1.EventSeries{
					Count:            3,
					LastObservedTime: metav1.MicroTime{Time: timestamp},
				},
			},
			want: "2026-08-28T10:15:02Z Normal ScalingReplicaSet Deployment/apps/backend x3 Scaled up replica set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(renderEventLine(tt.event)).To(Equal(tt.want))
		})
	}
}

func TestListEvents(t *testing.T) {
	now := time.Now().UTC()
	event := func(name, namespace, apiVersion, kind, objectName, eventType, reason, message string, age time.Duration) corev1.Event {
		return corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			InvolvedObject: corev1.ObjectReference{
				APIVersion: apiVersion,
				Kind:       kind,
				Name:       objectName,
				Namespace:  namespace,
			},
			Type:          eventType,
			Reason:        reason,
			Message:       message,
			Count:         1,
			LastTimestamp: metav1.NewTime(now.Add(-age)),
		}
	}

	backOff := event("backoff", "apps", "v1", "Pod", "backend", corev1.EventTypeWarning,
		"BackOff", "Back-off restarting failed container", time.Minute)
	backOff.Count = 17
	deployment := event("deployment", "apps", "apps/v1", "Deployment", "backend", corev1.EventTypeNormal,
		"Created", "Created replica set", 2*time.Minute)
	deployment.LastTimestamp = metav1.Time{}
	deployment.Series = &corev1.EventSeries{
		Count:            3,
		LastObservedTime: metav1.MicroTime{Time: now.Add(-2 * time.Minute)},
	}
	failedMount := event("mount", "apps", "v1", "PersistentVolumeClaim", "data", corev1.EventTypeWarning,
		"FailedMount", "Unable to attach volume", 3*time.Minute)
	failedMount.LastTimestamp = metav1.Time{}
	failedMount.EventTime = metav1.MicroTime{Time: now.Add(-3 * time.Minute)}
	failedScheduling := event("scheduling", "other", "v1", "Pod", "worker", corev1.EventTypeWarning,
		"FailedScheduling", "No nodes are available", 4*time.Minute)
	oomKilled := event("oom", "apps", "v1", "Pod", "frontend", corev1.EventTypeWarning,
		"ContainerRestart", "Container terminated with OOMKilled", 5*time.Minute)
	old := event("old", "apps", "v1", "Pod", "legacy", corev1.EventTypeWarning,
		"Unhealthy", "Readiness probe failed", 3*time.Hour)

	mockEventList := &corev1.EventList{Items: []corev1.Event{
		old, failedScheduling, oomKilled, failedMount, deployment, backOff,
	}}
	kubeClient := Client{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithLists(mockEventList).
			WithIndex(&corev1.Event{}, "involvedObject.apiVersion", func(obj client.Object) []string {
				return []string{obj.(*corev1.Event).InvolvedObject.APIVersion}
			}).
			WithIndex(&corev1.Event{}, "involvedObject.kind", func(obj client.Object) []string {
				return []string{obj.(*corev1.Event).InvolvedObject.Kind}
			}).
			WithIndex(&corev1.Event{}, "involvedObject.name", func(obj client.Object) []string {
				return []string{obj.(*corev1.Event).InvolvedObject.Name}
			}).
			WithIndex(&corev1.Event{}, "type", func(obj client.Object) []string {
				return []string{obj.(*corev1.Event).Type}
			}).
			Build(),
	}

	sinceWindow := 30 * time.Minute
	zeroWindow := time.Duration(0)
	tests := []struct {
		name          string
		opts          ListEventsOptions
		wantTotal     int
		wantLines     []string
		wantNamespace string
		wantTruncated bool
	}{
		{
			name:          "scopes to namespace",
			opts:          ListEventsOptions{Namespace: "apps", Limit: 100},
			wantTotal:     5,
			wantNamespace: "apps",
		},
		{
			name:      "selects apiVersion kind and name",
			opts:      ListEventsOptions{APIVersion: "apps/v1", Kind: "Deployment", Name: "backend"},
			wantTotal: 1,
			wantLines: []string{renderEventLine(deployment)},
		},
		{
			name:          "filters type",
			opts:          ListEventsOptions{Namespace: "apps", Type: corev1.EventTypeNormal},
			wantTotal:     1,
			wantNamespace: "apps",
		},
		{
			name:      "filters since window",
			opts:      ListEventsOptions{Since: &sinceWindow},
			wantTotal: 5,
		},
		{
			name:      "applies explicit zero since window",
			opts:      ListEventsOptions{Since: &zeroWindow},
			wantTotal: 0,
		},
		{
			name:      "greps reason case insensitively",
			opts:      ListEventsOptions{Grep: regexp.MustCompile("(?i)backoff")},
			wantTotal: 1,
			wantLines: []string{renderEventLine(backOff)},
		},
		{
			name:      "greps message case insensitively",
			opts:      ListEventsOptions{Grep: regexp.MustCompile("(?i)oomkilled")},
			wantTotal: 1,
			wantLines: []string{renderEventLine(oomKilled)},
		},
		{
			name:      "greps object case insensitively",
			opts:      ListEventsOptions{Grep: regexp.MustCompile("(?i)^persistentvolumeclaim/apps/DATA$")},
			wantTotal: 1,
			wantLines: []string{renderEventLine(failedMount)},
		},
		{
			name:          "sorts newest first and applies limit",
			opts:          ListEventsOptions{Namespace: "apps", Limit: 2},
			wantTotal:     5,
			wantLines:     []string{renderEventLine(backOff), renderEventLine(deployment)},
			wantNamespace: "apps",
			wantTruncated: true,
		},
		{
			name:      "returns empty result",
			opts:      ListEventsOptions{Name: "missing"},
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result, err := kubeClient.ListEvents(context.Background(), tt.opts)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Total).To(Equal(tt.wantTotal))
			g.Expect(result.Namespace).To(Equal(tt.wantNamespace))
			g.Expect(result.Truncated).To(Equal(tt.wantTruncated))
			if tt.wantLines != nil {
				lines := strings.Split(strings.TrimSuffix(result.Events, "\n"), "\n")
				g.Expect(lines).To(Equal(tt.wantLines))
			}
			if tt.wantTotal == 0 {
				g.Expect(result.Events).To(BeEmpty())
			}
		})
	}
}

// pagedEventLister serves synthetic events in pages through a List interceptor,
// honoring the requested Limit and Continue token.
func pagedEventLister(total int, reason string) func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
	return func(_ context.Context, _ client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		listOpts := &client.ListOptions{}
		listOpts.ApplyOptions(opts)
		start := 0
		if listOpts.Continue != "" {
			start, _ = strconv.Atoi(listOpts.Continue)
		}
		end := min(start+int(listOpts.Limit), total)

		events := list.(*corev1.EventList)
		events.Items = events.Items[:0]
		for i := start; i < end; i++ {
			events.Items = append(events.Items, corev1.Event{
				ObjectMeta:     metav1.ObjectMeta{Name: fmt.Sprintf("event-%d", i), Namespace: "apps"},
				InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "apps", Name: fmt.Sprintf("pod-%d", i)},
				Type:           corev1.EventTypeWarning,
				Reason:         reason,
				LastTimestamp:  metav1.NewTime(time.Unix(int64(i), 0)),
			})
		}
		events.Continue = ""
		if end < total {
			events.Continue = strconv.Itoa(end)
		}
		return nil
	}
}

func TestListEventsPagination(t *testing.T) {
	newClient := func(total int) *Client {
		return &Client{Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithInterceptorFuncs(interceptor.Funcs{List: pagedEventLister(total, "BackOff")}).
			Build()}
	}

	t.Run("collects every page below the cap", func(t *testing.T) {
		g := NewWithT(t)
		total := int(eventListPageSize)*2 + 7
		result, err := newClient(total).ListEvents(context.Background(), ListEventsOptions{Namespace: "apps", Limit: total})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Total).To(Equal(total))
		g.Expect(result.Truncated).To(BeFalse())
		g.Expect(result.Events).To(HavePrefix(fmt.Sprintf("1970-01-01T00:%02d:%02dZ Warning BackOff Pod/apps/pod-%d", (total-1)/60, (total-1)%60, total-1)))
	})

	t.Run("stops at the cap and reports truncation", func(t *testing.T) {
		g := NewWithT(t)
		result, err := newClient(eventListHardCap+1).ListEvents(context.Background(), ListEventsOptions{Namespace: "apps", Limit: 1})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Total).To(Equal(eventListHardCap))
		g.Expect(result.Truncated).To(BeTrue())
		g.Expect(strings.Count(result.Events, "\n")).To(Equal(1))
	})

	t.Run("keeps truncation when no inspected event matches", func(t *testing.T) {
		g := NewWithT(t)
		result, err := newClient(eventListHardCap+1).ListEvents(context.Background(),
			ListEventsOptions{Namespace: "apps", Grep: regexp.MustCompile("nomatch")})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Total).To(BeZero())
		g.Expect(result.Truncated).To(BeTrue())
		g.Expect(result.Events).To(BeEmpty())
	})
}

// newEventsClient builds a fake controller-runtime client with the field
// indexes used by the event selectors.
func newEventsClient(events ...corev1.Event) client.Client {
	return fake.NewClientBuilder().
		WithScheme(NewTestScheme()).
		WithLists(&corev1.EventList{Items: events}).
		WithIndex(&corev1.Event{}, "involvedObject.apiVersion", func(obj client.Object) []string {
			return []string{obj.(*corev1.Event).InvolvedObject.APIVersion}
		}).
		WithIndex(&corev1.Event{}, "involvedObject.kind", func(obj client.Object) []string {
			return []string{obj.(*corev1.Event).InvolvedObject.Kind}
		}).
		WithIndex(&corev1.Event{}, "involvedObject.name", func(obj client.Object) []string {
			return []string{obj.(*corev1.Event).InvolvedObject.Name}
		}).
		WithIndex(&corev1.Event{}, "type", func(obj client.Object) []string {
			return []string{obj.(*corev1.Event).Type}
		}).
		Build()
}

func TestListEventsWorkload(t *testing.T) {
	now := time.Now().UTC()
	event := func(name, namespace, apiVersion, kind, objectName, eventType, reason, message string, age time.Duration) corev1.Event {
		return corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			InvolvedObject: corev1.ObjectReference{
				APIVersion: apiVersion,
				Kind:       kind,
				Name:       objectName,
				Namespace:  namespace,
			},
			Type:          eventType,
			Reason:        reason,
			Message:       message,
			Count:         1,
			LastTimestamp: metav1.NewTime(now.Add(-age)),
		}
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "apps", UID: types.UID("deploy-uid")},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend"}}},
	}
	ownedRS := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Name: "backend-abc", Namespace: "apps", Labels: map[string]string{"app": "backend"},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "backend", UID: "deploy-uid"}},
	}}
	foreignRS := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Name: "other-abc", Namespace: "apps", Labels: map[string]string{"app": "backend"},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "other", UID: "other-uid"}},
	}}
	podNew := testLogPod("backend-abc-new", "apps", now, map[string]string{"app": "backend"},
		metav1.OwnerReference{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "backend-abc"}, "app")
	podOld := testLogPod("backend-abc-old", "apps", now.Add(-time.Minute), map[string]string{"app": "backend"},
		metav1.OwnerReference{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "backend-abc"}, "app")
	clientset := k8sfake.NewSimpleClientset(deployment, ownedRS, foreignRS, podNew, podOld)

	deployEvent := event("d", "apps", "apps/v1", "Deployment", "backend", corev1.EventTypeNormal,
		"ScalingReplicaSet", "Scaled up replica set backend-abc to 2", 5*time.Minute)
	rsEvent := event("rs", "apps", "apps/v1", "ReplicaSet", "backend-abc", corev1.EventTypeWarning,
		"FailedCreate", "Error creating: pods \"backend-abc-\" is forbidden: exceeded quota", 4*time.Minute)
	podEvent := event("p", "apps", "v1", "Pod", "backend-abc-new", corev1.EventTypeWarning,
		"BackOff", "Back-off restarting failed container", time.Minute)
	oldPodEvent := event("po", "apps", "v1", "Pod", "backend-abc-old", corev1.EventTypeWarning,
		"Unhealthy", "Readiness probe failed", 2*time.Minute)
	foreignRSEvent := event("frs", "apps", "apps/v1", "ReplicaSet", "other-abc", corev1.EventTypeWarning,
		"FailedCreate", "unrelated", time.Minute)
	foreignPodEvent := event("fp", "apps", "v1", "Pod", "other-abc-xyz", corev1.EventTypeWarning,
		"BackOff", "unrelated", time.Minute)
	kubeClient := newEventsClient(deployEvent, rsEvent, podEvent, oldPodEvent, foreignRSEvent, foreignPodEvent)

	t.Run("lists the workload, its replicasets and its pods", func(t *testing.T) {
		g := NewWithT(t)
		result, err := listEvents(context.Background(), kubeClient, clientset,
			ListEventsOptions{Kind: "Deployment", Name: "backend", Namespace: "apps"})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Total).To(Equal(4))
		g.Expect(result.Truncated).To(BeFalse())
		g.Expect(strings.Split(strings.TrimSpace(result.Events), "\n")).To(Equal([]string{
			renderEventLine(podEvent), renderEventLine(oldPodEvent), renderEventLine(rsEvent), renderEventLine(deployEvent),
		}))
	})

	t.Run("applies the type selector to every owned object", func(t *testing.T) {
		g := NewWithT(t)
		result, err := listEvents(context.Background(), kubeClient, clientset,
			ListEventsOptions{Kind: "Deployment", Name: "backend", Namespace: "apps", Type: corev1.EventTypeWarning})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Total).To(Equal(3))
		g.Expect(result.Events).NotTo(ContainSubstring("ScalingReplicaSet"))
	})

	t.Run("fails when the workload is missing", func(t *testing.T) {
		g := NewWithT(t)
		_, err := listEvents(context.Background(), kubeClient, clientset,
			ListEventsOptions{Kind: "Deployment", Name: "missing", Namespace: "apps"})
		g.Expect(err).To(HaveOccurred())
	})
}

func TestResolveEventTargetsCapsPods(t *testing.T) {
	g := NewWithT(t)
	now := time.Now()
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "apps"},
		Spec:       appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}}},
	}
	objects := []runtime.Object{statefulSet}
	for i := 0; i < maxLogPods+2; i++ {
		objects = append(objects, testLogPod(fmt.Sprintf("db-%d", i), "apps", now.Add(time.Duration(i)*time.Second),
			map[string]string{"app": "db"}, metav1.OwnerReference{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "db"}, "db"))
	}

	targets, truncated, err := resolveEventTargets(context.Background(), k8sfake.NewSimpleClientset(objects...), "StatefulSet", "db", "apps")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(truncated).To(BeTrue())
	g.Expect(targets).To(HaveLen(1 + maxLogPods))
	g.Expect(targets[0]).To(Equal(eventTarget{kind: "StatefulSet", name: "db"}))
	g.Expect(targets[1]).To(Equal(eventTarget{kind: "Pod", name: fmt.Sprintf("db-%d", maxLogPods+1)}))
}
