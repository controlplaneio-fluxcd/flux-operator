// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/podlogs"
)

const (
	// maxLogPods caps the number of workload pods selected for a log request.
	maxLogPods = 10

	// maxLogStreams caps the total pod and container streams in a log request.
	maxLogStreams = 20

	// maxLogBytes caps the merged log payload returned by the MCP tool.
	maxLogBytes = 256 * 1024

	// minPerStreamLogBytes ensures every selected stream gets a useful tail.
	minPerStreamLogBytes = 64 * 1024

	// grepTailLines is the tail requested per stream when grep is set.
	grepTailLines int64 = 1000
)

// LogsOptions configures a GetLogs request.
type LogsOptions struct {
	// Kind is the resource kind, Pod when empty.
	Kind string
	// Name is the pod or workload name.
	Name string
	// Namespace is the pod or workload namespace.
	Namespace string
	// Container restricts the streams to one container name.
	Container string
	// Limit caps the merged entries returned, keeping the newest.
	Limit int64
	// Previous reads the previously terminated container instances.
	Previous bool
	// Since keeps only the entries newer than this duration when positive.
	Since time.Duration
	// Grep keeps only the entries matching this expression when set.
	Grep *regexp.Regexp
}

// KubernetesLogs is the YAML response returned by get_kubernetes_logs.
type KubernetesLogs struct {
	Kind         string   `json:"kind"`
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Pods         []string `json:"pods"`
	Containers   []string `json:"containers"`
	PodsTotal    int      `json:"podsTotal"`
	PodsStreamed int      `json:"podsStreamed"`
	Tagged       bool     `json:"tagged"`
	Truncated    bool     `json:"truncated"`
	Logs         string   `json:"logs"`
}

// logSelection contains resolved pods and the capped streams to fetch.
type logSelection struct {
	kind         string
	pods         []corev1.Pod
	targets      []podlogs.LogTarget
	containers   []string
	podsTotal    int
	tagPod       bool
	tagContainer bool
	truncated    bool
}

// GetLogs resolves a pod or workload, fetches its selected container streams,
// and returns their timestamped logs merged chronologically.
func (k *Client) GetLogs(ctx context.Context, opts LogsOptions) (*KubernetesLogs, error) {
	clientset, err := kubernetes.NewForConfig(k.cfg)
	if err != nil {
		return nil, err
	}
	return getLogs(ctx, clientset, opts)
}

// getLogs implements GetLogs with an injectable clientset for unit tests.
func getLogs(ctx context.Context, clientset kubernetes.Interface, opts LogsOptions) (*KubernetesLogs, error) {
	selection, err := resolveLogSelection(ctx, clientset, opts.Kind, opts.Name, opts.Namespace, opts.Container)
	if err != nil {
		return nil, err
	}
	if len(selection.targets) == 0 {
		return &KubernetesLogs{
			Kind:       selection.kind,
			Name:       opts.Name,
			Namespace:  opts.Namespace,
			Pods:       []string{},
			Containers: []string{},
			Logs:       fmt.Sprintf("no pods found for %s %s/%s", selection.kind, opts.Namespace, opts.Name),
		}, nil
	}

	perStreamBytes := max(maxLogBytes/len(selection.targets), minPerStreamLogBytes)
	logsByTarget := make([]string, len(selection.targets))
	cappedByTarget := make([]bool, len(selection.targets))
	errsByTarget := make([]error, len(selection.targets))
	var wg sync.WaitGroup
	for i, target := range selection.targets {
		wg.Add(1)
		go func(i int, target podlogs.LogTarget) {
			defer wg.Done()
			logsByTarget[i], cappedByTarget[i], errsByTarget[i] = podlogs.FetchContainerLogWithStats(
				ctx, clientset, opts.Namespace, target.Pod, podLogOptions(opts, target.Container), perStreamBytes)
		}(i, target)
	}
	wg.Wait()

	streams := make([]podlogs.LogStream, 0, len(selection.targets))
	streamedPods := make(map[string]struct{})
	streamCapped := false
	var firstErr error
	for i, target := range selection.targets {
		if errsByTarget[i] != nil {
			if firstErr == nil {
				firstErr = errsByTarget[i]
			}
			continue
		}
		streamCapped = streamCapped || cappedByTarget[i]
		streams = append(streams, podlogs.LogStream{
			Pod:       target.Pod,
			Container: target.Container,
			Blob:      logsByTarget[i],
		})
		streamedPods[target.Pod] = struct{}{}
	}
	if len(streams) == 0 && firstErr != nil {
		return nil, firstErr
	}

	mergeOptions := []podlogs.MergeOption{podlogs.WithSecondTimestamps()}
	if opts.Grep != nil {
		mergeOptions = append(mergeOptions, podlogs.WithGrep(opts.Grep))
	}
	merged := podlogs.MergeLogStreamsWithStats(streams, int(opts.Limit), selection.tagPod, selection.tagContainer,
		maxLogBytes, mergeOptions...)
	logs := merged.Logs
	truncated := selection.truncated
	if opts.Grep != nil && (merged.Truncated || streamCapped) {
		truncated = true
	}
	switch {
	case merged.Entries == 0 && (opts.Grep != nil || opts.Since > 0):
		logs = noLogEntriesMessage(opts)
	case len(selection.targets) == 1 && logs == "":
		logs = fmt.Sprintf("no logs found for container %s", selection.targets[0].Container)
	}

	podNames := make([]string, 0, len(selection.pods))
	for _, pod := range selection.pods {
		podNames = append(podNames, pod.Name)
	}

	return &KubernetesLogs{
		Kind:         selection.kind,
		Name:         opts.Name,
		Namespace:    opts.Namespace,
		Pods:         podNames,
		Containers:   selection.containers,
		PodsTotal:    selection.podsTotal,
		PodsStreamed: len(streamedPods),
		Tagged:       selection.tagPod || selection.tagContainer,
		Truncated:    truncated,
		Logs:         logs,
	}, nil
}

// noLogEntriesMessage describes an empty result in terms of the active filters.
func noLogEntriesMessage(opts LogsOptions) string {
	message := "no log entries"
	if opts.Since > 0 {
		message += fmt.Sprintf(" newer than %s", opts.Since)
	}
	if opts.Grep != nil {
		message += " matching the grep expression"
	}
	return message
}

// podLogOptions builds the kubelet request for one container stream.
func podLogOptions(opts LogsOptions, container string) *corev1.PodLogOptions {
	tailLines := opts.Limit
	if opts.Grep != nil && tailLines < grepTailLines {
		tailLines = grepTailLines
	}
	logOptions := &corev1.PodLogOptions{
		Container:  container,
		TailLines:  &tailLines,
		Previous:   opts.Previous,
		Timestamps: true,
	}
	if opts.Since > 0 {
		sinceSeconds := int64(math.Ceil(opts.Since.Seconds()))
		logOptions.SinceSeconds = &sinceSeconds
	}
	return logOptions
}

// resolveLogSelection resolves the named resource to newest-first pods and
// builds the capped set of regular-container log streams. A workload that
// exists but owns no pods yields a selection without targets.
func resolveLogSelection(ctx context.Context, clientset kubernetes.Interface, kind, name, namespace, container string) (*logSelection, error) {
	canonicalKind, err := canonicalLogKind(kind)
	if err != nil {
		return nil, err
	}

	pods, err := resolveLogPods(ctx, clientset, canonicalKind, name, namespace)
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		return &logSelection{kind: canonicalKind}, nil
	}

	sort.SliceStable(pods, func(i, j int) bool {
		if pods[i].CreationTimestamp.Equal(&pods[j].CreationTimestamp) {
			return pods[i].Name < pods[j].Name
		}
		return pods[i].CreationTimestamp.After(pods[j].CreationTimestamp.Time)
	})

	selection := &logSelection{
		kind:      canonicalKind,
		podsTotal: len(pods),
	}
	if len(pods) > maxLogPods {
		pods = pods[:maxLogPods]
		selection.truncated = true
	}
	selection.pods = pods
	selection.tagPod = len(pods) > 1

	allTargets := make([]podlogs.LogTarget, 0)
	for _, pod := range pods {
		if container != "" {
			allTargets = append(allTargets, podlogs.LogTarget{Pod: pod.Name, Container: container})
			continue
		}
		for _, podContainer := range pod.Spec.Containers {
			allTargets = append(allTargets, podlogs.LogTarget{Pod: pod.Name, Container: podContainer.Name})
		}
	}
	if len(allTargets) == 0 {
		return nil, fmt.Errorf("no containers found for %s %s/%s", canonicalKind, namespace, name)
	}
	if len(allTargets) > maxLogStreams {
		allTargets = allTargets[:maxLogStreams]
		selection.truncated = true
	}
	selection.targets = allTargets

	containerNames := make([]string, 0, len(allTargets))
	for _, target := range allTargets {
		containerNames = append(containerNames, target.Container)
	}
	selection.containers, _ = podlogs.DedupeNames(containerNames, len(containerNames))
	selection.tagContainer = len(selection.containers) > 1

	return selection, nil
}

// canonicalLogKind validates a case-insensitive kind and returns its canonical
// Kubernetes spelling. An empty kind defaults to Pod.
func canonicalLogKind(kind string) (string, error) {
	switch strings.ToLower(kind) {
	case "", "pod":
		return "Pod", nil
	case "deployment":
		return "Deployment", nil
	case "statefulset":
		return "StatefulSet", nil
	case "daemonset":
		return "DaemonSet", nil
	case "cronjob":
		return "CronJob", nil
	case "job":
		return "Job", nil
	default:
		return "", fmt.Errorf("unsupported kind %q; supported kinds are Pod, Deployment, StatefulSet, DaemonSet, CronJob and Job", kind)
	}
}

// resolveLogPods returns all pods owned by the named canonical resource kind.
func resolveLogPods(ctx context.Context, clientset kubernetes.Interface, kind, name, namespace string) ([]corev1.Pod, error) {
	switch kind {
	case "Pod":
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return []corev1.Pod{*pod}, nil
	case "Deployment":
		obj, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return resolveAppsWorkloadPods(ctx, clientset, obj, namespace, name)
	case "StatefulSet":
		obj, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return resolveAppsWorkloadPods(ctx, clientset, obj, namespace, name)
	case "DaemonSet":
		obj, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return resolveAppsWorkloadPods(ctx, clientset, obj, namespace, name)
	case "CronJob":
		cronJob, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return resolveCronJobPods(ctx, clientset, cronJob)
	case "Job":
		job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return resolveJobPods(ctx, clientset, job)
	default:
		return nil, fmt.Errorf("unsupported canonical kind %q", kind)
	}
}

// resolveAppsWorkloadPods selects pods by the full workload selector and then
// verifies the apps/v1 owner name prefix used by workload controllers.
func resolveAppsWorkloadPods(ctx context.Context, clientset kubernetes.Interface, obj runtime.Object, namespace, name string) ([]corev1.Pod, error) {
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	selector := podlogs.WorkloadPodSelector(&unstructured.Unstructured{Object: content})
	if selector == nil || selector.Empty() {
		return nil, nil
	}
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	pods := make([]corev1.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		for _, owner := range pod.OwnerReferences {
			if owner.APIVersion == appsv1.SchemeGroupVersion.String() && strings.HasPrefix(owner.Name, name) {
				pods = append(pods, pod)
				break
			}
		}
	}
	return pods, nil
}

// resolveCronJobPods traverses the CronJob to Job to Pod ownership chain.
func resolveCronJobPods(ctx context.Context, clientset kubernetes.Interface, cronJob *batchv1.CronJob) ([]corev1.Pod, error) {
	jobList, err := clientset.BatchV1().Jobs(cronJob.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	ownedJobs := make(map[types.UID]string)
	for _, job := range jobList.Items {
		if hasOwnerUID(job.OwnerReferences, cronJob.UID) {
			ownedJobs[job.UID] = job.Name
		}
	}
	if len(ownedJobs) == 0 {
		return nil, nil
	}

	podList, err := clientset.CoreV1().Pods(cronJob.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		jobName := pod.Labels[batchv1.JobNameLabel]
		for jobUID, ownedJobName := range ownedJobs {
			if jobName == ownedJobName && hasOwnerUID(pod.OwnerReferences, jobUID) {
				pods = append(pods, pod)
				break
			}
		}
	}
	return pods, nil
}

// resolveJobPods returns pods carrying the job-name label and owned by the Job.
func resolveJobPods(ctx context.Context, clientset kubernetes.Interface, job *batchv1.Job) ([]corev1.Pod, error) {
	selector := labels.Set{batchv1.JobNameLabel: job.Name}.String()
	podList, err := clientset.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if hasOwnerUID(pod.OwnerReferences, job.UID) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

// hasOwnerUID reports whether owner references include the given UID.
func hasOwnerUID(refs []metav1.OwnerReference, uid types.UID) bool {
	for _, ref := range refs {
		if ref.UID == uid {
			return true
		}
	}
	return false
}
