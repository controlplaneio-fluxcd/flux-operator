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
	"k8s.io/client-go/rest"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// metricsWindow is the usage history span retained per container.
	metricsWindow = 30 * time.Minute

	// metricsScrapeTimeout bounds a single Metrics API request.
	metricsScrapeTimeout = 30 * time.Second

	// metricsCatchupInterval is the minimum time between off-schedule
	// scrapes requested via RequestScrape.
	metricsCatchupInterval = 15 * time.Second
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

// NewPodMetricsLister returns a PodMetricsLister that queries the Metrics
// API cluster-wide with the given config. Protobuf encoding is requested
// as it cuts the size and decode cost of the list compared to JSON.
func NewPodMetricsLister(config *rest.Config) (PodMetricsLister, error) {
	config = rest.CopyConfig(config)
	config.ContentType = runtime.ContentTypeProtobuf
	clientset, err := metricsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) (*metricsv1beta1api.PodMetricsList, error) {
		return clientset.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	}, nil
}

// containerSeries holds the retained samples of a single container.
type containerSeries struct {
	samples []MetricsSample
}

// podSeries holds the retained samples of a pod's containers along with
// the pod labels and the time the pod was last seen in a scrape.
type podSeries struct {
	containers map[string]*containerSeries
	labels     map[string]string
	lastSeen   time.Time
}

// MetricsCollector maintains an in-memory ring buffer of CPU/Memory usage
// samples for all pods in the cluster, populated by periodically querying
// the Kubernetes Metrics API. Safe for concurrent use.
type MetricsCollector struct {
	lister    PodMetricsLister
	interval  time.Duration
	retention time.Duration

	mu   sync.RWMutex
	ctx  context.Context
	pods map[string]*podSeries

	// lastSuccess is the time of the last successful scrape and failing
	// tracks the failure state for transition logging. ticks counts the
	// distinct scrape timestamps ingested since the collector was created.
	lastSuccess time.Time
	failing     bool
	ticks       int

	// lastScrape rate-limits the off-schedule scrapes requested via
	// RequestScrape; scraping guards against concurrent scrapes when a
	// catch-up overlaps a scheduled tick.
	lastScrape time.Time
	scraping   bool
}

// NewMetricsCollector creates a MetricsCollector that scrapes pod metrics
// using the given lister at the given interval.
func NewMetricsCollector(lister PodMetricsLister, interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		lister:    lister,
		interval:  interval,
		retention: metricsWindow + 2*interval,
		pods:      make(map[string]*podSeries),
	}
}

// Start performs an initial synchronous scrape and then launches a
// background goroutine that scrapes at the collector's interval until
// the context is canceled. It returns a channel that is closed when the
// goroutine stops.
func (mc *MetricsCollector) Start(ctx context.Context) <-chan struct{} {
	mc.mu.Lock()
	mc.ctx = ctx
	mc.mu.Unlock()

	mc.scrape(ctx)

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)

		timer := time.NewTimer(mc.nextScrapeDelay())
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				mc.scrape(ctx)
				timer.Reset(mc.nextScrapeDelay())
			}
		}
	}()

	return stopped
}

// nextScrapeDelay returns the delay until the next scheduled scrape:
// the configured interval, shortened to the catch-up interval while the
// collector is failing or holds fewer than two samples, so neither a
// transient error nor a fresh start leaves the UI without usage charts
// for a full interval.
func (mc *MetricsCollector) nextScrapeDelay() time.Duration {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	if mc.failing || mc.ticks < 2 {
		return min(metricsCatchupInterval, mc.interval)
	}
	return mc.interval
}

// Available reports whether the collector holds usable data: at least one
// scrape of the Metrics API succeeded and the last success is recent.
// Transient scrape failures do not hide the buffered history.
func (mc *MetricsCollector) Available() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.availableLocked()
}

// availableLocked implements Available; the caller must hold mc.mu.
func (mc *MetricsCollector) availableLocked() bool {
	return !mc.lastSuccess.IsZero() && time.Since(mc.lastSuccess) <= 3*mc.interval
}

// RequestScrape triggers one off-schedule scrape on a new goroutine,
// used to pick up pods created after the last scheduled scrape.
// Requests are dropped while a scrape is in flight, within
// metricsCatchupInterval of the last scrape, or when the Metrics API
// is unavailable.
func (mc *MetricsCollector) RequestScrape() {
	mc.mu.Lock()
	if mc.ctx == nil || !mc.availableLocked() || mc.scraping ||
		time.Since(mc.lastScrape) < metricsCatchupInterval {
		mc.mu.Unlock()
		return
	}
	scrapeCtx := mc.ctx
	mc.mu.Unlock()

	go mc.scrape(scrapeCtx)
}

// scrape queries the Metrics API and ingests the result, dropping the
// call when another scrape is already in flight so a catch-up
// overlapping a scheduled tick cannot issue concurrent cluster-wide
// queries. Failures are logged on the transition into the failing
// state, so a stalled or missing Metrics API produces one diagnostic
// instead of one per tick.
func (mc *MetricsCollector) scrape(ctx context.Context) {
	mc.mu.Lock()
	if mc.scraping {
		mc.mu.Unlock()
		return
	}
	mc.scraping = true
	mc.lastScrape = time.Now()
	mc.mu.Unlock()

	defer func() {
		mc.mu.Lock()
		mc.scraping = false
		mc.mu.Unlock()
	}()

	scrapeCtx, cancel := context.WithTimeout(ctx, metricsScrapeTimeout)
	defer cancel()

	list, err := mc.lister(scrapeCtx)
	if err != nil {
		// Stay silent only on shutdown, not on a scrape timeout.
		if ctx.Err() != nil {
			return
		}
		mc.mu.Lock()
		firstFailure := !mc.failing
		mc.failing = true
		mc.mu.Unlock()
		if firstFailure {
			log.FromContext(ctx).Error(err, "pod metrics collection failed, the Metrics API is unavailable")
		}
		return
	}
	mc.ingest(list, time.Now())
}

// ingest appends one sample per container from the given list, trims each
// series to the retention window and evicts pods missing from scrapes for
// longer than the retention period. Samples are stamped with the scrape
// time rather than the per-pod PodMetrics timestamp, so the ticks of all
// pods stay aligned and can be summed into workload-level series.
func (mc *MetricsCollector) ingest(list *metricsv1beta1api.PodMetricsList, now time.Time) {
	now = now.Truncate(time.Second)

	// Drop samples older than the window so a prolonged scrape outage
	// cannot resurface stale data.
	ageCutoff := now.Add(-(metricsWindow + mc.interval))

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Overlapping scrapes within the same truncated second share a tick.
	if !mc.lastSuccess.Equal(now) {
		mc.ticks++
	}
	mc.lastSuccess = now
	mc.failing = false

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
				// AsApproximateFloat64 preserves sub-millicore usage.
				CPU:    cpuQuantity.AsApproximateFloat64(),
				Memory: memQuantity.Value(),
			}

			cs, ok := ps.containers[container.Name]
			if !ok {
				cs = &containerSeries{}
				ps.containers[container.Name] = cs
			}
			// Overlapping scrapes can land within the same truncated
			// second; keep one sample per tick.
			if n := len(cs.samples); n > 0 && !cs.samples[n-1].Time.Before(sample.Time) {
				cs.samples[n-1] = sample
			} else {
				cs.samples = append(cs.samples, sample)
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
// namespace whose labels match it are included as well, preserving the
// chart history across rollouts. The merged series is trimmed to the
// window span ending at the newest sample.
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

	if n := len(sum); n > 0 {
		cutoff := sum[n-1].Time.Add(-metricsWindow)
		i := 0
		for i < n && sum[i].Time.Before(cutoff) {
			i++
		}
		sum = sum[i:]
	}
	return sum
}

// LabeledPods returns the names of the buffered pods in the namespace
// whose labels match the selector.
func (mc *MetricsCollector) LabeledPods(namespace string, selector labels.Selector) []string {
	prefix := namespace + "/"
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var names []string
	for key, ps := range mc.pods {
		if strings.HasPrefix(key, prefix) && selector.Matches(labels.Set(ps.labels)) {
			names = append(names, strings.TrimPrefix(key, prefix))
		}
	}
	return names
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
// recent enough to describe the pod's present state.
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
// containers, including sidecar init containers.
func sumPodResources(pod *corev1.Pod) PodResources {
	var res PodResources

	containers := make([]corev1.Container, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	containers = append(containers, pod.Spec.Containers...)
	for _, c := range pod.Spec.InitContainers {
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			containers = append(containers, c)
		}
	}

	for _, c := range containers {
		if q, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			res.CPURequests += q.AsApproximateFloat64()
		}
		if q, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
			res.CPULimits += q.AsApproximateFloat64()
		}
		if q, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			res.MemoryRequests += q.Value()
		}
		if q, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			res.MemoryLimits += q.Value()
		}
	}

	return res
}

// buildWorkloadMetrics computes the aggregated usage of a workload from
// the retained series of its pods, with the requests/limits summed over
// the running pods only. It returns nil when no samples have been
// collected for the given pods.
func (h *Handler) buildWorkloadMetrics(obj *unstructured.Unstructured, pods []WorkloadPodStatus) *WorkloadMetrics {
	if h.metrics == nil || !h.metrics.Available() {
		return nil
	}

	selector := workloadPodSelector(obj)

	names := make([]string, 0, len(pods))
	wm := &WorkloadMetrics{}
	for _, pod := range pods {
		names = append(names, pod.Name)
		if pod.Status == string(corev1.PodRunning) {
			wm.CPURequests += pod.resources.CPURequests
			wm.CPULimits += pod.resources.CPULimits
			wm.MemoryRequests += pod.resources.MemoryRequests
			wm.MemoryLimits += pod.resources.MemoryLimits
		}
	}

	wm.Samples = h.metrics.WorkloadSeries(obj.GetNamespace(), names, selector)
	if len(wm.Samples) == 0 {
		return nil
	}
	return wm
}
