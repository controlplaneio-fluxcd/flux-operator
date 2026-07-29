// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// metricsWindow is the usage history span retained per container.
	// The number of samples this amounts to depends on the configured
	// scrape interval (30 samples at the default 60s).
	metricsWindow = 30 * time.Minute

	// metricsScrapeTimeout bounds a single Metrics API request so that a
	// stalled aggregated API server cannot block the scrape loop or the
	// synchronous initial scrape performed on handler startup.
	metricsScrapeTimeout = 30 * time.Second
)

// MetricsSample is a point-in-time CPU/Memory usage measurement.
type MetricsSample struct {
	// Time is the scrape timestamp shared by all samples of the same scrape.
	Time time.Time `json:"t"`

	// CPU is the usage in cores.
	CPU float64 `json:"cpu"`

	// Memory is the usage in bytes.
	Memory int64 `json:"memory"`
}

// PodMetricsLister lists the current pod metrics cluster-wide.
type PodMetricsLister func(ctx context.Context) (*metricsv1beta1api.PodMetricsList, error)

// containerSeries holds the retained samples of a single container.
type containerSeries struct {
	samples []MetricsSample
}

// podSeries holds the retained samples of a pod's containers along with
// the pod labels and the time the pod was last seen in a scrape. The
// labels allow matching a workload's selector against pods that were
// replaced by a rollout and no longer exist in the cluster.
type podSeries struct {
	containers map[string]*containerSeries
	labels     map[string]string
	lastSeen   time.Time
}

// MetricsCollector maintains an in-memory ring buffer of CPU/Memory usage
// samples for all pods in the cluster, populated by periodically querying
// the Kubernetes Metrics API (metrics-server) with the privileged client.
// The Metrics API only serves instantaneous values, so history must be
// accumulated server-side. Lookups are pure in-memory reads and are safe
// for concurrent use.
type MetricsCollector struct {
	lister     PodMetricsLister
	interval   time.Duration
	maxSamples int
	retention  time.Duration

	mu        sync.RWMutex
	available bool
	pods      map[string]*podSeries
}

// NewMetricsCollector creates a MetricsCollector that scrapes pod metrics
// using the given lister at the given interval. The retained sample count
// is derived from the interval so the usage window stays ~30 minutes
// regardless of the configured scrape cadence.
func NewMetricsCollector(lister PodMetricsLister, interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		lister:   lister,
		interval: interval,
		// One extra sample so the retained span covers the full window:
		// n samples are (n-1) intervals apart.
		maxSamples: int(metricsWindow/interval) + 1,
		// Keep evicting pods slightly later than the sample window so a
		// single failed scrape does not drop otherwise fresh series.
		retention: metricsWindow + 2*interval,
		pods:      make(map[string]*podSeries),
	}
}

// Start launches a background goroutine that scrapes immediately and then
// at the collector's interval until the context is canceled. The initial
// scrape runs on the goroutine so that a stalled Metrics API cannot delay
// server startup or configuration reloads.
// It returns a channel that is closed when the goroutine stops.
func (mc *MetricsCollector) Start(ctx context.Context) <-chan struct{} {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)

		mc.scrape(ctx)

		ticker := time.NewTicker(mc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mc.scrape(ctx)
			}
		}
	}()

	return stopped
}

// Available reports whether the last scrape of the Metrics API succeeded.
// It returns false when metrics-server is not installed, allowing the UI
// to hide the metrics widgets.
func (mc *MetricsCollector) Available() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.available
}

// scrape queries the Metrics API and ingests the result. Failures mark the
// collector unavailable until a subsequent scrape succeeds, which doubles
// as feature detection for clusters without metrics-server.
func (mc *MetricsCollector) scrape(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, metricsScrapeTimeout)
	defer cancel()

	list, err := mc.lister(ctx)
	if err != nil {
		mc.mu.Lock()
		wasAvailable := mc.available
		mc.available = false
		mc.mu.Unlock()
		if wasAvailable && ctx.Err() == nil {
			log.FromContext(ctx).Error(err, "pod metrics collection failed, the Metrics API is unavailable")
		}
		return
	}
	mc.ingest(list, time.Now())
}

// ingest appends one sample per container from the given list, trims each
// series to the retention window and evicts pods missing from scrapes for
// longer than the retention period.
//
// Samples are stamped with the scrape time rather than the per-pod
// PodMetrics timestamp: a shared timestamp keeps the ticks of all pods
// aligned so they can be summed per tick into workload-level series.
// The Metrics API values may therefore be up to one metrics-server
// resolution window older than the sample timestamp suggests.
func (mc *MetricsCollector) ingest(list *metricsv1beta1api.PodMetricsList, now time.Time) {
	now = now.Truncate(time.Second)

	// Samples older than the window are dropped even when the count-based
	// trim retains them, so that a prolonged scrape outage cannot resurface
	// hours-old data as part of a nominal 30-minute chart.
	ageCutoff := now.Add(-(metricsWindow + mc.interval))

	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.available = true

	for _, item := range list.Items {
		key := podKey(item.Namespace, item.Name)
		ps, ok := mc.pods[key]
		if !ok {
			ps = &podSeries{containers: make(map[string]*containerSeries)}
			mc.pods[key] = ps
		}
		ps.labels = item.Labels
		ps.lastSeen = now

		for _, container := range item.Containers {
			cpuQuantity := container.Usage[corev1.ResourceCPU]
			memQuantity := container.Usage[corev1.ResourceMemory]
			sample := MetricsSample{
				Time: now,
				// AsApproximateFloat64 preserves sub-millicore usage,
				// which MilliValue would round up to a full millicore.
				CPU:    cpuQuantity.AsApproximateFloat64(),
				Memory: memQuantity.Value(),
			}

			cs, ok := ps.containers[container.Name]
			if !ok {
				cs = &containerSeries{}
				ps.containers[container.Name] = cs
			}
			cs.samples = append(cs.samples, sample)
			if len(cs.samples) > mc.maxSamples {
				cs.samples = cs.samples[len(cs.samples)-mc.maxSamples:]
			}
			for len(cs.samples) > 0 && cs.samples[0].Time.Before(ageCutoff) {
				cs.samples = cs.samples[1:]
			}
		}
	}

	// Evict pods that disappeared from the cluster.
	cutoff := now.Add(-mc.retention)
	for key, ps := range mc.pods {
		if ps.lastSeen.Before(cutoff) {
			delete(mc.pods, key)
		}
	}
}

// PodLatest returns the most recent usage sample of a pod,
// summed across its containers.
func (mc *MetricsCollector) PodLatest(namespace, name string) (MetricsSample, bool) {
	series := mc.PodSeries(namespace, name)
	if len(series) == 0 {
		return MetricsSample{}, false
	}
	return series[len(series)-1], true
}

// PodSeries returns the usage time series of a pod, with the containers
// summed per scrape timestamp and the result sorted chronologically.
func (mc *MetricsCollector) PodSeries(namespace, name string) []MetricsSample {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	ps, ok := mc.pods[podKey(namespace, name)]
	if !ok {
		return nil
	}

	seriesList := make([][]MetricsSample, 0, len(ps.containers))
	for _, cs := range ps.containers {
		seriesList = append(seriesList, cs.samples)
	}
	return sumSeries(seriesList...)
}

// WorkloadSeries returns the usage series of the given pods summed per
// scrape timestamp. When a selector is provided, buffered pods in the
// namespace whose labels match it are included as well: pods replaced by
// a rollout keep matching their workload's selector, which preserves the
// chart history across restarts even though the pod names have changed.
// This relies on the Kubernetes convention that selectors of workloads
// in the same namespace do not overlap; pods of a sibling workload with
// an overlapping selector would be aggregated into the series.
// All pod series are read under a single lock so the aggregate reflects
// one collector generation, and the merged series is trimmed to the
// newest maxSamples ticks so retained history of completed pods cannot
// stretch the window beyond its intended span.
func (mc *MetricsCollector) WorkloadSeries(namespace string, names []string, selector labels.Selector) []MetricsSample {
	keys := make(map[string]bool, len(names))
	for _, name := range names {
		keys[podKey(namespace, name)] = true
	}

	mc.mu.RLock()
	if selector != nil && !selector.Empty() {
		prefix := namespace + "/"
		for key, ps := range mc.pods {
			if strings.HasPrefix(key, prefix) && selector.Matches(labels.Set(ps.labels)) {
				keys[key] = true
			}
		}
	}

	seriesList := make([][]MetricsSample, 0, len(keys))
	for key := range keys {
		if ps, ok := mc.pods[key]; ok {
			for _, cs := range ps.containers {
				seriesList = append(seriesList, cs.samples)
			}
		}
	}
	sum := sumSeries(seriesList...)
	mc.mu.RUnlock()

	if len(sum) > mc.maxSamples {
		sum = sum[len(sum)-mc.maxSamples:]
	}
	return sum
}

// sumSeries merges multiple sample series into one by summing
// the values that share a timestamp. The result is sorted chronologically.
func sumSeries(seriesList ...[]MetricsSample) []MetricsSample {
	byTime := make(map[time.Time]*MetricsSample)
	for _, samples := range seriesList {
		for _, s := range samples {
			agg, ok := byTime[s.Time]
			if !ok {
				agg = &MetricsSample{Time: s.Time}
				byTime[s.Time] = agg
			}
			agg.CPU += s.CPU
			agg.Memory += s.Memory
		}
	}

	result := make([]MetricsSample, 0, len(byTime))
	for _, s := range byTime {
		result = append(result, *s)
	}
	slices.SortFunc(result, func(a, b MetricsSample) int {
		return a.Time.Compare(b.Time)
	})
	return result
}

// podKey builds the buffer key of a pod.
func podKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// workloadPodSelector extracts the pod label selector (matchLabels and
// matchExpressions) from a workload spec. Returns nil when the workload
// has no selector or it cannot be parsed.
func workloadPodSelector(obj *unstructured.Unstructured) labels.Selector {
	sel, found, _ := unstructured.NestedMap(obj.Object, "spec", "selector")
	if !found {
		return nil
	}
	labelSelector := &metav1.LabelSelector{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(sel, labelSelector); err != nil {
		return nil
	}
	s, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil
	}
	return s
}

// currentPodMetrics returns the latest usage sample of a pod when it is
// recent enough to describe the pod's present state, guarding against
// stale samples retained for pods that have completed or restarted.
func (h *Handler) currentPodMetrics(namespace, name string) *MetricsSample {
	if h.metrics == nil || !h.metrics.Available() {
		return nil
	}
	latest, ok := h.metrics.PodLatest(namespace, name)
	if !ok || time.Since(latest.Time) > 3*h.metrics.interval {
		return nil
	}
	return &latest
}

// sumPodResources sums the CPU/Memory requests and limits of the pod
// containers, including sidecar init containers as they consume
// resources for the pod's whole lifetime.
func sumPodResources(pod *corev1.Pod) podResources {
	var res podResources

	containers := make([]corev1.Container, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	containers = append(containers, pod.Spec.Containers...)
	for _, c := range pod.Spec.InitContainers {
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			containers = append(containers, c)
		}
	}

	for _, c := range containers {
		if q, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			res.cpuRequests += q.AsApproximateFloat64()
		}
		if q, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
			res.cpuLimits += q.AsApproximateFloat64()
		}
		if q, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			res.memoryRequests += q.Value()
		}
		if q, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			res.memoryLimits += q.Value()
		}
	}

	return res
}

// buildWorkloadMetrics computes the aggregated usage of a workload from
// the retained series of its pods. The samples include the history of
// completed and recently replaced pods (matched by the workload selector),
// while the requests/limits are summed over the running pods only, as
// they describe the present resource reservation. It returns nil when
// no samples have been collected for the given pods.
func (h *Handler) buildWorkloadMetrics(obj *unstructured.Unstructured, pods []WorkloadPodStatus) *WorkloadMetrics {
	if h.metrics == nil || !h.metrics.Available() {
		return nil
	}

	// The workload pod selector preserves the chart history across
	// rollouts (CronJobs have none; their completed pods are already
	// part of the pod list).
	selector := workloadPodSelector(obj)

	names := make([]string, 0, len(pods))
	wm := &WorkloadMetrics{}
	for _, pod := range pods {
		names = append(names, pod.Name)
		if pod.Status == string(corev1.PodRunning) {
			wm.CPURequests += pod.resources.cpuRequests
			wm.CPULimits += pod.resources.cpuLimits
			wm.MemoryRequests += pod.resources.memoryRequests
			wm.MemoryLimits += pod.resources.memoryLimits
		}
	}

	wm.Samples = h.metrics.WorkloadSeries(obj.GetNamespace(), names, selector)
	if len(wm.Samples) == 0 {
		return nil
	}
	return wm
}
