// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestMetricsCollector_IngestAndSeries(t *testing.T) {
	g := NewWithT(t)

	mc := NewMetricsCollector(nil, time.Minute)
	now := time.Now().Truncate(time.Second)

	list := &metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "main",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(64<<20, resource.BinarySI),
						},
					},
					{
						Name: "sidecar",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(50, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(32<<20, resource.BinarySI),
						},
					},
				},
			},
		},
	}
	mc.ingest(list, now)

	g.Expect(mc.Available()).To(BeTrue())

	// Containers are summed per timestamp.
	series := mc.PodSeries("default", "app-1")
	g.Expect(series).To(HaveLen(1))
	g.Expect(series[0].Time).To(Equal(now))
	g.Expect(series[0].CPU).To(BeNumerically("~", 0.15, 1e-9))
	g.Expect(series[0].Memory).To(Equal(int64(96 << 20)))

	latest, ok := mc.PodLatest("default", "app-1")
	g.Expect(ok).To(BeTrue())
	g.Expect(latest.CPU).To(BeNumerically("~", 0.15, 1e-9))

	// Unknown pods return no data.
	g.Expect(mc.PodSeries("default", "unknown")).To(BeEmpty())
	_, ok = mc.PodLatest("other", "app-1")
	g.Expect(ok).To(BeFalse())
}

func TestMetricsCollector_IngestAgePruning(t *testing.T) {
	g := NewWithT(t)

	mc := NewMetricsCollector(nil, time.Minute)
	start := time.Now().Truncate(time.Second)

	list := &metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "main",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(10, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	// A few scrapes, then a prolonged outage: the first scrape after
	// recovery must not resurface the hours-old samples.
	mc.ingest(list, start)
	mc.ingest(list, start.Add(time.Minute))
	mc.ingest(list, start.Add(3*time.Hour))

	series := mc.PodSeries("default", "app-1")
	g.Expect(series).To(HaveLen(1))
	g.Expect(series[0].Time).To(Equal(start.Add(3 * time.Hour)))
}

func TestMetricsCollector_RetentionAndOrder(t *testing.T) {
	g := NewWithT(t)

	mc := NewMetricsCollector(nil, time.Minute)
	start := time.Now().Truncate(time.Second)

	// Ingest more scrapes than the retained window spans.
	// The last scrape lands at start+35m, so the age cutoff
	// (window + interval = 31m) drops the ticks before start+4m.
	for i := range 36 {
		list := &metricsv1beta1api.PodMetricsList{
			Items: []metricsv1beta1api.PodMetrics{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
					Containers: []metricsv1beta1api.ContainerMetrics{
						{
							Name: "main",
							Usage: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(i), resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(int64(i), resource.BinarySI),
							},
						},
					},
				},
			},
		}
		mc.ingest(list, start.Add(time.Duration(i)*time.Minute))
	}

	series := mc.PodSeries("default", "app-1")
	g.Expect(series).To(HaveLen(32))

	// Oldest samples were dropped and the series is chronological.
	g.Expect(series[0].Memory).To(Equal(int64(4)))
	for i := 1; i < len(series); i++ {
		g.Expect(series[i].Time.After(series[i-1].Time)).To(BeTrue())
	}
}

func TestMetricsCollector_Eviction(t *testing.T) {
	g := NewWithT(t)

	mc := NewMetricsCollector(nil, time.Minute)
	start := time.Now().Truncate(time.Second)

	list := func(name string) *metricsv1beta1api.PodMetricsList {
		return &metricsv1beta1api.PodMetricsList{
			Items: []metricsv1beta1api.PodMetrics{
				{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Containers: []metricsv1beta1api.ContainerMetrics{
						{
							Name: "main",
							Usage: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(10, resource.BinarySI),
							},
						},
					},
				},
			},
		}
	}

	mc.ingest(list("app-old"), start)

	// A scrape within the retention window keeps the missing pod.
	mc.ingest(list("app-new"), start.Add(time.Minute))
	g.Expect(mc.PodSeries("default", "app-old")).NotTo(BeEmpty())

	// A scrape past the retention window evicts it.
	mc.ingest(list("app-new"), start.Add(mc.retention+2*time.Minute))
	g.Expect(mc.PodSeries("default", "app-old")).To(BeEmpty())
	g.Expect(mc.PodSeries("default", "app-new")).NotTo(BeEmpty())
}

func TestMetricsCollector_Scrape(t *testing.T) {
	g := NewWithT(t)

	// A collector that never scraped successfully is unavailable.
	failing := NewMetricsCollector(func(_ context.Context) (*metricsv1beta1api.PodMetricsList, error) {
		return nil, errors.New("the server could not find the requested resource")
	}, time.Minute)
	failing.scrape(context.Background())
	g.Expect(failing.Available()).To(BeFalse())
	g.Expect(failing.failing).To(BeTrue())

	// A successful scrape makes it available.
	working := NewMetricsCollector(func(_ context.Context) (*metricsv1beta1api.PodMetricsList, error) {
		return &metricsv1beta1api.PodMetricsList{}, nil
	}, time.Minute)
	working.scrape(context.Background())
	g.Expect(working.Available()).To(BeTrue())

	// A transient failure keeps the buffered data available.
	working.lister = failing.lister
	working.scrape(context.Background())
	g.Expect(working.Available()).To(BeTrue())
	g.Expect(working.failing).To(BeTrue())

	// Availability expires when scrapes keep failing.
	working.mu.Lock()
	working.lastSuccess = time.Now().Add(-4 * time.Minute)
	working.mu.Unlock()
	g.Expect(working.Available()).To(BeFalse())

	// A successful scrape clears the failure state.
	working.lister = func(_ context.Context) (*metricsv1beta1api.PodMetricsList, error) {
		return &metricsv1beta1api.PodMetricsList{}, nil
	}
	working.scrape(context.Background())
	g.Expect(working.Available()).To(BeTrue())
	g.Expect(working.failing).To(BeFalse())
}

func TestMetricsCollector_IngestDuplicateTick(t *testing.T) {
	g := NewWithT(t)

	mc := NewMetricsCollector(nil, time.Minute)
	now := time.Now().Truncate(time.Second)
	list := &metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{{
			ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
			Containers: []metricsv1beta1api.ContainerMetrics{{
				Name: "main",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(64<<20, resource.BinarySI),
				},
			}},
		}},
	}

	// Two scrapes landing within the same truncated second (a catch-up
	// overlapping the ticker) keep a single sample for the tick, so the
	// aggregation never double-counts.
	mc.ingest(list, now)
	mc.ingest(list, now.Add(500*time.Millisecond))

	series := mc.PodSeries("default", "app-1")
	g.Expect(series).To(HaveLen(1))
	g.Expect(series[0].CPU).To(BeNumerically("~", 0.1, 1e-9))

	// A later tick appends normally.
	mc.ingest(list, now.Add(time.Minute))
	g.Expect(mc.PodSeries("default", "app-1")).To(HaveLen(2))
}

func TestMetricsCollector_RequestScrape(t *testing.T) {
	g := NewWithT(t)

	scrapes := make(chan struct{}, 10)
	mc := NewMetricsCollector(func(_ context.Context) (*metricsv1beta1api.PodMetricsList, error) {
		scrapes <- struct{}{}
		return &metricsv1beta1api.PodMetricsList{}, nil
	}, time.Minute)

	// Without a started collector (no context) requests are dropped.
	mc.RequestScrape()
	g.Expect(scrapes).NotTo(Receive())

	// While the Metrics API is unavailable requests are dropped.
	mc.ctx = context.Background()
	mc.RequestScrape()
	g.Expect(scrapes).NotTo(Receive())

	mc.mu.Lock()
	mc.lastSuccess = time.Now()
	mc.mu.Unlock()

	// An overdue collector serves the catch-up request.
	g.Expect(time.Since(mc.lastScrape)).To(BeNumerically(">", metricsCatchupInterval))
	mc.RequestScrape()
	g.Eventually(scrapes).Should(Receive())

	// A second request right after is rate-limited.
	g.Eventually(func() bool {
		mc.mu.RLock()
		defer mc.mu.RUnlock()
		return mc.catchup
	}).Should(BeFalse())
	mc.RequestScrape()
	g.Consistently(scrapes, 100*time.Millisecond).ShouldNot(Receive())
}

func TestMetricsCollector_WorkloadSeries(t *testing.T) {
	g := NewWithT(t)

	mc := NewMetricsCollector(nil, time.Minute)
	start := time.Now().Truncate(time.Second)

	item := func(pod string, value int64) metricsv1beta1api.PodMetrics {
		return metricsv1beta1api.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{Name: pod, Namespace: "default"},
			Containers: []metricsv1beta1api.ContainerMetrics{
				{
					Name: "main",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    *resource.NewMilliQuantity(value, resource.DecimalSI),
						corev1.ResourceMemory: *resource.NewQuantity(value, resource.BinarySI),
					},
				},
			},
		}
	}

	// A completed pod ("job") stops appearing in scrapes after tick 9,
	// while a long-running pod ("app") keeps reporting for 41 ticks. The
	// union of their retained samples spans more than the usage window.
	for i := range 41 {
		items := []metricsv1beta1api.PodMetrics{item("app", int64(i))}
		if i < 10 {
			items = append(items, item("job", 100))
		}
		mc.ingest(&metricsv1beta1api.PodMetricsList{Items: items}, start.Add(time.Duration(i)*time.Minute))
	}

	series := mc.WorkloadSeries("default", []string{"app", "job", "missing"}, nil)

	// The merged series is trimmed to the window span ending at the newest
	// sample (ticks 10-40) and stays chronological.
	g.Expect(series).To(HaveLen(31))
	for i := 1; i < len(series); i++ {
		g.Expect(series[i].Time.After(series[i-1].Time)).To(BeTrue())
	}

	// The completed pod's ticks fall outside the window and the newest
	// tick carries only the running pod's usage.
	g.Expect(series[0].Memory).To(Equal(int64(10)))
	g.Expect(series[len(series)-1].Memory).To(Equal(int64(40)))

	// Ticks where both pods reported carry the summed usage: the oldest
	// retained tick is 10 (39-29), past the completed pod's last report,
	// so no summed tick survives the trim in this shape. Verify summing
	// directly on an untrimmed two-pod window instead.
	short := NewMetricsCollector(nil, time.Minute)
	short.ingest(&metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{item("a", 1), item("b", 2)},
	}, start)
	sum := short.WorkloadSeries("default", []string{"a", "b"}, nil)
	g.Expect(sum).To(HaveLen(1))
	g.Expect(sum[0].Memory).To(Equal(int64(3)))
}

func TestMetricsCollector_WorkloadSeries_Rollout(t *testing.T) {
	g := NewWithT(t)

	mc := NewMetricsCollector(nil, time.Minute)
	start := time.Now().Truncate(time.Second)

	item := func(pod string, podLabels map[string]string) metricsv1beta1api.PodMetrics {
		return metricsv1beta1api.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{Name: pod, Namespace: "apps", Labels: podLabels},
			Containers: []metricsv1beta1api.ContainerMetrics{
				{
					Name: "main",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
						corev1.ResourceMemory: *resource.NewQuantity(100, resource.BinarySI),
					},
				},
			},
		}
	}
	appLabels := map[string]string{"app": "backend", "pod-template-hash": "old"}
	newLabels := map[string]string{"app": "backend", "pod-template-hash": "new"}
	otherLabels := map[string]string{"app": "frontend"}

	// 10 scrapes with the old pod, then a rollout: the old pod disappears
	// and a new pod with a different name (but the same app label) starts.
	for i := range 10 {
		mc.ingest(&metricsv1beta1api.PodMetricsList{Items: []metricsv1beta1api.PodMetrics{
			item("backend-old-1", appLabels),
			item("frontend-1", otherLabels),
		}}, start.Add(time.Duration(i)*time.Minute))
	}
	mc.ingest(&metricsv1beta1api.PodMetricsList{Items: []metricsv1beta1api.PodMetrics{
		item("backend-new-1", newLabels),
		item("frontend-1", otherLabels),
	}}, start.Add(10*time.Minute))

	selector := labels.SelectorFromSet(map[string]string{"app": "backend"})

	// Right after the rollout only the new pod is listed, but the selector
	// pulls in the replaced pod's history: the chart stays continuous.
	series := mc.WorkloadSeries("apps", []string{"backend-new-1"}, selector)
	g.Expect(series).To(HaveLen(11))
	g.Expect(series[0].Memory).To(Equal(int64(100)))
	g.Expect(series[len(series)-1].Memory).To(Equal(int64(100)))

	// Pods of other workloads are not picked up.
	for _, s := range series {
		g.Expect(s.Memory).To(Equal(int64(100)))
	}

	// Without a selector, only the new pod's single sample is returned.
	g.Expect(mc.WorkloadSeries("apps", []string{"backend-new-1"}, nil)).To(HaveLen(1))

	// An unrelated namespace with the same labels yields nothing.
	g.Expect(mc.WorkloadSeries("prod", []string{"backend-new-1"}, selector)).To(BeEmpty())

	// LabeledPods matches buffered pods on their labels.
	g.Expect(mc.LabeledPods("apps", selector)).To(ConsistOf("backend-old-1", "backend-new-1"))
	g.Expect(mc.LabeledPods("prod", selector)).To(BeEmpty())
}

func TestSumPodResources(t *testing.T) {
	g := NewWithT(t)

	sidecarPolicy := corev1.ContainerRestartPolicyAlways
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					// Plain init containers are excluded from the totals.
					Name: "init",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
					},
				},
				{
					// Sidecar init containers are included.
					Name:          "sidecar",
					RestartPolicy: &sidecarPolicy,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "main",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
				{
					// A container without resources contributes nothing.
					Name: "noop",
				},
			},
		},
	}

	res := sumPodResources(pod)
	g.Expect(res.cpuRequests).To(BeNumerically("~", 0.15, 1e-9))
	g.Expect(res.cpuLimits).To(BeZero())
	g.Expect(res.memoryRequests).To(Equal(int64(96 << 20)))
	g.Expect(res.memoryLimits).To(Equal(int64(128 << 20)))
}

func TestBuildWorkloadMetrics(t *testing.T) {
	g := NewWithT(t)

	now := time.Now().Truncate(time.Second)
	mc := NewMetricsCollector(nil, time.Minute)
	mc.ingest(&metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "main",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(64<<20, resource.BinarySI),
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-2", Namespace: "default"},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "main",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(200, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(32<<20, resource.BinarySI),
						},
					},
				},
			},
		},
	}, now)

	handler := &Handler{metrics: mc}
	pods := []WorkloadPodStatus{
		{
			Name:   "app-1",
			Status: string(corev1.PodRunning),
			resources: podResources{
				cpuRequests:    0.1,
				memoryRequests: 64 << 20,
				memoryLimits:   128 << 20,
			},
		},
		{
			Name:   "app-2",
			Status: string(corev1.PodRunning),
			resources: podResources{
				cpuRequests:    0.1,
				memoryRequests: 64 << 20,
				memoryLimits:   128 << 20,
			},
		},
		{
			// Completed pods contribute no requests/limits.
			Name:   "app-done",
			Status: string(corev1.PodSucceeded),
			resources: podResources{
				cpuRequests: 5,
			},
		},
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "app", "namespace": "default"},
	}}

	wm := handler.buildWorkloadMetrics(obj, pods)
	g.Expect(wm).NotTo(BeNil())
	g.Expect(wm.Samples).To(HaveLen(1))
	g.Expect(wm.Samples[0].CPU).To(BeNumerically("~", 0.3, 1e-9))
	g.Expect(wm.Samples[0].Memory).To(Equal(int64(96 << 20)))
	g.Expect(wm.CPURequests).To(BeNumerically("~", 0.2, 1e-9))
	g.Expect(wm.CPULimits).To(BeZero())
	g.Expect(wm.MemoryRequests).To(Equal(int64(128 << 20)))
	g.Expect(wm.MemoryLimits).To(Equal(int64(256 << 20)))

	// Unknown pods produce no metrics.
	g.Expect(handler.buildWorkloadMetrics(obj, []WorkloadPodStatus{{Name: "ghost"}})).To(BeNil())

	// A nil collector or unavailable Metrics API produces no metrics.
	g.Expect((&Handler{}).buildWorkloadMetrics(obj, pods)).To(BeNil())
	g.Expect((&Handler{metrics: NewMetricsCollector(nil, time.Minute)}).buildWorkloadMetrics(obj, pods)).To(BeNil())
}

func TestCurrentPodMetrics(t *testing.T) {
	g := NewWithT(t)

	mc := NewMetricsCollector(nil, time.Minute)
	list := &metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "default"},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "main",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(64<<20, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	// A fresh sample is returned.
	mc.ingest(list, time.Now())
	handler := &Handler{metrics: mc}
	m := handler.currentPodMetrics("default", "app-1")
	g.Expect(m).NotTo(BeNil())
	g.Expect(m.CPU).To(BeNumerically("~", 0.1, 1e-9))

	// A stale sample is not.
	stale := NewMetricsCollector(nil, time.Minute)
	stale.ingest(list, time.Now().Add(-30*time.Minute))
	handler = &Handler{metrics: stale}
	g.Expect(handler.currentPodMetrics("default", "app-1")).To(BeNil())

	// A handler without a collector returns nothing.
	g.Expect((&Handler{}).currentPodMetrics("default", "app-1")).To(BeNil())
}

func TestGetWorkloadStatus_PodMetrics(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment and a managed pod (envtest has no controllers).
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-metrics",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-metrics"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-metrics"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-metrics-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "test-workload-metrics"},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-workload-metrics-abc123",
					UID:        "test-metrics-uid",
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, pod)).To(Succeed())
	defer testClient.Delete(ctx, pod)

	pod.Status.Phase = corev1.PodRunning
	g.Expect(testClient.Status().Update(ctx, pod)).To(Succeed())

	// Seed the collector with a scrape for the pod.
	mc := NewMetricsCollector(nil, time.Minute)
	mc.ingest(&metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "nginx",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(50, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(32<<20, resource.BinarySI),
						},
					},
				},
			},
		},
	}, time.Now())

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		metrics:       mc,
	}

	workload, err := handler.GetWorkloadStatus(ctx, "Deployment", deployment.Name, "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload.Pods).To(HaveLen(1))

	// The pod carries its current usage and spec resources.
	g.Expect(workload.Pods[0].Metrics).NotTo(BeNil())
	g.Expect(workload.Pods[0].Metrics.CPU).To(BeNumerically("~", 0.05, 1e-9))
	g.Expect(workload.Pods[0].Metrics.Memory).To(Equal(int64(32 << 20)))
	g.Expect(workload.Pods[0].resources.cpuRequests).To(BeNumerically("~", 0.1, 1e-9))

	// The workload-level aggregation picks up the running pod.
	depObj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": deployment.Name, "namespace": "default"},
		"spec": map[string]any{
			"selector": map[string]any{
				"matchLabels": map[string]any{"app": "test-workload-metrics"},
			},
		},
	}}
	wm := handler.buildWorkloadMetrics(depObj, workload.Pods)
	g.Expect(wm).NotTo(BeNil())
	g.Expect(wm.Samples).To(HaveLen(1))
	g.Expect(wm.CPURequests).To(BeNumerically("~", 0.1, 1e-9))
	g.Expect(wm.MemoryRequests).To(Equal(int64(64 << 20)))
}

func TestGetWorkloadStatus_MatchExpressionsSelector(t *testing.T) {
	g := NewWithT(t)

	// A Deployment whose selector uses matchExpressions only.
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-expr",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"test-workload-expr"}},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-expr"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-expr-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "test-workload-expr"},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-workload-expr-abc123",
					UID:        "test-expr-uid",
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, pod)).To(Succeed())
	defer testClient.Delete(ctx, pod)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	workload, err := handler.GetWorkloadStatus(ctx, "Deployment", deployment.Name, "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload.Pods).To(HaveLen(1))
	g.Expect(workload.Pods[0].Name).To(Equal(pod.Name))
}

func TestControllerMetrics(t *testing.T) {
	g := NewWithT(t)

	// Create a Flux controller pod with requests set and no limits.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ctrl-metrics-pod",
			Namespace: "default",
			Labels:    map[string]string{"app.kubernetes.io/part-of": "flux"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "manager",
					Image: "ghcr.io/fluxcd/source-controller:v1.7.4",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, pod)).To(Succeed())
	defer testClient.Delete(ctx, pod)

	pod.Status.Phase = corev1.PodRunning
	g.Expect(testClient.Status().Update(ctx, pod)).To(Succeed())

	// A terminated controller pod must not contribute stale usage.
	donePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ctrl-metrics-done",
			Namespace: "default",
			Labels:    map[string]string{"app.kubernetes.io/part-of": "flux"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{Name: "manager", Image: "ghcr.io/fluxcd/source-controller:v1.7.4"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, donePod)).To(Succeed())
	defer testClient.Delete(ctx, donePod)

	donePod.Status.Phase = corev1.PodSucceeded
	g.Expect(testClient.Status().Update(ctx, donePod)).To(Succeed())

	mc := NewMetricsCollector(nil, time.Minute)
	mc.ingest(&metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "manager",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(50, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(32<<20, resource.BinarySI),
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: donePod.Name, Namespace: donePod.Namespace},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "manager",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(8<<20, resource.BinarySI),
						},
					},
				},
			},
		},
	}, time.Now())

	handler := &Handler{
		kubeClient: kubeClient,
		namespace:  "default",
		metrics:    mc,
	}

	metrics := handler.controllerMetrics(ctx)
	g.Expect(metrics).To(HaveLen(1))
	g.Expect(metrics[0].Pod).To(Equal(pod.Name))
	g.Expect(metrics[0].Namespace).To(Equal("default"))
	g.Expect(metrics[0].CPU).To(BeNumerically("~", 0.05, 1e-9))
	g.Expect(metrics[0].Memory).To(Equal(int64(32 << 20)))
	g.Expect(metrics[0].CPURequests).To(BeNumerically("~", 0.1, 1e-9))
	g.Expect(metrics[0].CPULimits).To(BeZero())
	g.Expect(metrics[0].MemoryRequests).To(Equal(int64(64 << 20)))
	g.Expect(metrics[0].MemoryLimits).To(BeZero())

	// A disabled or unavailable collector produces no metrics.
	g.Expect((&Handler{kubeClient: kubeClient, namespace: "default"}).controllerMetrics(ctx)).To(BeNil())
	g.Expect((&Handler{
		kubeClient: kubeClient,
		namespace:  "default",
		metrics:    NewMetricsCollector(nil, time.Minute),
	}).controllerMetrics(ctx)).To(BeNil())
}

func TestSumSeries(t *testing.T) {
	g := NewWithT(t)

	t0 := time.Now().Truncate(time.Second)
	t1 := t0.Add(time.Minute)

	a := []MetricsSample{{Time: t0, CPU: 0.1, Memory: 100}, {Time: t1, CPU: 0.2, Memory: 200}}
	b := []MetricsSample{{Time: t1, CPU: 0.3, Memory: 300}}

	sum := sumSeries(a, b)
	g.Expect(sum).To(HaveLen(2))
	g.Expect(sum[0].Time).To(Equal(t0))
	g.Expect(sum[0].CPU).To(BeNumerically("~", 0.1, 1e-9))
	g.Expect(sum[1].CPU).To(BeNumerically("~", 0.5, 1e-9))
	g.Expect(sum[1].Memory).To(Equal(int64(500)))

	g.Expect(sumSeries()).To(BeEmpty())
	g.Expect(sumSeries(nil, nil)).To(BeEmpty())
}
