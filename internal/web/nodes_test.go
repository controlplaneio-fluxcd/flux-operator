// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// testCoreNode returns a Ready node with 4 cores, 8Gi of memory and 110 pods.
func testCoreNode(name string, mutate ...func(*corev1.Node)) corev1.Node {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{}},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
			},
			NodeInfo: corev1.NodeSystemInfo{KubeletVersion: "v1.33.4", Architecture: "arm64"},
		},
	}
	for _, m := range mutate {
		m(&node)
	}
	return node
}

// testCorePod returns a running pod on the node with the given
// memory request and limit (empty for unset).
func testCorePod(namespace, name, nodeName, memRequest, memLimit string) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: corev1.PodSpec{
			NodeName:   nodeName,
			Containers: []corev1.Container{{Name: "main", Image: "ghcr.io/org/app:v1"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	res := &pod.Spec.Containers[0].Resources
	if memRequest != "" {
		res.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse(memRequest),
		}
	}
	if memLimit != "" {
		res.Limits = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse(memLimit)}
	}
	return pod
}

// nodeMetricsList returns a NodeMetricsList with the given usage in
// cores and GiB per node.
func nodeMetricsList(usage map[string][2]float64) *metricsv1beta1api.NodeMetricsList {
	list := &metricsv1beta1api.NodeMetricsList{}
	for name, u := range usage {
		list.Items = append(list.Items, metricsv1beta1api.NodeMetrics{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Usage: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(u[0]*1000), resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(int64(u[1]*(1<<30)), resource.BinarySI),
			},
		})
	}
	return list
}

// podMetricsList returns a PodMetricsList with the given memory usage
// in GiB per namespace/name.
func podMetricsList(usage map[[2]string]float64) *metricsv1beta1api.PodMetricsList {
	list := &metricsv1beta1api.PodMetricsList{}
	for key, mem := range usage {
		list.Items = append(list.Items, metricsv1beta1api.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{Namespace: key[0], Name: key[1]},
			Containers: []metricsv1beta1api.ContainerMetrics{{
				Name: "main",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(int64(mem*(1<<30)), resource.BinarySI),
				},
			}},
		})
	}
	return list
}

// newTestCollector returns a collector with a node lister, without
// scraping; data is fed with ingest and ingestNodes.
func newTestCollector() *MetricsCollector {
	mc := NewMetricsCollector(nil, time.Minute)
	mc.nodeLister = func(context.Context) (*metricsv1beta1api.NodeMetricsList, error) {
		return &metricsv1beta1api.NodeMetricsList{}, nil
	}
	return mc
}

func TestNewNodesSnapshot_Aggregation(t *testing.T) {
	g := NewWithT(t)
	now := time.Now()

	sidecar := corev1.ContainerRestartPolicyAlways
	placeholder := testCorePod("autoscaler", "overprovisioning", "w1", "1Gi", "1Gi")
	placeholder.Spec.Priority = new(int32(-10))
	placeholder.Spec.Containers[0].Image = "registry.k8s.io/pause:3.10@sha256:abc"

	noRequest := testCorePod("apps", "no-request", "w1", "", "")
	noRequest.Status.Phase = corev1.PodPending

	noLimit := testCorePod("monitoring", "no-limit", "w1", "512Mi", "")
	noLimit.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "main",
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Reason: "OOMKilled", FinishedAt: metav1.NewTime(now.Add(-2 * time.Hour)),
		}},
	}}

	oomKilled := testCorePod("apps", "oom", "w1", "1Gi", "1Gi")
	oomKilled.Status.Phase = corev1.PodFailed
	oomKilled.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "main",
		State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Reason: "OOMKilled", FinishedAt: metav1.NewTime(now.Add(-10 * time.Minute)),
		}},
	}}

	bounded := testCorePod("apps", "api-abc12-xyz", "w1", "1Gi", "2Gi")
	bounded.Labels = map[string]string{"pod-template-hash": "abc12"}
	bounded.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "api-abc12", Controller: new(true)}}
	// A sidecar without a memory limit makes the pod unbounded.
	withSidecar := testCorePod("apps", "sidecar", "w2", "256Mi", "256Mi")
	withSidecar.Spec.InitContainers = []corev1.Container{{
		Name: "proxy", RestartPolicy: &sidecar,
		Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")}},
	}}

	pendingMemory := testCorePod("apps", "pending-1", "", "1Gi", "1Gi")
	pendingMemory.Status = corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable,
		Message: "0/5 nodes are available: 1 node(s) had untolerated taint {node-role.kubernetes.io/control-plane: }, 3 Insufficient memory. preemption: 0/5 nodes are available: 5 No preemption victims found for incoming pod.",
	}}}
	pendingTaint := pendingMemory
	pendingTaint.Name = "pending-2"
	pendingTaint.Status.Conditions = []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable,
		Message: "0/5 nodes are available: 4 node(s) had untolerated taint {dedicated: gpu}, 1 Insufficient cpu.",
	}}
	pendingScheduling := testCorePod("apps", "pending-3", "", "1Gi", "1Gi")
	pendingScheduling.Status.Phase = corev1.PodPending

	nodes := []corev1.Node{
		testCoreNode("w1", func(n *corev1.Node) {
			n.Labels["karpenter.sh/nodepool"] = "general"
			n.Labels["node-role.kubernetes.io/worker"] = ""
			n.Labels[corev1.LabelTopologyZone] = "zone-a"
			n.Labels[corev1.LabelInstanceTypeStable] = "standard-4"
		}),
		testCoreNode("w2", func(n *corev1.Node) {
			n.Labels["node-role.kubernetes.io/worker"] = ""
			n.Labels["node-role.kubernetes.io/infra"] = ""
		}),
		testCoreNode("cp", func(n *corev1.Node) {
			n.Labels["node-role.kubernetes.io/control-plane"] = ""
			n.Spec.Taints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}
		}),
		testCoreNode("down", func(n *corev1.Node) {
			n.Status.Conditions[0].Status = corev1.ConditionFalse
		}),
		testCoreNode("new", func(n *corev1.Node) {
			n.CreationTimestamp = metav1.NewTime(now)
			n.Status.Conditions = nil
		}),
	}

	mc := newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"w1": {2, 4}, "w2": {1, 2}, "cp": {1, 3}}), now)
	mc.ingest(podMetricsList(map[[2]string]float64{
		{"apps", "api-abc12-xyz"}:  1.5,
		{"monitoring", "no-limit"}: 0.25,
	}), now)

	renew := metav1.NewMicroTime(now.Add(-12 * time.Second))
	snap := newNodesSnapshot(nodesInput{
		now:           now,
		nodes:         nodes,
		pods:          []corev1.Pod{placeholder, noRequest, noLimit, oomKilled, bounded, withSidecar, pendingMemory, pendingTaint, pendingScheduling},
		leases:        []coordinationv1.Lease{{ObjectMeta: metav1.ObjectMeta{Name: "w1"}, Spec: coordinationv1.LeaseSpec{RenewTime: &renew}}},
		serverVersion: "v1.33.4",
		metrics:       mc,
		managedWorkloads: managedWorkloadKeys([]reporter.WorkloadRef{
			{Kind: "Deployment", Namespace: "apps", Name: "api"},
		}),
	})

	g.Expect(snap.MetricsAvailable).To(BeTrue())
	g.Expect(snap.PodMetricsAvailable).To(BeTrue())
	g.Expect(snap.Nodes).To(HaveLen(5))

	byName := map[string]*NodeDetail{}
	for _, n := range snap.Nodes {
		byName[n.Name] = n
	}

	// Label resolution and status.
	w1, w2, cp := byName["w1"], byName["w2"], byName["cp"]
	g.Expect(w1.Pool).To(Equal("general"))
	g.Expect(w1.Zone).To(Equal("zone-a"))
	g.Expect(w1.InstanceType).To(Equal("standard-4"))
	g.Expect(w2.Pool).To(Equal("infra"))
	g.Expect(w2.Roles).To(Equal([]string{"infra", "worker"}))
	g.Expect(w2.Zone).To(BeEmpty())
	g.Expect(cp.ControlPlane).To(BeTrue())
	g.Expect(cp.Pool).To(Equal("control-plane"))
	g.Expect(byName["down"].Status).To(Equal(NodeStatusNotReady))
	g.Expect(byName["new"].Status).To(Equal(NodeStatusUnknown))

	// Pod aggregation on w1: the placeholder, the pending-with-node, the
	// no-limit and the bounded pods take slots, the failed pod does not.
	g.Expect(w1.Pods).To(Equal(4))
	g.Expect(w1.Requests.Memory).To(Equal(int64(1<<30 + 512<<20 + 1<<30)))
	g.Expect(w1.Requests.CPU).To(BeNumerically("~", 1.5, 1e-9))
	g.Expect(w1.Limits.Memory).To(Equal(int64(3 << 30)))
	g.Expect(w1.placeholderRequests.Memory).To(Equal(int64(1 << 30)))
	g.Expect(w1.PodsWithoutMemoryRequest).To(Equal(1))
	g.Expect(w1.PodsWithoutMemoryLimit).To(Equal(1))
	g.Expect(w1.OOMKills).To(Equal(1))
	g.Expect(w2.PodsWithoutMemoryLimit).To(Equal(1))

	// Usage, above requests and heartbeat.
	g.Expect(w1.Usage.CPU).To(BeNumerically("~", 2, 1e-9))
	g.Expect(w1.Usage.Memory).To(Equal(int64(4 << 30)))
	g.Expect(w1.AboveRequests.Memory).To(Equal(int64(512 << 20)))
	g.Expect(w2.AboveRequests.Memory).To(BeZero())
	g.Expect(*w1.HeartbeatSeconds).To(Equal(12))
	g.Expect(w2.HeartbeatSeconds).To(BeNil())
	g.Expect(w1.ReadyTransitions).To(BeNil())
	g.Expect(byName["new"].Usage).To(BeNil())

	// Capacity: w1 and w2 only (control-plane, not ready and unknown are left out).
	g.Expect(*snap.Capacity.CPU.Allocatable).To(BeNumerically("~", 8, 1e-9))
	g.Expect(*snap.Capacity.Memory.Allocatable).To(BeNumerically("~", 16<<30, 1))
	g.Expect(*snap.Capacity.CPU.Used).To(BeNumerically("~", 3, 1e-9))
	g.Expect(*snap.Capacity.Memory.Used).To(BeNumerically("~", 6<<30, 1))
	g.Expect(*snap.Capacity.Memory.AboveRequests).To(BeNumerically("~", 512<<20, 1))
	g.Expect(*snap.Capacity.Pods.Allocatable).To(Equal(int64(220)))
	g.Expect(*snap.Capacity.Pods.Running).To(Equal(int64(5)))
	g.Expect(snap.Usage.Samples).To(HaveLen(1))
	g.Expect(snap.Usage.Samples[0].Memory).To(Equal(int64(6 << 30)))

	// Unschedulable pods with their top reasons.
	g.Expect(snap.UnschedulablePods).To(Equal(2))
	g.Expect(snap.UnschedulableReasons).To(Equal([]UnschedulableReason{
		{Reason: "Insufficient memory", Pods: 1},
		{Reason: "untolerated taint", Pods: 1},
	}))
	g.Expect(snap.UnboundedNamespaces.WithoutRequest).To(Equal([]NamespacePods{{Namespace: "apps", Pods: 1}}))
	g.Expect(snap.UnboundedNamespaces.WithoutLimit).To(Equal([]NamespacePods{
		{Namespace: "apps", Pods: 1}, {Namespace: "monitoring", Pods: 1},
	}))

	// Bursting workloads: the Deployment pod is 0.5Gi above its request.
	g.Expect(snap.workloads).To(HaveLen(1))
	w := snap.workloads[0]
	g.Expect(w.Kind).To(Equal("Deployment"))
	g.Expect(w.Name).To(Equal("api"))
	g.Expect(w.Pods).To(Equal(1))
	g.Expect(w.Nodes).To(Equal(1))
	g.Expect(w.AboveRequests).To(Equal(int64(512 << 20)))
	g.Expect(*w.Limits).To(Equal(int64(2 << 30)))
	g.Expect(w.NoLimit).To(BeFalse())

	// Findings and checks are computed.
	g.Expect(findLine(snap, CodeNotReady)).NotTo(BeNil())
	g.Expect(findLine(snap, CodeUnschedulablePods)).NotTo(BeNil())
	g.Expect(findLine(snap, CodeOOMKills)).NotTo(BeNil())
	g.Expect(findCheck(snap, "Heartbeat").Status).To(Equal(CheckPassed))
	g.Expect(findCheck(snap, "Heartbeat").Fact).To(Equal("1 of 5 leases renewed within 20s"))
	g.Expect(findCheck(snap, "Node readiness").Status).To(Equal(CheckRaised))
	g.Expect(findCheck(snap, "Readiness flapping").Status).To(Equal(CheckSkipped))
	g.Expect(snap.summary.Down).To(Equal(1))
}

func TestNewNodesSnapshot_NoEligibleNodes(t *testing.T) {
	g := NewWithT(t)

	snap := newNodesSnapshot(nodesInput{
		now: time.Now(),
		nodes: []corev1.Node{testCoreNode("down", func(n *corev1.Node) {
			n.Status.Conditions[0].Status = corev1.ConditionUnknown
		})},
	})
	g.Expect(snap.Capacity.CPU.Allocatable).To(BeNil())
	g.Expect(snap.Capacity.Memory.Requested).To(BeNil())
	g.Expect(snap.Capacity.Pods.Running).To(BeNil())
}

func TestNewNodesSnapshot_MetricsAbsent(t *testing.T) {
	g := NewWithT(t)

	snap := newNodesSnapshot(nodesInput{
		now:   time.Now(),
		nodes: []corev1.Node{testCoreNode("w1")},
		pods:  []corev1.Pod{testCorePod("apps", "app", "w1", "1Gi", "1Gi")},
	})
	g.Expect(snap.MetricsAvailable).To(BeFalse())
	g.Expect(snap.PodMetricsAvailable).To(BeFalse())
	g.Expect(snap.Nodes[0].Usage).To(BeNil())
	g.Expect(snap.Nodes[0].AboveRequests).To(BeNil())
	g.Expect(snap.Capacity.CPU.Used).To(BeNil())
	g.Expect(snap.Capacity.Memory.AboveRequests).To(BeNil())
	g.Expect(*snap.Capacity.Memory.Requested).To(BeNumerically("~", 1<<30, 1))
	g.Expect(findCheck(snap, "Memory usage").Status).To(Equal(CheckSkipped))
	g.Expect(findCheck(snap, "Heartbeat").Status).To(Equal(CheckSkipped))

	// Empty lists serialize as arrays.
	b, err := json.Marshal(snap)
	g.Expect(err).NotTo(HaveOccurred())
	var raw map[string]any
	g.Expect(json.Unmarshal(b, &raw)).To(Succeed())
	g.Expect(raw["usage"]).To(Equal(map[string]any{"samples": []any{}}))
	g.Expect(raw["unschedulableReasons"]).To(Equal([]any{}))
	node := raw["nodes"].([]any)[0].(map[string]any)
	g.Expect(node["usage"]).To(BeNil())
	g.Expect(node["findings"]).To(Equal([]any{}))
}

func TestNewNodesSnapshot_IndependentMetricsFailure(t *testing.T) {
	g := NewWithT(t)
	nodes := []corev1.Node{testCoreNode("w1")}
	pods := []corev1.Pod{testCorePod("apps", "app", "w1", "1Gi", "1Gi")}

	// Node metrics failing, pod metrics working.
	mc := NewMetricsCollector(func(context.Context) (*metricsv1beta1api.PodMetricsList, error) {
		return podMetricsList(map[[2]string]float64{{"apps", "app"}: 2}), nil
	}, time.Minute)
	mc.nodeLister = func(context.Context) (*metricsv1beta1api.NodeMetricsList, error) {
		return nil, errors.New("forbidden")
	}
	mc.scrape(context.Background())
	g.Expect(mc.Available()).To(BeTrue())
	g.Expect(mc.NodesAvailable()).To(BeFalse())
	g.Expect(mc.nodeState.failing).To(BeTrue())
	g.Expect(mc.failing).To(BeFalse())

	snap := newNodesSnapshot(nodesInput{now: time.Now(), nodes: nodes, pods: pods, metrics: mc})
	g.Expect(snap.MetricsAvailable).To(BeFalse())
	g.Expect(snap.PodMetricsAvailable).To(BeTrue())
	g.Expect(snap.Nodes[0].Usage).To(BeNil())
	g.Expect(snap.Nodes[0].AboveRequests.Memory).To(Equal(int64(1 << 30)))
	g.Expect(snap.Capacity.Memory.Used).To(BeNil())
	g.Expect(snap.Capacity.Memory.AboveRequests).To(BeNil())

	// Pod metrics failing, node metrics working.
	mc = NewMetricsCollector(func(context.Context) (*metricsv1beta1api.PodMetricsList, error) {
		return nil, errors.New("timeout")
	}, time.Minute)
	mc.nodeLister = func(context.Context) (*metricsv1beta1api.NodeMetricsList, error) {
		return nodeMetricsList(map[string][2]float64{"w1": {1, 2}}), nil
	}
	mc.scrape(context.Background())
	g.Expect(mc.Available()).To(BeFalse())
	g.Expect(mc.NodesAvailable()).To(BeTrue())

	snap = newNodesSnapshot(nodesInput{now: time.Now(), nodes: nodes, pods: pods, metrics: mc})
	g.Expect(snap.MetricsAvailable).To(BeTrue())
	g.Expect(snap.PodMetricsAvailable).To(BeFalse())
	g.Expect(snap.Nodes[0].Usage.Memory).To(Equal(int64(2 << 30)))
	g.Expect(snap.Nodes[0].AboveRequests).To(BeNil())
	g.Expect(*snap.Capacity.Memory.Used).To(BeNumerically("~", 2<<30, 1))
	g.Expect(snap.Capacity.Memory.AboveRequests).To(BeNil())
	g.Expect(snap.workloads).To(BeEmpty())

	// Stale pod samples relative to the node samples are not used.
	mc = newTestCollector()
	now := time.Now()
	mc.ingest(podMetricsList(map[[2]string]float64{{"apps", "app"}: 2}), now.Add(-3*time.Minute))
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"w1": {1, 2}}), now)
	mc.lastSuccess = now.Add(-2*time.Minute - time.Second)
	snap = newNodesSnapshot(nodesInput{now: now, nodes: nodes, pods: pods, metrics: mc})
	g.Expect(snap.PodMetricsAvailable).To(BeFalse())
}

func TestPodHelpers(t *testing.T) {
	g := NewWithT(t)

	// Placeholder pods: negative priority and pause images only.
	pod := testCorePod("ns", "p", "n", "1Gi", "")
	pod.Spec.Containers[0].Image = "localhost:5000/pause"
	g.Expect(isPlaceholderPod(&pod)).To(BeFalse())
	pod.Spec.Priority = new(int32(-1))
	g.Expect(isPlaceholderPod(&pod)).To(BeTrue())
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Image: "busybox"})
	g.Expect(isPlaceholderPod(&pod)).To(BeFalse())

	g.Expect(imageBasename("registry.k8s.io/pause:3.10")).To(Equal("pause"))
	g.Expect(imageBasename("pause@sha256:abc")).To(Equal("pause"))
	g.Expect(imageBasename("ghcr.io/org/pause-app:v1")).To(Equal("pause-app"))

	// Memory bounds over app and sidecar containers.
	tests := []struct {
		name           string
		mutate         func(*corev1.Pod)
		withoutRequest bool
		withoutLimit   bool
	}{
		{name: "bounded", mutate: func(p *corev1.Pod) {
			p.Spec.Containers[0].Resources.Limits = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}
		}},
		{name: "no resources", mutate: func(p *corev1.Pod) {
			p.Spec.Containers[0].Resources = corev1.ResourceRequirements{}
		}, withoutRequest: true},
		{name: "zero request", mutate: func(p *corev1.Pod) {
			p.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse("0")
		}, withoutRequest: true},
		{name: "request without limit", mutate: func(*corev1.Pod) {}, withoutLimit: true},
		{name: "pod-level request", mutate: func(p *corev1.Pod) {
			p.Spec.Containers[0].Resources = corev1.ResourceRequirements{}
			p.Spec.Resources = &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			}
		}},
		{name: "zero pod-level request", mutate: func(p *corev1.Pod) {
			p.Spec.Containers[0].Resources = corev1.ResourceRequirements{}
			p.Spec.Resources = &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("0")},
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			}
		}, withoutRequest: true},
		{name: "zero pod-level limit", mutate: func(p *corev1.Pod) {
			p.Spec.Resources = &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("0")},
			}
		}, withoutLimit: true},
		{name: "mixed containers", mutate: func(p *corev1.Pod) {
			p.Spec.Containers[0].Resources.Limits = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}
			p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: "helper"})
		}, withoutRequest: true},
	}
	for _, tt := range tests {
		p := testCorePod("ns", "p", "n", "1Gi", "")
		tt.mutate(&p)
		noRequest, noLimit := podMemoryBounds(&p)
		g.Expect(noRequest).To(Equal(tt.withoutRequest), tt.name)
		g.Expect(noLimit).To(Equal(tt.withoutLimit), tt.name)
	}

	// OOM kills count once per container, from its last known termination.
	now := time.Now()
	oom := func(ago time.Duration) *corev1.ContainerStateTerminated {
		return &corev1.ContainerStateTerminated{Reason: "OOMKilled", FinishedAt: metav1.NewTime(now.Add(-ago))}
	}
	p := testCorePod("ns", "p", "n", "", "")
	p.Status.ContainerStatuses = []corev1.ContainerStatus{
		// Terminated twice by OOM: counted once.
		{Name: "a", State: corev1.ContainerState{Terminated: oom(time.Minute)},
			LastTerminationState: corev1.ContainerState{Terminated: oom(2 * time.Minute)}},
		// Running after an OOM kill.
		{Name: "b", LastTerminationState: corev1.ContainerState{Terminated: oom(5 * time.Minute)}},
		// The last termination is not an OOM kill.
		{Name: "c", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error"}},
			LastTerminationState: corev1.ContainerState{Terminated: oom(time.Minute)}},
		// Too old.
		{Name: "d", LastTerminationState: corev1.ContainerState{Terminated: oom(61 * time.Minute)}},
	}
	g.Expect(podOOMKills(&p, now)).To(Equal(2))

	// Scheduler messages.
	g.Expect(topSchedulerReason("0/3 nodes are available: 1 Too many pods, 2 node(s) didn't match Pod's node affinity/selector. preemption: x")).
		To(Equal("didn't match Pod's node affinity/selector"))
	g.Expect(topSchedulerReason("0/3 nodes are available: pod has unbound immediate PersistentVolumeClaims.")).
		To(Equal("pod has unbound immediate PersistentVolumeClaims"))
	g.Expect(topSchedulerReason("no match")).To(BeEmpty())

	// Workload resolution.
	jobOwner := func(_, name string) string {
		if name == "backup-123" {
			return "backup"
		}
		return ""
	}
	p = testCorePod("ns", "p", "n", "", "")
	kind, name := podWorkload(&p, jobOwner)
	g.Expect([]string{kind, name}).To(Equal([]string{"Pod", "p"}))
	p.OwnerReferences = []metav1.OwnerReference{{Kind: "Job", Name: "backup-123", Controller: new(true)}}
	kind, name = podWorkload(&p, jobOwner)
	g.Expect([]string{kind, name}).To(Equal([]string{"CronJob", "backup"}))
	p.OwnerReferences = []metav1.OwnerReference{{Kind: "StatefulSet", Name: "db", Controller: new(true)}}
	kind, name = podWorkload(&p, jobOwner)
	g.Expect([]string{kind, name}).To(Equal([]string{"StatefulSet", "db"}))
	p.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "standalone", Controller: new(true)}}
	kind, name = podWorkload(&p, jobOwner)
	g.Expect([]string{kind, name}).To(Equal([]string{"Pod", "p"}))
	p.OwnerReferences = []metav1.OwnerReference{{Kind: "Job", Name: "manual", Controller: new(true)}}
	kind, name = podWorkload(&p, jobOwner)
	g.Expect([]string{kind, name}).To(Equal([]string{"Pod", "p"}))
}

func TestTransformNode(t *testing.T) {
	g := NewWithT(t)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n", ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubelet"}}},
		Status:     corev1.NodeStatus{Images: []corev1.ContainerImage{{Names: []string{"img"}}}},
	}
	obj, err := TransformNode(node)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj.(*corev1.Node).Status.Images).To(BeNil())
	g.Expect(obj.(*corev1.Node).ManagedFields).To(BeNil())
}

// grantNodesAccess grants list nodes to the user and get resourcesets in
// the given namespace, so the user sees only that namespace.
func grantNodesAccess(t *testing.T, g *WithT, username, namespace string) {
	t.Helper()
	objs := []client.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: username + "-nodes"},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"list"}}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: username + "-nodes"},
			Subjects:   []rbacv1.Subject{{Kind: "User", Name: username}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: username + "-nodes"},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: username + "-rsets", Namespace: namespace},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{fluxcdv1.GroupVersion.Group}, Resources: []string{"resourcesets"}, Verbs: []string{"get"},
			}},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: username + "-rsets", Namespace: namespace},
			Subjects:   []rbacv1.Subject{{Kind: "User", Name: username}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: username + "-rsets"},
		},
	}
	for _, obj := range objs {
		g.Expect(testClient.Create(ctx, obj)).To(Succeed())
		t.Cleanup(func() { _ = testClient.Delete(context.Background(), obj) })
	}
}

// userSession returns a context with a session for the given user.
func userSession(g *WithT, kc *kubeclient.Client, username string) context.Context {
	imp := user.Impersonation{Username: username, Groups: []string{"nodes-test-group"}}
	userClient, err := kc.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	return user.StoreSession(ctx, user.Details{Impersonation: imp}, userClient)
}

func TestNodesHandlers_RBAC(t *testing.T) {
	g := NewWithT(t)

	for _, name := range []string{"nodes-rbac-a", "nodes-rbac-b"} {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		g.Expect(testClient.Create(ctx, ns)).To(Succeed())
		t.Cleanup(func() { _ = testClient.Delete(context.Background(), ns) })
	}

	kc, err := kubeclient.New(testClient, testClient, testEnv.Config, testScheme, 100, time.Minute)
	g.Expect(err).NotTo(HaveOccurred())
	handler := &Handler{
		kubeClient: kc,
		nodesSnapshot: &NodesSnapshot{
			PodMetricsAvailable: true,
			Nodes:               []*NodeDetail{readyNode("node-1")},
			workloads: []BurstingWorkload{
				{Kind: "Deployment", Namespace: "nodes-rbac-b", Name: "b", AboveRequests: 300},
				{Kind: "Deployment", Namespace: "nodes-rbac-a", Name: "a", AboveRequests: 200},
			},
		},
	}

	serve := func(reqCtx context.Context, path string, h http.HandlerFunc) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(reqCtx)
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	// Users who can't list nodes get a 403 on both endpoints.
	deniedCtx := userSession(g, kc, "nodes-denied-user")
	g.Expect(serve(deniedCtx, "/api/v1/nodes", handler.NodesHandler).Code).To(Equal(http.StatusForbidden))
	g.Expect(serve(deniedCtx, "/api/v1/nodes/workloads", handler.NodesWorkloadsHandler).Code).To(Equal(http.StatusForbidden))

	// Without authentication the dashboard is open.
	rec := serve(ctx, "/api/v1/nodes", handler.NodesHandler)
	g.Expect(rec.Code).To(Equal(http.StatusOK))
	var snap map[string]any
	g.Expect(json.Unmarshal(rec.Body.Bytes(), &snap)).To(Succeed())
	g.Expect(snap["nodes"]).To(HaveLen(1))
	g.Expect(snap).NotTo(HaveKey("workloads"))

	// A user who can list nodes sees the workloads of their namespaces only.
	grantNodesAccess(t, g, "nodes-allowed-user", "nodes-rbac-a")
	allowedCtx := userSession(g, kc, "nodes-allowed-user")
	g.Eventually(func() int {
		return serve(allowedCtx, "/api/v1/nodes", handler.NodesHandler).Code
	}, 10*time.Second, 200*time.Millisecond).Should(Equal(http.StatusOK))

	rec = serve(allowedCtx, "/api/v1/nodes/workloads", handler.NodesWorkloadsHandler)
	g.Expect(rec.Code).To(Equal(http.StatusOK))
	var workloads NodesWorkloads
	g.Expect(json.Unmarshal(rec.Body.Bytes(), &workloads)).To(Succeed())
	g.Expect(workloads.MetricsAvailable).To(BeTrue())
	g.Expect(workloads.Workloads).To(HaveLen(1))
	g.Expect(workloads.Workloads[0].Name).To(Equal("a"))

	// Without auth, all namespaces are visible, ranked by aboveRequests.
	rec = serve(ctx, "/api/v1/nodes/workloads", handler.NodesWorkloadsHandler)
	g.Expect(json.Unmarshal(rec.Body.Bytes(), &workloads)).To(Succeed())
	g.Expect(workloads.Workloads).To(HaveLen(2))
	g.Expect(workloads.Workloads[0].Name).To(Equal("b"))
}

func TestGetReport_NodesSummaryAndCanViewNodes(t *testing.T) {
	g := NewWithT(t)

	// A node with node-sourced strings that must never reach the summary.
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: "secret-node-name.example.internal",
		Labels: map[string]string{
			"karpenter.sh/nodepool":         "secret-pool",
			corev1.LabelTopologyZone:        "secret-zone",
			corev1.LabelInstanceTypeStable:  "secret-instance",
			"node-role.kubernetes.io/infra": "",
		},
	}, Spec: corev1.NodeSpec{
		Taints: []corev1.Taint{{Key: "secret-taint-key", Effect: corev1.TaintEffectNoSchedule}},
	}}
	g.Expect(testClient.Create(ctx, node)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(context.Background(), node) })
	node.Status = corev1.NodeStatus{
		Allocatable: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("4"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
			corev1.ResourcePods:   resource.MustParse("110"),
		},
		Conditions: []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now(), LastHeartbeatTime: metav1.Now()},
			{Type: "SecretConditionType", Status: corev1.ConditionTrue, Message: "secret message",
				LastTransitionTime: metav1.Now(), LastHeartbeatTime: metav1.Now()},
		},
		NodeInfo: corev1.NodeSystemInfo{KubeletVersion: "v1.20.99-secret"},
	}
	g.Expect(testClient.Status().Update(ctx, node)).To(Succeed())

	kc, err := kubeclient.New(testClient, testClient, testEnv.Config, testScheme, 100, time.Minute)
	g.Expect(err).NotTo(HaveOccurred())
	handler := &Handler{
		kubeClient:    kc,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
	}
	handler.refreshReportCache(ctx)
	g.Expect(handler.getCachedNodesSnapshot()).NotTo(BeNil())

	// Without authentication canViewNodes is true.
	report, err := handler.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	spec := report.Object["spec"].(map[string]any)
	g.Expect(spec["userInfo"].(map[string]any)["canViewNodes"]).To(BeTrue())

	nodes, found := spec["nodes"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(nodes["findings"].(map[string]any)["warn"]).To(BeNumerically(">=", 1))
	b, err := json.Marshal(nodes)
	g.Expect(err).NotTo(HaveOccurred())
	for _, s := range []string{"secret", "example.internal", "infra", "v1.20"} {
		g.Expect(string(b)).NotTo(ContainSubstring(s))
	}

	// The full snapshot has them, for the gated dashboard.
	full, err := json.Marshal(handler.getCachedNodesSnapshot())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(full)).To(ContainSubstring("secret-pool"))

	// A user without list nodes gets false.
	userCtx := userSession(g, kc, "nodes-report-user")
	report, err = handler.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	userInfo := report.Object["spec"].(map[string]any)["userInfo"].(map[string]any)
	g.Expect(userInfo["canViewNodes"]).To(BeFalse())

	// A failed access check yields false and never fails the report.
	kc, err = kubeclient.New(testClient, testClient, testEnv.Config, testScheme, 100, time.Minute)
	g.Expect(err).NotTo(HaveOccurred())
	handler.kubeClient = kc
	userCtx = userSession(g, kc, "nodes-report-error-user")
	_, _, err = kc.ListUserNamespaces(userCtx) // cached, no API call below
	g.Expect(err).NotTo(HaveOccurred())
	canceledCtx, cancelCtx := context.WithCancel(userCtx)
	cancelCtx()
	report, err = handler.GetReport(canceledCtx)
	g.Expect(err).NotTo(HaveOccurred())
	userInfo = report.Object["spec"].(map[string]any)["userInfo"].(map[string]any)
	g.Expect(userInfo["canViewNodes"]).To(BeFalse())
}

// comparableClient wraps a client in a comparable type.
type comparableClient struct {
	client.WithWatch
}

func TestRefreshReportCache_SnapshotFailure(t *testing.T) {
	g := NewWithT(t)

	var failPods atomic.Bool
	base, err := client.NewWithWatch(testEnv.Config, client.Options{Scheme: testScheme})
	g.Expect(err).NotTo(HaveOccurred())
	failing := interceptor.NewClient(base, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*corev1.PodList); ok && failPods.Load() {
				return errors.New("pods unavailable")
			}
			return c.List(ctx, list, opts...)
		},
	})
	// The kubeclient compares clients, so the interceptor is wrapped in a pointer.
	kc, err := kubeclient.New(testClient, &comparableClient{failing}, testEnv.Config, testScheme, 100, time.Minute)
	g.Expect(err).NotTo(HaveOccurred())
	handler := &Handler{
		kubeClient:    kc,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
	}

	handler.refreshReportCache(ctx)
	g.Expect(handler.getCachedNodesSnapshot()).NotTo(BeNil())
	report, err := handler.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report.Object["spec"]).To(HaveKey("nodes"))

	// A failed snapshot drops both the cached snapshot and spec.nodes.
	failPods.Store(true)
	handler.refreshReportCache(ctx)
	g.Expect(handler.getCachedNodesSnapshot()).To(BeNil())
	report, err = handler.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report.Object["spec"]).NotTo(HaveKey("nodes"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.NodesHandler(rec, req)
	g.Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
}

func TestNewNodesSnapshot_BurstingWorkloads(t *testing.T) {
	g := NewWithT(t)
	now := time.Now()

	// Two Deployment pods and a static pod, all using memory above
	// their requests. Static pods are mirrored with their Node as owner.
	deployment := func(name string) corev1.Pod {
		pod := testCorePod("apps", name+"-abc12-xyz", "n1", "100Mi", "1Gi")
		pod.Labels = map[string]string{"pod-template-hash": "abc12"}
		pod.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: name + "-abc12", Controller: new(true)}}
		return pod
	}
	mirror := testCorePod("kube-system", "etcd-n1", "n1", "100Mi", "")
	mirror.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Node", Name: "n1", Controller: new(true)}}
	pods := []corev1.Pod{deployment("indexed"), deployment("unmanaged"), mirror}

	mc := newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"n1": {1, 3}}), now)
	mc.ingest(podMetricsList(map[[2]string]float64{
		{"apps", "indexed-abc12-xyz"}:   0.5,
		{"apps", "unmanaged-abc12-xyz"}: 0.5,
		{"kube-system", "etcd-n1"}:      0.5,
	}), now)

	snap := newNodesSnapshot(nodesInput{
		now:     now,
		nodes:   []corev1.Node{testCoreNode("n1")},
		pods:    pods,
		metrics: mc,
		managedWorkloads: managedWorkloadKeys([]reporter.WorkloadRef{
			{Kind: "Deployment", Namespace: "apps", Name: "indexed"},
		}),
	})

	// Only the Flux-managed Deployment is ranked: the non-indexed
	// Deployment and the mirror pod are excluded.
	g.Expect(snap.workloads).To(HaveLen(1))
	g.Expect(snap.workloads[0].Kind).To(Equal("Deployment"))
	g.Expect(snap.workloads[0].Name).To(Equal("indexed"))

	// The usage above requests of every pod still counts on the node.
	g.Expect(snap.Nodes[0].AboveRequests.Memory).To(Equal(int64(3 * (512<<20 - 100<<20))))

	// The workload metadata covers all its pods, including those missing
	// from the latest scrape: an unmeasured pod without a memory limit on
	// another node makes the workload unbounded.
	unmeasured := deployment("indexed")
	unmeasured.Name = "indexed-abc12-new"
	unmeasured.Spec.NodeName = "n2"
	unmeasured.Spec.Containers[0].Resources.Limits = nil
	snap = newNodesSnapshot(nodesInput{
		now:     now,
		nodes:   []corev1.Node{testCoreNode("n1"), testCoreNode("n2")},
		pods:    append(slices.Clone(pods), unmeasured),
		metrics: mc,
		managedWorkloads: managedWorkloadKeys([]reporter.WorkloadRef{
			{Kind: "Deployment", Namespace: "apps", Name: "indexed"},
		}),
	})
	g.Expect(snap.workloads).To(HaveLen(1))
	w := snap.workloads[0]
	g.Expect(w.Pods).To(Equal(2))
	g.Expect(w.Nodes).To(Equal(2))
	g.Expect(w.Requests).To(Equal(int64(200 << 20)))
	g.Expect(w.NoLimit).To(BeTrue())
	g.Expect(w.Limits).To(BeNil())
	g.Expect(w.Usage).To(Equal(int64(512 << 20)))
	g.Expect(w.AboveRequests).To(Equal(int64(512<<20 - 100<<20)))

	// Without managed workloads nothing is ranked.
	snap = newNodesSnapshot(nodesInput{now: now, nodes: []corev1.Node{testCoreNode("n1")}, pods: pods, metrics: mc})
	g.Expect(snap.workloads).To(BeEmpty())
}
