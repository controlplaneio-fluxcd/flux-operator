// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/podlogs"
)

func TestResolveLogSelectionDeployment(t *testing.T) {
	g := NewWithT(t)
	now := time.Now()
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "apps"},
		Spec: appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "backend"},
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "track", Operator: metav1.LabelSelectorOpIn, Values: []string{"stable"},
			}},
		}},
	}
	ownedOld := testLogPod("backend-old", "apps", now.Add(-time.Minute), map[string]string{"app": "backend", "track": "stable"},
		metav1.OwnerReference{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "backend-abc"}, "app")
	ownedNew := testLogPod("backend-new", "apps", now, map[string]string{"app": "backend", "track": "stable"},
		metav1.OwnerReference{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "backend-def"}, "app")
	unowned := testLogPod("shared-labels", "apps", now.Add(time.Minute), map[string]string{"app": "backend", "track": "stable"},
		metav1.OwnerReference{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "other-abc"}, "app")
	wrongExpression := testLogPod("wrong-track", "apps", now, map[string]string{"app": "backend", "track": "canary"},
		metav1.OwnerReference{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "backend-ghi"}, "app")

	selection, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(
		deployment, ownedOld, ownedNew, unowned, wrongExpression), "deployment", "backend", "apps", "")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(selection.kind).To(Equal("Deployment"))
	g.Expect(selection.podsTotal).To(Equal(2))
	g.Expect(logPodNames(selection.pods)).To(Equal([]string{"backend-new", "backend-old"}))
	g.Expect(selection.targets).To(Equal([]podlogs.LogTarget{
		{Pod: "backend-new", Container: "app"},
		{Pod: "backend-old", Container: "app"},
	}))
	g.Expect(selection.tagPod).To(BeTrue())
	g.Expect(selection.truncated).To(BeFalse())
}

func TestResolveLogSelectionAppsWorkloads(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		kind      string
		workload  runtime.Object
		ownerKind string
		ownerName string
		wantKind  string
	}{
		{
			name: "StatefulSet", kind: "STATEFULSET", wantKind: "StatefulSet",
			workload: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "apps"},
				Spec:       appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "database"}}},
			},
			ownerKind: "StatefulSet", ownerName: "database",
		},
		{
			name: "DaemonSet", kind: "daemonset", wantKind: "DaemonSet",
			workload: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "apps"},
				Spec:       appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "agent"}}},
			},
			ownerKind: "DaemonSet", ownerName: "agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			pod := testLogPod(tt.ownerName+"-0", "apps", now, map[string]string{"app": tt.ownerName},
				metav1.OwnerReference{APIVersion: "apps/v1", Kind: tt.ownerKind, Name: tt.ownerName}, "main")
			selection, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(tt.workload, pod),
				tt.kind, tt.ownerName, "apps", "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(selection.kind).To(Equal(tt.wantKind))
			g.Expect(logPodNames(selection.pods)).To(Equal([]string{pod.Name}))
		})
	}
}

func TestResolveLogSelectionCronJob(t *testing.T) {
	g := NewWithT(t)
	cronUID := types.UID("cron-uid")
	jobUID := types.UID("job-uid")
	cronJob := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "apps", UID: cronUID}}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "cleanup-123", Namespace: "apps", UID: jobUID,
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: "cleanup", UID: cronUID}},
	}}
	otherJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "other-123", Namespace: "apps", UID: "other-job"}}
	ownedPod := testLogPod("cleanup-pod", "apps", time.Now(), map[string]string{batchv1.JobNameLabel: job.Name},
		metav1.OwnerReference{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: jobUID}, "worker")
	wrongOwner := testLogPod("wrong-owner", "apps", time.Now(), map[string]string{batchv1.JobNameLabel: job.Name},
		metav1.OwnerReference{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: "wrong"}, "worker")
	otherPod := testLogPod("other-pod", "apps", time.Now(), map[string]string{batchv1.JobNameLabel: otherJob.Name},
		metav1.OwnerReference{APIVersion: "batch/v1", Kind: "Job", Name: otherJob.Name, UID: otherJob.UID}, "worker")

	selection, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(
		cronJob, job, otherJob, ownedPod, wrongOwner, otherPod), "CronJob", "cleanup", "apps", "")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(logPodNames(selection.pods)).To(Equal([]string{"cleanup-pod"}))
	g.Expect(selection.targets).To(Equal([]podlogs.LogTarget{{Pod: "cleanup-pod", Container: "worker"}}))
}

func TestResolveLogSelectionJob(t *testing.T) {
	g := NewWithT(t)
	jobUID := types.UID("job-uid")
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "migrate", Namespace: "apps", UID: jobUID}}
	ownedPod := testLogPod("migrate-pod", "apps", time.Now(), map[string]string{batchv1.JobNameLabel: job.Name},
		metav1.OwnerReference{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: jobUID}, "worker")
	unownedPod := testLogPod("migrate-other", "apps", time.Now(), map[string]string{batchv1.JobNameLabel: job.Name},
		metav1.OwnerReference{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: "other"}, "worker")

	selection, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(job, ownedPod, unownedPod),
		"job", "migrate", "apps", "")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(selection.kind).To(Equal("Job"))
	g.Expect(logPodNames(selection.pods)).To(Equal([]string{"migrate-pod"}))
}

func TestResolveLogSelectionValidation(t *testing.T) {
	t.Run("unknown kind", func(t *testing.T) {
		g := NewWithT(t)
		_, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(), "ReplicaSet", "app", "apps", "")
		g.Expect(err).To(MatchError(ContainSubstring("supported kinds are Pod, Deployment, StatefulSet, DaemonSet, CronJob and Job")))
	})

	t.Run("workload with zero pods", func(t *testing.T) {
		g := NewWithT(t)
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "apps"},
			Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend"}}},
		}
		selection, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(deployment), "Deployment", "backend", "apps", "")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(selection.kind).To(Equal("Deployment"))
		g.Expect(selection.targets).To(BeEmpty())
		g.Expect(selection.podsTotal).To(BeZero())
	})

	t.Run("missing named pod", func(t *testing.T) {
		g := NewWithT(t)
		_, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(), "", "missing", "apps", "")
		g.Expect(err).To(HaveOccurred())
	})
}

func TestResolveLogSelectionCapsPodsAndStreams(t *testing.T) {
	g := NewWithT(t)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "apps"},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend"}}},
	}
	objects := []runtime.Object{deployment}
	base := time.Now()
	for i := 0; i < maxLogPods+1; i++ {
		objects = append(objects, testLogPod(fmt.Sprintf("backend-%02d", i), "apps", base.Add(time.Duration(i)*time.Second),
			map[string]string{"app": "backend"},
			metav1.OwnerReference{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "backend-rs"}, "app", "sidecar", "metrics"))
	}

	selection, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(objects...),
		"Deployment", "backend", "apps", "")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(selection.podsTotal).To(Equal(maxLogPods + 1))
	g.Expect(selection.pods).To(HaveLen(maxLogPods))
	g.Expect(selection.pods[0].Name).To(Equal("backend-10"))
	g.Expect(selection.targets).To(HaveLen(maxLogStreams))
	g.Expect(selection.truncated).To(BeTrue())
}

func TestResolveLogSelectionContainers(t *testing.T) {
	pod := testLogPod("backend", "default", time.Now(), nil, metav1.OwnerReference{}, "app", "sidecar")
	pod.OwnerReferences = nil
	pod.Spec.InitContainers = []corev1.Container{{Name: "init"}}

	t.Run("omitted container selects regular containers only", func(t *testing.T) {
		g := NewWithT(t)
		selection, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(pod), "Pod", pod.Name, pod.Namespace, "")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(selection.targets).To(Equal([]podlogs.LogTarget{
			{Pod: pod.Name, Container: "app"},
			{Pod: pod.Name, Container: "sidecar"},
		}))
		g.Expect(selection.containers).To(Equal([]string{"app", "sidecar"}))
		g.Expect(selection.tagContainer).To(BeTrue())
	})

	t.Run("container filter selects only the requested name", func(t *testing.T) {
		g := NewWithT(t)
		selection, err := resolveLogSelection(context.Background(), fake.NewSimpleClientset(pod), "pod", pod.Name, pod.Namespace, "sidecar")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(selection.targets).To(Equal([]podlogs.LogTarget{{Pod: pod.Name, Container: "sidecar"}}))
		g.Expect(selection.containers).To(Equal([]string{"sidecar"}))
		g.Expect(selection.tagContainer).To(BeFalse())
	})
}

func TestGetLogsWithFakeClient(t *testing.T) {
	g := NewWithT(t)
	pod := testLogPod("backend", "default", time.Now(), nil, metav1.OwnerReference{}, "app")

	result, err := getLogs(context.Background(), fake.NewSimpleClientset(pod), "", pod.Name, pod.Namespace, "", 100, false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Kind).To(Equal("Pod"))
	g.Expect(result.Name).To(Equal("backend"))
	g.Expect(result.Namespace).To(Equal("default"))
	g.Expect(result.Pods).To(Equal([]string{"backend"}))
	g.Expect(result.Containers).To(Equal([]string{"app"}))
	g.Expect(result.PodsTotal).To(Equal(1))
	g.Expect(result.PodsStreamed).To(Equal(1))
	g.Expect(result.Tagged).To(BeFalse())
	g.Expect(result.Truncated).To(BeFalse())
	g.Expect(result.Logs).To(Equal("fake logs\n"))
}

func TestGetLogsWorkloadWithoutPods(t *testing.T) {
	g := NewWithT(t)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "apps"},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend"}}},
	}

	result, err := getLogs(context.Background(), fake.NewSimpleClientset(deployment), "deployment", "backend", "apps", "", 100, false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Kind).To(Equal("Deployment"))
	g.Expect(result.Name).To(Equal("backend"))
	g.Expect(result.Namespace).To(Equal("apps"))
	g.Expect(result.Pods).To(BeEmpty())
	g.Expect(result.Containers).To(BeEmpty())
	g.Expect(result.PodsTotal).To(BeZero())
	g.Expect(result.PodsStreamed).To(BeZero())
	g.Expect(result.Tagged).To(BeFalse())
	g.Expect(result.Truncated).To(BeFalse())
	g.Expect(result.Logs).To(Equal("no pods found for Deployment apps/backend"))
}
func testLogPod(name, namespace string, created time.Time, podLabels map[string]string, owner metav1.OwnerReference, containers ...string) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: namespace, Labels: podLabels, CreationTimestamp: metav1.NewTime(created),
	}}
	if owner.Name != "" {
		pod.OwnerReferences = []metav1.OwnerReference{owner}
	}
	for _, container := range containers {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: container})
	}
	return pod
}

func logPodNames(pods []corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	return names
}
