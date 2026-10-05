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
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

	// All nodes cordoned: no capacity and no usage series.
	cordon := func(n *corev1.Node) { n.Spec.Unschedulable = true }
	now := time.Now()
	mc := newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"w1": {1, 2}, "w2": {1, 2}}), now)
	snap = newNodesSnapshot(nodesInput{
		now:     now,
		nodes:   []corev1.Node{testCoreNode("w1", cordon), testCoreNode("w2", cordon)},
		pods:    []corev1.Pod{testCorePod("apps", "app", "w1", "1Gi", "1Gi")},
		metrics: mc,
	})
	g.Expect(snap.MetricsAvailable).To(BeTrue())
	g.Expect(snap.Usage.Samples).To(BeEmpty())
	g.Expect(snap.Capacity).To(Equal(NodesCapacity{}))
}

func TestNewNodesSnapshot_CordonedNodeNotCapacity(t *testing.T) {
	g := NewWithT(t)
	now := time.Now()

	mc := newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"w1": {1, 2}, "w2": {3, 4}}), now)
	snap := newNodesSnapshot(nodesInput{
		now: now,
		nodes: []corev1.Node{
			testCoreNode("w1"),
			testCoreNode("w2", func(n *corev1.Node) { n.Spec.Unschedulable = true }),
		},
		pods: []corev1.Pod{
			testCorePod("apps", "a", "w1", "1Gi", "1Gi"),
			testCorePod("apps", "b", "w2", "2Gi", "2Gi"),
			testCorePod("apps", "c", "w2", "2Gi", "2Gi"),
		},
		metrics: mc,
	})

	// The cordoned node keeps its own totals.
	var w2 *NodeDetail
	for _, n := range snap.Nodes {
		if n.Name == "w2" {
			w2 = n
		}
	}
	g.Expect(w2.Unschedulable).To(BeTrue())
	g.Expect(w2.Pods).To(Equal(2))
	g.Expect(w2.Requests.Memory).To(Equal(int64(4 << 30)))

	// Capacity totals and usage series count only w1.
	g.Expect(*snap.Capacity.CPU.Allocatable).To(BeNumerically("~", 4, 1e-9))
	g.Expect(*snap.Capacity.CPU.Requested).To(BeNumerically("~", 0.5, 1e-9))
	g.Expect(*snap.Capacity.Memory.Allocatable).To(BeNumerically("~", 8<<30, 1))
	g.Expect(*snap.Capacity.Memory.Requested).To(BeNumerically("~", 1<<30, 1))
	g.Expect(*snap.Capacity.Memory.Limits).To(BeNumerically("~", 1<<30, 1))
	g.Expect(*snap.Capacity.Pods.Allocatable).To(Equal(int64(110)))
	g.Expect(*snap.Capacity.Pods.Running).To(Equal(int64(1)))
	g.Expect(snap.Usage.Samples).NotTo(BeEmpty())
	last := snap.Usage.Samples[len(snap.Usage.Samples)-1]
	g.Expect(last.CPU).To(BeNumerically("~", 1, 1e-9))
	g.Expect(last.Memory).To(Equal(int64(2 << 30)))
	g.Expect(*snap.Capacity.CPU.Used).To(BeNumerically("~", 1, 1e-9))
	g.Expect(*snap.Capacity.Memory.Used).To(BeNumerically("~", 2<<30, 1))
}

func TestNewNodesSnapshot_UsageSeriesAllocatable(t *testing.T) {
	g := NewWithT(t)
	now := time.Now().Truncate(time.Second)
	tick := func(i int) time.Time { return now.Add(time.Duration(i-6) * time.Minute) }

	large := func(n *corev1.Node) {
		n.Status.Allocatable[corev1.ResourceCPU] = resource.MustParse("8")
		n.Status.Allocatable[corev1.ResourceMemory] = resource.MustParse("16Gi")
	}
	nodes := []corev1.Node{
		testCoreNode("w1"),
		testCoreNode("w2", large),
		testCoreNode("w3"),
		testCoreNode("cordoned", func(n *corev1.Node) { n.Spec.Unschedulable = true }),
		testCoreNode("down", func(n *corev1.Node) { n.Status.Conditions[0].Status = corev1.ConditionFalse }),
	}

	// w2 misses ticks 1 to 3 and w3 joins at tick 5. The cordoned and
	// not ready nodes report at every tick.
	usage := map[string][2]float64{"w1": {1, 1}, "w2": {2, 2}, "w3": {3, 3}, "cordoned": {10, 10}, "down": {20, 20}}
	present := [][]string{
		{"w1", "w2"},
		{"w1"},
		{"w1"},
		{"w1"},
		{"w1", "w2"},
		{"w1", "w2", "w3"},
		{"w1", "w2", "w3"},
	}
	mc := newTestCollector()
	for i, names := range present {
		list := map[string][2]float64{"cordoned": usage["cordoned"], "down": usage["down"]}
		for _, name := range names {
			list[name] = usage[name]
		}
		mc.ingestNodes(nodeMetricsList(list), tick(i))
	}
	snap := newNodesSnapshot(nodesInput{now: now, nodes: nodes, metrics: mc})

	// The first missed tick is dropped, the following ones count w1 only,
	// and w3 counts from its first tick.
	type point struct {
		Time        time.Time
		CPU         float64
		Allocatable float64
		Memory      int64
		AllocMemory int64
	}
	got := make([]point, 0, len(snap.Usage.Samples))
	for _, s := range snap.Usage.Samples {
		got = append(got, point{s.Time, s.CPU, s.AllocatableCPU, s.Memory, s.AllocatableMemory})
	}
	g.Expect(got).To(Equal([]point{
		{tick(0), 3, 12, 3 << 30, 24 << 30},
		{tick(2), 1, 4, 1 << 30, 8 << 30},
		{tick(3), 1, 4, 1 << 30, 8 << 30},
		{tick(4), 3, 12, 3 << 30, 24 << 30},
		{tick(5), 6, 16, 6 << 30, 32 << 30},
		{tick(6), 6, 16, 6 << 30, 32 << 30},
	}))
}

func TestNewNodesSnapshot_UsedOverSampledNodes(t *testing.T) {
	g := NewWithT(t)
	now := time.Now().Truncate(time.Second)
	nodes := []corev1.Node{testCoreNode("w1"), testCoreNode("w2"), testCoreNode("w3")}

	// w2 missed the latest scrape and w3 joins with its first sample.
	mc := newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"w1": {1, 1}, "w2": {2, 2}}), now.Add(-time.Minute))
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"w1": {1, 2}, "w3": {2, 3}}), now)
	snap := newNodesSnapshot(nodesInput{now: now, nodes: nodes, metrics: mc})

	g.Expect(*snap.Capacity.CPU.Allocatable).To(BeNumerically("~", 12, 1e-9))
	g.Expect(*snap.Capacity.CPU.Used).To(BeNumerically("~", 3, 1e-9))
	g.Expect(*snap.Capacity.CPU.UsedOf).To(BeNumerically("~", 8, 1e-9))
	g.Expect(*snap.Capacity.Memory.Allocatable).To(BeNumerically("~", 24<<30, 1))
	g.Expect(*snap.Capacity.Memory.Used).To(BeNumerically("~", 5<<30, 1))
	g.Expect(*snap.Capacity.Memory.UsedOf).To(BeNumerically("~", 16<<30, 1))

	// The latest tick is incomplete and left out of the series.
	g.Expect(snap.Usage.Samples).To(HaveLen(1))
	g.Expect(snap.Usage.Samples[0].Time).To(Equal(now.Add(-time.Minute)))

	// No eligible node sampled at the latest scrape.
	mc = newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"w1": {1, 1}}), now.Add(-time.Minute))
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"other": {1, 1}}), now)
	snap = newNodesSnapshot(nodesInput{now: now, nodes: nodes, metrics: mc})
	g.Expect(snap.MetricsAvailable).To(BeTrue())
	g.Expect(*snap.Capacity.CPU.Allocatable).To(BeNumerically("~", 12, 1e-9))
	g.Expect(snap.Capacity.CPU.Used).To(BeNil())
	g.Expect(snap.Capacity.CPU.UsedOf).To(BeNil())
	g.Expect(snap.Capacity.Memory.Used).To(BeNil())
	g.Expect(snap.Capacity.Memory.UsedOf).To(BeNil())
	g.Expect(snap.Capacity.Memory.AboveRequests).To(BeNil())
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

	meta := &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Node"},
		ObjectMeta: metav1.ObjectMeta{Name: "n", ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubelet"}}},
	}
	obj, err = TransformNode(meta)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj.(*metav1.PartialObjectMetadata).ManagedFields).To(BeNil())
	g.Expect(obj.(*metav1.PartialObjectMetadata).Name).To(Equal("n"))
}

func TestTransformNode_MetadataInformer(t *testing.T) {
	g := NewWithT(t)

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "transform-metadata-node",
		Labels: map[string]string{"test": "transform"},
		ManagedFields: []metav1.ManagedFieldsEntry{{
			Manager:    "test",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			APIVersion: "v1",
			FieldsType: "FieldsV1",
			FieldsV1:   metav1.NewFieldsV1(`{"f:metadata":{"f:labels":{"f:test":{}}}}`),
		}},
	}}
	g.Expect(testClient.Create(ctx, node)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(context.Background(), node) })
	direct := &metav1.PartialObjectMetadata{}
	direct.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), direct)).To(Succeed())
	g.Expect(direct.ManagedFields).NotTo(BeEmpty())

	// The transform is registered for the typed Node only, as in the
	// operator main, and must apply to the metadata Node informer too.
	c, err := ctrlcache.New(testEnv.Config, ctrlcache.Options{
		Scheme: testScheme,
		ByObject: map[client.Object]ctrlcache.ByObject{
			&corev1.Node{}: {Transform: TransformNode},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())
	cacheCtx, cancelCache := context.WithCancel(ctx)
	t.Cleanup(cancelCache)
	go func() { _ = c.Start(cacheCtx) }()
	g.Expect(c.WaitForCacheSync(cacheCtx)).To(BeTrue())

	nodes := &metav1.PartialObjectMetadataList{}
	nodes.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
	g.Eventually(func() []metav1.PartialObjectMetadata {
		g.Expect(c.List(ctx, nodes)).To(Succeed())
		return nodes.Items
	}, 10*time.Second, 100*time.Millisecond).ShouldNot(BeEmpty())
	for _, item := range nodes.Items {
		g.Expect(item.ManagedFields).To(BeNil())
	}
}

// fakeLeaseReader serves the node Leases in pages and records the
// list calls. It fails the page at failPage (1-based) with failErr.
type fakeLeaseReader struct {
	client.Reader

	pages    [][]coordinationv1.Lease
	failPage int
	failErr  error

	calls     int
	opts      []client.ListOptions
	deadlines []time.Time
}

// List returns the page selected by the continue token.
func (r *fakeLeaseReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	r.calls++
	var o client.ListOptions
	o.ApplyOptions(opts)
	r.opts = append(r.opts, o)
	deadline, _ := ctx.Deadline()
	r.deadlines = append(r.deadlines, deadline)

	if r.calls == r.failPage {
		return r.failErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	page := 0
	if o.Continue != "" {
		page, _ = strconv.Atoi(o.Continue)
	}
	leaseList := list.(*coordinationv1.LeaseList)
	leaseList.Items = r.pages[page]
	leaseList.Continue = ""
	if page+1 < len(r.pages) {
		leaseList.Continue = strconv.Itoa(page + 1)
	}
	return nil
}

// testLeases returns n node Leases named with the given prefix.
func testLeases(prefix string, n int) []coordinationv1.Lease {
	leases := make([]coordinationv1.Lease, n)
	for i := range leases {
		leases[i] = coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
			Name: prefix + strconv.Itoa(i), Namespace: NodeLeaseNamespace,
		}}
	}
	return leases
}

func TestListNodeLeases_Pages(t *testing.T) {
	g := NewWithT(t)

	reader := &fakeLeaseReader{pages: [][]coordinationv1.Lease{
		testLeases("a", 2), testLeases("b", 2), testLeases("c", 1),
	}}
	start := time.Now()
	leases, err := listNodeLeases(ctx, reader)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(leases).To(HaveLen(5))
	g.Expect(leases[0].Name).To(Equal("a0"))
	g.Expect(leases[4].Name).To(Equal("c0"))

	// The continue tokens are followed and every page is scoped to the
	// kube-node-lease namespace with the page size.
	g.Expect(reader.calls).To(Equal(3))
	for i, o := range reader.opts {
		g.Expect(o.Namespace).To(Equal(corev1.NamespaceNodeLease))
		g.Expect(o.Limit).To(Equal(int64(nodeLeasesPageSize)))
		g.Expect(o.Continue).To(Equal([]string{"", "1", "2"}[i]))
	}

	// All pages share one deadline.
	g.Expect(reader.deadlines[0]).NotTo(BeZero())
	g.Expect(reader.deadlines[0]).To(BeTemporally("<=", start.Add(nodeLeasesTimeout+time.Second)))
	g.Expect(reader.deadlines).To(HaveEach(Equal(reader.deadlines[0])))
}

func TestListNodeLeases_Errors(t *testing.T) {
	gr := coordinationv1.Resource("leases")
	for _, tt := range []struct {
		name     string
		failPage int
		err      error
	}{
		{name: "forbidden", failPage: 1, err: apierrors.NewForbidden(gr, "", errors.New("denied"))},
		{name: "timeout", failPage: 2, err: context.DeadlineExceeded},
		{name: "expired continue token", failPage: 3, err: apierrors.NewResourceExpired("continue token expired")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			reader := &fakeLeaseReader{
				pages:    [][]coordinationv1.Lease{testLeases("a", 2), testLeases("b", 2), testLeases("c", 2)},
				failPage: tt.failPage,
				failErr:  tt.err,
			}
			leases, err := listNodeLeases(ctx, reader)
			g.Expect(err).To(MatchError(tt.err))
			g.Expect(leases).To(BeNil())
			g.Expect(reader.calls).To(Equal(tt.failPage))
		})
	}

	t.Run("canceled context", func(t *testing.T) {
		g := NewWithT(t)
		reader := &fakeLeaseReader{pages: [][]coordinationv1.Lease{testLeases("a", 1)}}
		canceledCtx, cancelCtx := context.WithCancel(ctx)
		cancelCtx()
		leases, err := listNodeLeases(canceledCtx, reader)
		g.Expect(err).To(MatchError(context.Canceled))
		g.Expect(leases).To(BeNil())
	})
}

func TestBuildNodesSnapshot_LeasesUnavailable(t *testing.T) {
	g := NewWithT(t)

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "leases-unavailable-node"}}
	g.Expect(testClient.Create(ctx, node)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(context.Background(), node) })

	reader := &fakeLeaseReader{
		pages:    [][]coordinationv1.Lease{testLeases("a", 1), testLeases("b", 1)},
		failPage: 2,
		failErr:  apierrors.NewResourceExpired("continue token expired"),
	}
	kc, err := kubeclient.New(reader, testClient, testEnv.Config, testScheme, 100, time.Minute)
	g.Expect(err).NotTo(HaveOccurred())
	handler := &Handler{kubeClient: kc}

	var logs []string
	logCtx := log.IntoContext(ctx, funcr.New(func(_, args string) {
		logs = append(logs, args)
	}, funcr.Options{}))

	// A failed page drops the partial result and the snapshot is still built.
	snap, err := handler.buildNodesSnapshot(logCtx, "", nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(snap).NotTo(BeNil())
	g.Expect(findCheck(snap, "Heartbeat").Status).To(Equal(CheckSkipped))
	g.Expect(logs).To(HaveLen(1))
	g.Expect(logs[0]).To(ContainSubstring("continue token expired"))

	// The error is logged once per failure streak.
	reader.calls = 1
	_, err = handler.buildNodesSnapshot(logCtx, "", nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(logs).To(HaveLen(1))

	// The recovery is logged once and a new failure is logged again.
	reader.calls, reader.failPage = 0, 0
	_, err = handler.buildNodesSnapshot(logCtx, "", nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(logs).To(HaveLen(2))
	g.Expect(logs[1]).To(ContainSubstring("heartbeat check resumed"))

	reader.calls, reader.failPage = 0, 1
	_, err = handler.buildNodesSnapshot(logCtx, "", nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(logs).To(HaveLen(3))
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
	g.Expect(snap).To(HaveKey("serverTime"))
	serverTime, err := time.Parse(time.RFC3339, snap["serverTime"].(string))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(serverTime).To(BeTemporally("~", time.Now(), time.Minute))

	// The server time is not written into the cached snapshot.
	cached, err := json.Marshal(handler.getCachedNodesSnapshot())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(cached)).NotTo(ContainSubstring("serverTime"))
	delete(snap, "serverTime")
	var cachedMap map[string]any
	g.Expect(json.Unmarshal(cached, &cachedMap)).To(Succeed())
	g.Expect(snap).To(Equal(cachedMap))

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

func TestNewNodesSnapshot_RunningAndSchedulingViews(t *testing.T) {
	g := NewWithT(t)
	now := time.Now()

	deployment := func(name string) corev1.Pod {
		pod := testCorePod("apps", name+"-abc12-xyz", "n1", "100Mi", "1Gi")
		pod.Labels = map[string]string{"pod-template-hash": "abc12"}
		pod.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: name + "-abc12", Controller: new(true)}}
		return pod
	}

	// A running pod with a large regular init container: the init peak
	// counts in the node requests, while the usage is measured against
	// the app containers.
	migrated := deployment("migrated")
	migrated.Spec.InitContainers = []corev1.Container{{
		Name: "migrate",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
		},
	}}
	migrated.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodInitialized, Status: corev1.ConditionTrue}}

	// A pod still running its init containers counts in the node requests,
	// but its usage is left out of the usage comparisons.
	initializing := deployment("initializing")
	initializing.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodInitialized, Status: corev1.ConditionFalse}}

	mc := newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"n1": {1, 3}}), now)
	mc.ingest(podMetricsList(map[[2]string]float64{
		{"apps", migrated.Name}:     0.5,
		{"apps", initializing.Name}: 1,
	}), now)

	snap := newNodesSnapshot(nodesInput{
		now:     now,
		nodes:   []corev1.Node{testCoreNode("n1")},
		pods:    []corev1.Pod{migrated, initializing},
		metrics: mc,
		managedWorkloads: managedWorkloadKeys([]reporter.WorkloadRef{
			{Kind: "Deployment", Namespace: "apps", Name: "migrated"},
			{Kind: "Deployment", Namespace: "apps", Name: "initializing"},
		}),
	})

	n := snap.Nodes[0]
	g.Expect(n.Pods).To(Equal(2))
	g.Expect(n.Requests.Memory).To(Equal(int64(2<<30 + 100<<20)))
	g.Expect(n.Limits.Memory).To(Equal(int64(2 << 30)))
	g.Expect(n.AboveRequests.Memory).To(Equal(int64(512<<20 - 100<<20)))

	g.Expect(snap.workloads).To(HaveLen(1))
	w := snap.workloads[0]
	g.Expect(w.Name).To(Equal("migrated"))
	g.Expect(w.Requests).To(Equal(int64(100 << 20)))
	g.Expect(w.NoLimit).To(BeFalse())
	g.Expect(*w.Limits).To(Equal(int64(1 << 30)))
	g.Expect(w.Usage).To(Equal(int64(512 << 20)))
	g.Expect(w.AboveRequests).To(Equal(int64(512<<20 - 100<<20)))

	// A pod-level memory limit bounds a workload whose
	// containers have no memory limit.
	migrated.Spec.Containers[0].Resources.Limits = nil
	migrated.Spec.Resources = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
	}
	snap = newNodesSnapshot(nodesInput{
		now:     now,
		nodes:   []corev1.Node{testCoreNode("n1")},
		pods:    []corev1.Pod{migrated},
		metrics: mc,
		managedWorkloads: managedWorkloadKeys([]reporter.WorkloadRef{
			{Kind: "Deployment", Namespace: "apps", Name: "migrated"},
		}),
	})
	g.Expect(snap.workloads).To(HaveLen(1))
	g.Expect(snap.workloads[0].NoLimit).To(BeFalse())
	g.Expect(*snap.workloads[0].Limits).To(Equal(int64(1 << 30)))
}

func TestNewNodesSnapshot_SampleBeforeInitCompleted(t *testing.T) {
	g := NewWithT(t)
	now := time.Now().Truncate(time.Second)

	deployment := func(name string, initialized time.Time) corev1.Pod {
		pod := testCorePod("apps", name+"-abc12-xyz", "n1", "100Mi", "1Gi")
		pod.Labels = map[string]string{"pod-template-hash": "abc12"}
		pod.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: name + "-abc12", Controller: new(true)}}
		pod.Status.Conditions = []corev1.PodCondition{{
			Type: corev1.PodInitialized, Status: corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(initialized),
		}}
		return pod
	}

	// The usage of both pods is measured over [now-50s, now-30s].
	// The stale pod completed its init containers during that window,
	// the fresh pod before it.
	stale := deployment("stale", now.Add(-40*time.Second))
	fresh := deployment("fresh", now.Add(-time.Minute))

	list := podMetricsList(map[[2]string]float64{
		{"apps", stale.Name}: 2,
		{"apps", fresh.Name}: 0.5,
	})
	for i := range list.Items {
		list.Items[i].Timestamp = metav1.NewTime(now.Add(-30 * time.Second))
		list.Items[i].Window = metav1.Duration{Duration: 20 * time.Second}
	}
	mc := newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"n1": {1, 3}}), now)
	mc.ingest(list, now)
	g.Expect(mc.PodsLatest()["apps/"+stale.Name].MeasuredFrom).To(Equal(now.Add(-50 * time.Second)))

	snap := newNodesSnapshot(nodesInput{
		now:     now,
		nodes:   []corev1.Node{testCoreNode("n1")},
		pods:    []corev1.Pod{stale, fresh},
		metrics: mc,
		managedWorkloads: managedWorkloadKeys([]reporter.WorkloadRef{
			{Kind: "Deployment", Namespace: "apps", Name: "stale"},
			{Kind: "Deployment", Namespace: "apps", Name: "fresh"},
		}),
	})

	// Only the sample measured after init completed counts.
	g.Expect(snap.Nodes[0].AboveRequests.Memory).To(Equal(int64(512<<20 - 100<<20)))
	g.Expect(snap.workloads).To(HaveLen(1))
	g.Expect(snap.workloads[0].Name).To(Equal("fresh"))
	g.Expect(snap.workloads[0].Usage).To(Equal(int64(512 << 20)))
}

func TestPodMeasuredAfterInit(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	initialized := func(status corev1.ConditionStatus, at time.Time) *corev1.Pod {
		return &corev1.Pod{Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
			Type: corev1.PodInitialized, Status: status, LastTransitionTime: metav1.NewTime(at),
		}}}}
	}

	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{name: "no condition", pod: &corev1.Pod{}, want: true},
		{name: "initializing", pod: initialized(corev1.ConditionFalse, now.Add(-time.Hour)), want: false},
		{name: "init completed before the sample", pod: initialized(corev1.ConditionTrue, now.Add(-time.Second)), want: true},
		{name: "init completed at the sample", pod: initialized(corev1.ConditionTrue, now), want: false},
		{name: "init completed after the sample", pod: initialized(corev1.ConditionTrue, now.Add(time.Second)), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(podMeasuredAfterInit(tt.pod, now)).To(Equal(tt.want))
		})
	}
}

// decQuantity returns the quantity in decimal form, the form in which
// in-place arithmetic writes into a value shared by shallow copies.
func decQuantity(s string) resource.Quantity {
	q := resource.MustParse(s)
	q.AsDec()
	return q
}

func TestNodesSnapshot_DoesNotModifyInputs(t *testing.T) {
	g := NewWithT(t)
	now := time.Now()

	sidecarPolicy := corev1.ContainerRestartPolicyAlways
	podLevel := testCorePod("apps", "pod-level", "n1", "100Mi", "1Gi")
	podLevel.Spec.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    decQuantity("500m"),
			corev1.ResourceMemory: decQuantity("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    decQuantity("1"),
			corev1.ResourceMemory: decQuantity("512Mi"),
		},
	}
	podLevel.Spec.Overhead = corev1.ResourceList{
		corev1.ResourceCPU:    decQuantity("10m"),
		corev1.ResourceMemory: decQuantity("16Mi"),
	}
	podLevel.OwnerReferences = []metav1.OwnerReference{{Kind: "StatefulSet", Name: "pod-level", Controller: new(true)}}

	resized := testCorePod("apps", "resized", "n1", "100Mi", "1Gi")
	resized.Spec.InitContainers = []corev1.Container{
		{Name: "init", Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: decQuantity("2Gi")},
		}},
		{Name: "proxy", RestartPolicy: &sidecarPolicy, Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: decQuantity("32Mi")},
		}},
	}
	resized.Spec.Overhead = corev1.ResourceList{corev1.ResourceMemory: decQuantity("16Mi")}
	resized.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "main",
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: decQuantity("256Mi")},
		},
	}}

	nodes := []corev1.Node{testCoreNode("n1", func(n *corev1.Node) {
		n.Spec.Taints = []corev1.Taint{{Key: "k", Effect: corev1.TaintEffectNoSchedule, TimeAdded: &metav1.Time{Time: now}}}
	})}
	pods := []corev1.Pod{podLevel, resized}
	wantNodes := make([]corev1.Node, len(nodes))
	for i := range nodes {
		nodes[i].DeepCopyInto(&wantNodes[i])
	}
	wantPods := make([]corev1.Pod, len(pods))
	for i := range pods {
		pods[i].DeepCopyInto(&wantPods[i])
	}

	mc := newTestCollector()
	mc.ingestNodes(nodeMetricsList(map[string][2]float64{"n1": {1, 3}}), now)
	mc.ingest(podMetricsList(map[[2]string]float64{{"apps", "pod-level"}: 1, {"apps", "resized"}: 1}), now)
	snap := newNodesSnapshot(nodesInput{
		now:     now,
		nodes:   nodes,
		pods:    pods,
		metrics: mc,
		managedWorkloads: managedWorkloadKeys([]reporter.WorkloadRef{
			{Kind: "StatefulSet", Namespace: "apps", Name: "pod-level"},
		}),
	})
	g.Expect(snap.Nodes).To(HaveLen(1))
	g.Expect(snap.workloads).To(HaveLen(1))

	// The controller metrics and workload views sum the same pods.
	for i := range pods {
		_ = sumPodResources(&pods[i])
		_ = podSchedulingResources(&pods[i])
	}

	g.Expect(equality.Semantic.DeepEqual(nodes, wantNodes)).To(BeTrue())
	g.Expect(equality.Semantic.DeepEqual(pods, wantPods)).To(BeTrue())
}
