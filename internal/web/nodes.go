// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// NodeLeaseNamespace is the namespace of the kubelet heartbeat Leases.
const NodeLeaseNamespace = corev1.NamespaceNodeLease

// Node status classification, worst first.
const (
	NodeStatusUnreachable = "Unreachable"
	NodeStatusNotReady    = "NotReady"
	NodeStatusUnknown     = "Unknown"
	NodeStatusPressure    = "Pressure"
	NodeStatusCordoned    = "Cordoned"
	NodeStatusReady       = "Ready"
)

const (
	// nodeRoleLabelPrefix is the prefix of the node role labels.
	nodeRoleLabelPrefix = "node-role.kubernetes.io/"

	// oomKillsWindow is the span over which OOM kills are counted.
	oomKillsWindow = time.Hour

	// readyConditionGracePeriod is how long a new node may run without a
	// Ready condition before it is considered not ready.
	readyConditionGracePeriod = 2 * time.Minute

	// reasonNoReadyCondition is the not-ready reason of a node
	// without a Ready condition past the grace period.
	reasonNoReadyCondition = "NoReadyCondition"
)

var (
	// nodePoolLabels are the node pool labels in priority order.
	nodePoolLabels = []string{
		"karpenter.sh/nodepool",
		"eks.amazonaws.com/nodegroup",
		"cloud.google.com/gke-nodepool",
		"kubernetes.azure.com/agentpool",
		"agentpool",
	}

	// controlPlaneTaints mark control-plane nodes that do not take workloads.
	controlPlaneTaints = []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
	}

	// pressureConditions are the kubelet eviction pressure conditions.
	pressureConditions = []corev1.NodeConditionType{
		corev1.NodeMemoryPressure,
		corev1.NodeDiskPressure,
		corev1.NodePIDPressure,
	}

	// standardConditions are the conditions set by the kubelet and
	// the node controllers; any other condition comes from agents
	// such as node-problem-detector.
	standardConditions = []corev1.NodeConditionType{
		corev1.NodeReady,
		corev1.NodeMemoryPressure,
		corev1.NodeDiskPressure,
		corev1.NodePIDPressure,
		corev1.NodeNetworkUnavailable,
	}
)

// NodesSnapshot is the state of the cluster nodes served by GET /api/v1/nodes,
// built on every report refresh. It carries node names and node-sourced
// strings and must only be served to users who can list nodes.
type NodesSnapshot struct {
	// MetricsAvailable reports whether node metrics are available.
	MetricsAvailable bool `json:"metricsAvailable"`

	// PodMetricsAvailable reports whether pod metrics are available and in
	// step with the node metrics, which is required for aboveRequests.
	PodMetricsAvailable bool `json:"podMetricsAvailable"`

	// ControlPlaneVersion is the Kubernetes API server version.
	ControlPlaneVersion string `json:"controlPlaneVersion"`

	// UnschedulablePods is the number of pending pods the scheduler
	// marked as unschedulable.
	UnschedulablePods int `json:"unschedulablePods"`

	// UnschedulableReasons counts the unschedulable pods per top reason.
	UnschedulableReasons []UnschedulableReason `json:"unschedulableReasons"`

	// UnboundedNamespaces counts the pods without memory bounds per namespace.
	UnboundedNamespaces UnboundedNamespaces `json:"unboundedNamespaces"`

	// Capacity holds the cluster totals over the workload capacity nodes.
	Capacity NodesCapacity `json:"capacity"`

	// Usage holds the 30m cluster usage series.
	Usage NodesUsage `json:"usage"`

	// Nodes holds the per-node details.
	Nodes []*NodeDetail `json:"nodes"`

	// Findings holds the grouped finding lines, worst first.
	Findings []NodeFindingLine `json:"findings"`

	// Checks holds the health check results in catalogue order.
	Checks []NodeCheck `json:"checks"`

	// summary is the name-less summary injected into the report.
	summary *NodesSummary

	// workloads holds the cluster-wide Flux-managed workloads using memory
	// above their requests, ranked by the amount, before namespace filtering.
	workloads []BurstingWorkload
}

// UnschedulableReason is a scheduler reason with the number of pods for
// which it is the top reason.
type UnschedulableReason struct {
	Reason string `json:"reason"`
	Pods   int    `json:"pods"`
}

// NamespacePods is a namespace with a pod count.
type NamespacePods struct {
	Namespace string `json:"namespace"`
	Pods      int    `json:"pods"`
}

// UnboundedNamespaces holds the namespaces of the pods without a memory
// request (hence without a limit) and of the pods with a memory request
// but without a limit, most pods first.
type UnboundedNamespaces struct {
	WithoutRequest []NamespacePods `json:"withoutRequest"`
	WithoutLimit   []NamespacePods `json:"withoutLimit"`
}

// CapacityResource holds the cluster totals of a resource, CPU in cores and
// memory in bytes. Used and AboveRequests are nil when metrics are
// unavailable; every field is nil when no node contributes capacity.
type CapacityResource struct {
	Allocatable   *float64 `json:"allocatable"`
	Requested     *float64 `json:"requested"`
	Used          *float64 `json:"used"`
	AboveRequests *float64 `json:"aboveRequests"`
	Limits        *float64 `json:"limits"`
}

// CapacityPods holds the cluster pod slots and the non-terminal pods
// assigned to nodes; nil when no node contributes capacity.
type CapacityPods struct {
	Allocatable *int64 `json:"allocatable"`
	Running     *int64 `json:"running"`
}

// NodesCapacity holds the cluster capacity totals.
type NodesCapacity struct {
	CPU    CapacityResource `json:"cpu"`
	Memory CapacityResource `json:"memory"`
	Pods   CapacityPods     `json:"pods"`
}

// NodesUsage holds the cluster usage series.
type NodesUsage struct {
	Samples []MetricsSample `json:"samples"`
}

// NodeResources holds CPU in cores and memory in bytes.
type NodeResources struct {
	CPU    float64 `json:"cpu"`
	Memory int64   `json:"memory"`
}

// NodeAllocatable holds the node allocatable resources.
type NodeAllocatable struct {
	CPU    float64 `json:"cpu"`
	Memory int64   `json:"memory"`
	Pods   int64   `json:"pods"`
}

// NodeTaint is a node taint.
type NodeTaint struct {
	Key       string       `json:"key"`
	Value     string       `json:"value,omitempty"`
	Effect    string       `json:"effect"`
	TimeAdded *metav1.Time `json:"timeAdded,omitempty"`
}

// NodeConditionInfo is a node condition.
type NodeConditionInfo struct {
	Type               string      `json:"type"`
	Status             string      `json:"status"`
	Reason             string      `json:"reason,omitempty"`
	Message            string      `json:"message,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// NodeSystemInfo holds the node system information.
type NodeSystemInfo struct {
	KubeletVersion          string `json:"kubeletVersion"`
	OSImage                 string `json:"osImage"`
	KernelVersion           string `json:"kernelVersion"`
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
	Architecture            string `json:"architecture"`
}

// NodeDetail holds the state of a node with its aggregated pod resources,
// usage and findings.
type NodeDetail struct {
	Name          string      `json:"name"`
	Pool          string      `json:"pool"`
	Zone          string      `json:"zone"`
	InstanceType  string      `json:"instanceType"`
	Roles         []string    `json:"roles"`
	CreatedAt     metav1.Time `json:"createdAt"`
	Unschedulable bool        `json:"unschedulable"`
	ControlPlane  bool        `json:"controlPlane"`

	// Status is one of Unreachable, NotReady, Unknown, Pressure, Cordoned or Ready.
	Status     string              `json:"status"`
	Pressures  []string            `json:"pressures"`
	Taints     []NodeTaint         `json:"taints"`
	Conditions []NodeConditionInfo `json:"conditions"`
	Info       NodeSystemInfo      `json:"info"`

	Allocatable NodeAllocatable `json:"allocatable"`
	Requests    NodeResources   `json:"requests"`
	Limits      NodeResources   `json:"limits"`

	// Pods counts the non-terminal pods assigned to the node.
	Pods                     int `json:"pods"`
	PodsWithoutMemoryRequest int `json:"podsWithoutMemoryRequest"`
	PodsWithoutMemoryLimit   int `json:"podsWithoutMemoryLimit"`

	// OOMKills is a lower bound of the containers OOM-killed in the last hour.
	OOMKills int `json:"oomKills"`

	// Usage is the latest sample, nil when the node has no metrics.
	Usage *NodeResources `json:"usage"`

	// AboveRequests is the sum over pods of max(0, usage - request) at the
	// latest pod sample, nil without pod metrics.
	AboveRequests *NodeResources `json:"aboveRequests"`

	// LeaseRenewTime and HeartbeatSeconds are nil when the node has no Lease.
	LeaseRenewTime   *metav1.Time `json:"leaseRenewTime"`
	HeartbeatSeconds *int         `json:"heartbeatSeconds"`

	// ReadyTransitions is nil until 15m of history has been observed.
	ReadyTransitions *int `json:"readyTransitions"`

	// Findings holds the node findings, worst first.
	Findings []NodeFinding `json:"findings"`

	// readyTrue reports whether the Ready condition is True.
	readyTrue bool

	// placeholderRequests are the requests of the autoscaler
	// placeholder pods, which the scheduler preempts.
	placeholderRequests NodeResources
}

// BurstingWorkload is a workload using memory above its requests.
type BurstingWorkload struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Pods      int    `json:"pods"`
	Nodes     int    `json:"nodes"`

	// Usage, Requests, Limits and AboveRequests are memory bytes.
	// Limits is nil when a pod has no memory limit.
	Usage         int64  `json:"usage"`
	Requests      int64  `json:"requests"`
	Limits        *int64 `json:"limits"`
	AboveRequests int64  `json:"aboveRequests"`
	NoLimit       bool   `json:"noLimit"`

	nodes  map[string]struct{}
	limits int64
}

// NodesWorkloads is the response of GET /api/v1/nodes/workloads.
type NodesWorkloads struct {
	// MetricsAvailable reports whether pod metrics are available.
	MetricsAvailable bool               `json:"metricsAvailable"`
	Workloads        []BurstingWorkload `json:"workloads"`
}

// burstingWorkloadsLimit caps the number of workloads served by
// GET /api/v1/nodes/workloads.
const burstingWorkloadsLimit = 20

// NodesHandler handles GET /api/v1/nodes requests and returns the cached
// nodes snapshot. Users who can't list nodes get a 403.
func (h *Handler) NodesHandler(w http.ResponseWriter, req *http.Request) {
	snap := h.authorizedNodesSnapshot(w, req)
	if snap == nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snap); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// NodesWorkloadsHandler handles GET /api/v1/nodes/workloads requests and
// returns the top Flux-managed workloads by memory usage above requests,
// filtered to the namespaces the user can access. Users who can't list nodes get a 403.
func (h *Handler) NodesWorkloadsHandler(w http.ResponseWriter, req *http.Request) {
	snap := h.authorizedNodesSnapshot(w, req)
	if snap == nil {
		return
	}

	namespaces, _, err := h.kubeClient.ListUserNamespaces(req.Context())
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to list user namespaces")
		http.Error(w, fmt.Sprintf("Failed to list user namespaces: %v", err), http.StatusInternalServerError)
		return
	}

	resp := NodesWorkloads{
		MetricsAvailable: snap.PodMetricsAvailable,
		Workloads:        filterWorkloads(snap.workloads, namespaces, burstingWorkloadsLimit),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// authorizedNodesSnapshot checks that the user can list nodes and returns
// the cached nodes snapshot. It writes the error response and returns nil
// when the user is not allowed, the check fails or the snapshot is not
// available yet.
func (h *Handler) authorizedNodesSnapshot(w http.ResponseWriter, req *http.Request) *NodesSnapshot {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil
	}

	allowed, err := h.kubeClient.CanListNodes(req.Context())
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to check the user access to nodes")
		http.Error(w, fmt.Sprintf("Failed to check access to nodes: %v", err), http.StatusInternalServerError)
		return nil
	}
	if !allowed {
		perms := user.Permissions(req.Context())
		http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to list nodes",
			perms.Username), http.StatusForbidden)
		return nil
	}

	snap := h.getCachedNodesSnapshot()
	if snap == nil {
		http.Error(w, "Nodes snapshot not available", http.StatusServiceUnavailable)
		return nil
	}
	return snap
}

// nodesInput holds the inputs of the nodes snapshot builder.
type nodesInput struct {
	now           time.Time
	nodes         []corev1.Node
	pods          []corev1.Pod
	leases        []coordinationv1.Lease
	serverVersion string
	metrics       *MetricsCollector
	transitions   *ReadyTransitionTracker

	// jobOwner returns the name of the CronJob owning a Job, if any.
	jobOwner func(namespace, name string) string

	// managedWorkloads holds the keys of the Flux-managed workloads,
	// the only ones ranked as bursting workloads.
	managedWorkloads map[string]struct{}
}

// workloadKey returns the key of a workload in nodesInput.managedWorkloads.
func workloadKey(kind, namespace, name string) string {
	return kind + "/" + namespace + "/" + name
}

// managedWorkloadKeys returns the keys of the given Flux-managed workloads.
func managedWorkloadKeys(workloads []reporter.WorkloadRef) map[string]struct{} {
	keys := make(map[string]struct{}, len(workloads))
	for _, w := range workloads {
		keys[workloadKey(w.Kind, w.Namespace, w.Name)] = struct{}{}
	}
	return keys
}

// TransformNode drops the container images list and the managed fields
// of Node objects before they are committed to the cache.
func TransformNode(obj any) (any, error) {
	if node, ok := obj.(*corev1.Node); ok {
		node.Status.Images = nil
		node.ManagedFields = nil
	}
	return obj, nil
}

// buildNodesSnapshot lists the nodes, pods and node Leases from the
// privileged cached client and builds the nodes snapshot.
func (h *Handler) buildNodesSnapshot(ctx context.Context, serverVersion string, workloads []reporter.WorkloadRef) (*NodesSnapshot, error) {
	kubeClient := h.kubeClient.GetClient(ctx, kubeclient.WithPrivileges())

	var nodeList corev1.NodeList
	if err := kubeClient.List(ctx, &nodeList); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Pod phases are filtered in memory, field selectors
	// are not supported by the cached client.
	var podList corev1.PodList
	if err := kubeClient.List(ctx, &podList); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// The heartbeat check is skipped when the Leases can't be read.
	var leaseList coordinationv1.LeaseList
	_ = kubeClient.List(ctx, &leaseList, client.InNamespace(NodeLeaseNamespace))

	jobOwner := func(namespace, name string) string {
		var job batchv1.Job
		if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &job); err != nil {
			return ""
		}
		if ref := metav1.GetControllerOf(&job); ref != nil && ref.Kind == "CronJob" {
			return ref.Name
		}
		return ""
	}

	return newNodesSnapshot(nodesInput{
		now:           time.Now(),
		nodes:         nodeList.Items,
		pods:          podList.Items,
		leases:        leaseList.Items,
		serverVersion: serverVersion,
		metrics:       h.metrics,
		transitions:   h.nodeTransitions,
		jobOwner:      jobOwner,

		managedWorkloads: managedWorkloadKeys(workloads),
	}), nil
}

// newNodesSnapshot builds the nodes snapshot from the given inputs:
// per-node details with the aggregated pod resources, the cluster capacity
// and usage, the findings, the check results and the name-less summary.
func newNodesSnapshot(in nodesInput) *NodesSnapshot {
	snap := &NodesSnapshot{
		ControlPlaneVersion:  in.serverVersion,
		UnschedulableReasons: []UnschedulableReason{},
		UnboundedNamespaces: UnboundedNamespaces{
			WithoutRequest: []NamespacePods{},
			WithoutLimit:   []NamespacePods{},
		},
		Usage:     NodesUsage{Samples: []MetricsSample{}},
		Nodes:     make([]*NodeDetail, 0, len(in.nodes)),
		workloads: []BurstingWorkload{},
	}

	// Leases by node name.
	leases := make(map[string]*coordinationv1.Lease, len(in.leases))
	for i := range in.leases {
		leases[in.leases[i].Name] = &in.leases[i]
	}

	// Node details and the workload capacity nodes.
	byName := make(map[string]*NodeDetail, len(in.nodes))
	var eligible []string
	for i := range in.nodes {
		n := newNodeDetail(&in.nodes[i], in.now)
		if lease, ok := leases[n.Name]; ok && lease.Spec.RenewTime != nil {
			renew := metav1.NewTime(lease.Spec.RenewTime.Time)
			n.LeaseRenewTime = &renew
			age := max(0, int(in.now.Sub(renew.Time).Seconds()))
			n.HeartbeatSeconds = &age
		}
		n.ReadyTransitions = in.transitions.Transitions(n.Name)
		snap.Nodes = append(snap.Nodes, n)
		byName[n.Name] = n
		if isWorkloadCapacity(n) {
			eligible = append(eligible, n.Name)
		}
	}

	// Metrics: node and pod metrics have their own availability state,
	// read in one consistent view.
	var view NodesMetricsView
	if in.metrics != nil {
		view = in.metrics.NodesView(eligible)
	}
	podMetricsOK := view.PodsAvailable
	if podMetricsOK && view.NodesAvailable {
		// The per-pod sums must be taken at about the same time as the
		// node usage they are compared with.
		skew := view.NodeTick.Sub(view.PodTick)
		if skew < 0 {
			skew = -skew
		}
		if skew > 2*view.Interval {
			podMetricsOK = false
		}
	}
	snap.MetricsAvailable = view.NodesAvailable
	snap.PodMetricsAvailable = podMetricsOK
	var podSamples map[string]MetricsSample
	if podMetricsOK {
		podSamples = view.PodLatest
	}
	for _, n := range snap.Nodes {
		if s, ok := view.NodeLatest[n.Name]; ok {
			n.Usage = &NodeResources{CPU: s.CPU, Memory: s.Memory}
		}
		if podMetricsOK {
			n.AboveRequests = &NodeResources{}
		}
	}

	aggregatePods(snap, in, byName, podSamples)

	// Cluster capacity and usage over the workload capacity nodes.
	if view.NodesAvailable && len(eligible) > 0 {
		snap.Usage.Samples = view.Series
	}
	snap.Capacity = clusterCapacity(snap.Nodes, snap.Usage.Samples, view.NodeTick)

	evaluateNodes(snap)
	snap.summary = newNodesSummary(snap)
	return snap
}

// aggregatePods sums the pod resources, slots, memory bounds and OOM kills
// per node, the usage above requests per node and per workload at the
// latest pod sample, and counts the unschedulable pods with their reasons.
// Pod phases are filtered in memory.
func aggregatePods(snap *NodesSnapshot, in nodesInput, byName map[string]*NodeDetail, podSamples map[string]MetricsSample) {
	withoutRequest := make(map[string]int)
	withoutLimit := make(map[string]int)
	reasons := make(map[string]int)
	workloads := make(map[string]*BurstingWorkload)
	for i := range in.pods {
		pod := &in.pods[i]
		terminal := pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed

		if pod.Spec.NodeName == "" {
			if !terminal {
				if reason, ok := unschedulableReason(pod); ok {
					snap.UnschedulablePods++
					reasons[reason]++
				}
			}
			continue
		}

		n, ok := byName[pod.Spec.NodeName]
		if !ok {
			continue
		}
		n.OOMKills += podOOMKills(pod, in.now)
		if terminal {
			continue
		}

		// Pod slots count every non-terminal pod assigned to the node,
		// as the scheduler does against allocatable pods.
		n.Pods++
		res := sumPodResources(pod)
		n.Requests.CPU += res.CPURequests
		n.Requests.Memory += res.MemoryRequests
		n.Limits.CPU += res.CPULimits
		n.Limits.Memory += res.MemoryLimits
		if isPlaceholderPod(pod) {
			n.placeholderRequests.CPU += res.CPURequests
			n.placeholderRequests.Memory += res.MemoryRequests
		}

		noRequest, noLimit := podMemoryBounds(pod)
		if noRequest {
			n.PodsWithoutMemoryRequest++
			withoutRequest[pod.Namespace]++
		}
		if noLimit {
			n.PodsWithoutMemoryLimit++
			withoutLimit[pod.Namespace]++
		}

		// Usage above requests, summed per pod at the latest pod sample.
		sample, measured := podSamples[podKey(pod.Namespace, pod.Name)]
		var aboveMemory int64
		if measured {
			aboveCPU := max(0, sample.CPU-res.CPURequests)
			aboveMemory = max(0, sample.Memory-res.MemoryRequests)
			n.AboveRequests.CPU += aboveCPU
			n.AboveRequests.Memory += aboveMemory
		}

		// Only the Flux-managed workloads are ranked. Their pods, nodes,
		// requests and limits cover all their pods, while the usage and
		// the amount above requests are sums over the measured pods.
		if podSamples == nil || len(in.managedWorkloads) == 0 {
			continue
		}
		kind, name := podWorkload(pod, in.jobOwner)
		key := workloadKey(kind, pod.Namespace, name)
		if _, managed := in.managedWorkloads[key]; !managed {
			continue
		}
		w, ok := workloads[key]
		if !ok {
			w = &BurstingWorkload{Kind: kind, Namespace: pod.Namespace, Name: name, nodes: make(map[string]struct{})}
			workloads[key] = w
		}
		w.Pods++
		w.nodes[pod.Spec.NodeName] = struct{}{}
		w.Requests += res.MemoryRequests
		if noRequest || noLimit {
			w.NoLimit = true
		} else {
			w.limits += res.MemoryLimits
		}
		if measured {
			w.Usage += sample.Memory
			w.AboveRequests += aboveMemory
		}
	}
	for _, w := range workloads {
		if w.AboveRequests <= 0 {
			continue
		}
		w.Nodes = len(w.nodes)
		if !w.NoLimit {
			w.Limits = new(w.limits)
		}
		snap.workloads = append(snap.workloads, *w)
	}
	slices.SortFunc(snap.workloads, func(a, b BurstingWorkload) int {
		return cmp.Or(cmp.Compare(b.AboveRequests, a.AboveRequests),
			cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Name, b.Name))
	})

	for reason, pods := range reasons {
		snap.UnschedulableReasons = append(snap.UnschedulableReasons, UnschedulableReason{Reason: reason, Pods: pods})
	}
	slices.SortFunc(snap.UnschedulableReasons, func(a, b UnschedulableReason) int {
		return cmp.Or(cmp.Compare(b.Pods, a.Pods), cmp.Compare(a.Reason, b.Reason))
	})
	snap.UnboundedNamespaces.WithoutRequest = sortedNamespacePods(withoutRequest)
	snap.UnboundedNamespaces.WithoutLimit = sortedNamespacePods(withoutLimit)
}

// newNodeDetail returns the details of a node without
// the pod aggregates, usage and findings.
func newNodeDetail(node *corev1.Node, now time.Time) *NodeDetail {
	n := &NodeDetail{
		Name:          node.Name,
		Zone:          node.Labels[corev1.LabelTopologyZone],
		InstanceType:  node.Labels[corev1.LabelInstanceTypeStable],
		Roles:         nodeRoles(node),
		CreatedAt:     node.CreationTimestamp,
		Unschedulable: node.Spec.Unschedulable,
		ControlPlane:  isControlPlaneNode(node),
		Status:        nodeStatus(node, now),
		Pressures:     nodePressures(node),
		Taints:        make([]NodeTaint, 0, len(node.Spec.Taints)),
		Conditions:    make([]NodeConditionInfo, 0, len(node.Status.Conditions)),
		Info: NodeSystemInfo{
			KubeletVersion:          node.Status.NodeInfo.KubeletVersion,
			OSImage:                 node.Status.NodeInfo.OSImage,
			KernelVersion:           node.Status.NodeInfo.KernelVersion,
			ContainerRuntimeVersion: node.Status.NodeInfo.ContainerRuntimeVersion,
			Architecture:            node.Status.NodeInfo.Architecture,
		},
		Findings: []NodeFinding{},
	}
	n.Pool = nodePool(node, n.Roles)
	if ready := nodeCondition(node, corev1.NodeReady); ready != nil {
		n.readyTrue = ready.Status == corev1.ConditionTrue
	}

	if q, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
		n.Allocatable.CPU = q.AsApproximateFloat64()
	}
	if q, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
		n.Allocatable.Memory = q.Value()
	}
	if q, ok := node.Status.Allocatable[corev1.ResourcePods]; ok {
		n.Allocatable.Pods = q.Value()
	}

	for _, t := range node.Spec.Taints {
		n.Taints = append(n.Taints, NodeTaint{Key: t.Key, Value: t.Value, Effect: string(t.Effect), TimeAdded: t.TimeAdded})
	}
	for _, c := range node.Status.Conditions {
		n.Conditions = append(n.Conditions, NodeConditionInfo{
			Type:               string(c.Type),
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}
	return n
}

// nodeCondition returns the condition of the given type or nil.
func nodeCondition(node *corev1.Node, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == conditionType {
			return &node.Status.Conditions[i]
		}
	}
	return nil
}

// nodeStatus classifies a node into a single status, worst first:
// Unreachable (Ready=Unknown), NotReady (Ready=False, or no Ready condition
// for longer than the grace period), Unknown (no Ready condition yet, e.g.
// a node registered before its first kubelet status update), Pressure,
// Cordoned and Ready.
func nodeStatus(node *corev1.Node, now time.Time) string {
	ready := nodeCondition(node, corev1.NodeReady)
	switch {
	case ready == nil && missingReadyCondition(node, now):
		return NodeStatusNotReady
	case ready == nil:
		return NodeStatusUnknown
	case ready.Status == corev1.ConditionUnknown:
		return NodeStatusUnreachable
	case ready.Status != corev1.ConditionTrue:
		return NodeStatusNotReady
	case len(nodePressures(node)) > 0:
		return NodeStatusPressure
	case node.Spec.Unschedulable:
		return NodeStatusCordoned
	default:
		return NodeStatusReady
	}
}

// missingReadyCondition reports whether a node without a Ready condition
// was created longer than the grace period ago.
func missingReadyCondition(node *corev1.Node, now time.Time) bool {
	return now.Sub(node.CreationTimestamp.Time) > readyConditionGracePeriod
}

// nodePressures returns the pressure condition types reported True.
func nodePressures(node *corev1.Node) []string {
	pressures := []string{}
	for _, t := range pressureConditions {
		if c := nodeCondition(node, t); c != nil && c.Status == corev1.ConditionTrue {
			pressures = append(pressures, string(t))
		}
	}
	return pressures
}

// nodeRoles returns the sorted node-role.kubernetes.io/<role> suffixes.
func nodeRoles(node *corev1.Node) []string {
	roles := []string{}
	for k := range node.Labels {
		if role, ok := strings.CutPrefix(k, nodeRoleLabelPrefix); ok && role != "" {
			roles = append(roles, role)
		}
	}
	slices.Sort(roles)
	return roles
}

// nodePool returns the node pool from the provider labels in priority
// order, falling back to the first node role, else empty.
func nodePool(node *corev1.Node, roles []string) string {
	for _, l := range nodePoolLabels {
		if v := node.Labels[l]; v != "" {
			return v
		}
	}
	if len(roles) > 0 {
		return roles[0]
	}
	return ""
}

// isControlPlaneNode reports whether the node is a control-plane node that
// does not take workloads (control-plane or master taint, except
// PreferNoSchedule).
func isControlPlaneNode(node *corev1.Node) bool {
	for _, t := range node.Spec.Taints {
		if slices.Contains(controlPlaneTaints, t.Key) && t.Effect != corev1.TaintEffectPreferNoSchedule {
			return true
		}
	}
	return false
}

// isAvailable reports whether a node contributes capacity: unreachable,
// not ready and unknown nodes do not, so their allocatable is never
// shown as free.
func isAvailable(n *NodeDetail) bool {
	switch n.Status {
	case NodeStatusUnreachable, NodeStatusNotReady, NodeStatusUnknown:
		return false
	default:
		return true
	}
}

// isWorkloadCapacity reports whether a node's capacity is available
// to workloads: available and not a control-plane node.
func isWorkloadCapacity(n *NodeDetail) bool {
	return isAvailable(n) && !n.ControlPlane
}

// clusterCapacity sums the capacity of the workload capacity nodes.
// Used is the latest sample of the cluster usage series, when it is
// from the latest node scrape.
func clusterCapacity(nodes []*NodeDetail, samples []MetricsSample, nodeTick time.Time) NodesCapacity {
	var capacity NodesCapacity
	var cpu, memory struct{ allocatable, requested, limits, above float64 }
	var podsAllocatable, podsRunning int64
	eligible := 0
	aboveOK := true
	for _, n := range nodes {
		if !isWorkloadCapacity(n) {
			continue
		}
		eligible++
		cpu.allocatable += n.Allocatable.CPU
		cpu.requested += n.Requests.CPU
		cpu.limits += n.Limits.CPU
		memory.allocatable += float64(n.Allocatable.Memory)
		memory.requested += float64(n.Requests.Memory)
		memory.limits += float64(n.Limits.Memory)
		podsAllocatable += n.Allocatable.Pods
		podsRunning += int64(n.Pods)
		if n.AboveRequests == nil {
			aboveOK = false
		} else {
			cpu.above += n.AboveRequests.CPU
			memory.above += float64(n.AboveRequests.Memory)
		}
	}
	if eligible == 0 {
		return capacity
	}

	capacity.CPU = CapacityResource{
		Allocatable: new(cpu.allocatable),
		Requested:   new(cpu.requested),
		Limits:      new(cpu.limits),
	}
	capacity.Memory = CapacityResource{
		Allocatable: new(memory.allocatable),
		Requested:   new(memory.requested),
		Limits:      new(memory.limits),
	}
	capacity.Pods = CapacityPods{
		Allocatable: new(podsAllocatable),
		Running:     new(podsRunning),
	}

	if len(samples) > 0 {
		last := samples[len(samples)-1]
		if last.Time.Equal(nodeTick) {
			capacity.CPU.Used = new(last.CPU)
			capacity.Memory.Used = new(float64(last.Memory))
			if aboveOK {
				capacity.CPU.AboveRequests = new(cpu.above)
				capacity.Memory.AboveRequests = new(memory.above)
			}
		}
	}
	return capacity
}

// sortedNamespacePods returns the namespace counts, most pods first.
func sortedNamespacePods(counts map[string]int) []NamespacePods {
	result := make([]NamespacePods, 0, len(counts))
	for ns, pods := range counts {
		result = append(result, NamespacePods{Namespace: ns, Pods: pods})
	}
	slices.SortFunc(result, func(a, b NamespacePods) int {
		return cmp.Or(cmp.Compare(b.Pods, a.Pods), cmp.Compare(a.Namespace, b.Namespace))
	})
	return result
}

// podContainers returns the app containers and the restartable
// init (sidecar) containers of a pod.
func podContainers(pod *corev1.Pod) []*corev1.Container {
	containers := make([]*corev1.Container, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	for i := range pod.Spec.InitContainers {
		c := &pod.Spec.InitContainers[i]
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			containers = append(containers, c)
		}
	}
	for i := range pod.Spec.Containers {
		containers = append(containers, &pod.Spec.Containers[i])
	}
	return containers
}

// podMemoryBounds classifies the memory bounds of a pod over its app and
// sidecar containers, as two disjoint cases. A pod is without request when
// no non-zero pod-level memory request is set and some container has no memory
// request or a zero one: the API server copies a limit into a missing
// request, so the scheduler reserves nothing for that container. A pod is
// without limit when it has requests, no non-zero pod-level memory limit
// and some container has no memory limit.
func podMemoryBounds(pod *corev1.Pod) (withoutRequest, withoutLimit bool) {
	// An explicit zero pod-level quantity bounds nothing.
	var podRequest, podLimit bool
	if r := pod.Spec.Resources; r != nil {
		if q, ok := r.Requests[corev1.ResourceMemory]; ok && !q.IsZero() {
			podRequest = true
		}
		if q, ok := r.Limits[corev1.ResourceMemory]; ok && !q.IsZero() {
			podLimit = true
		}
	}

	var missingRequest, missingLimit bool
	for _, c := range podContainers(pod) {
		if q, ok := c.Resources.Requests[corev1.ResourceMemory]; !ok || q.IsZero() {
			missingRequest = true
		}
		if _, ok := c.Resources.Limits[corev1.ResourceMemory]; !ok {
			missingLimit = true
		}
	}

	withoutRequest = !podRequest && missingRequest
	withoutLimit = !withoutRequest && !podLimit && missingLimit
	return withoutRequest, withoutLimit
}

// isPlaceholderPod reports whether a pod is an autoscaler placeholder:
// negative priority and only pause containers. The scheduler preempts
// such pods, so their requests are not a capacity risk.
func isPlaceholderPod(pod *corev1.Pod) bool {
	if pod.Spec.Priority == nil || *pod.Spec.Priority >= 0 {
		return false
	}
	containers := append(slices.Clone(pod.Spec.InitContainers), pod.Spec.Containers...)
	if len(containers) == 0 {
		return false
	}
	for _, c := range containers {
		if imageBasename(c.Image) != "pause" {
			return false
		}
	}
	return true
}

// imageBasename returns the repository basename of an image reference,
// without registry, path, tag and digest.
func imageBasename(image string) string {
	image, _, _ = strings.Cut(image, "@")
	base := path.Base(image)
	base, _, _ = strings.Cut(base, ":")
	return base
}

// podOOMKills counts the containers of a pod whose last known termination
// (the current state when terminated, else the last state) was an OOM kill
// that finished within the window. Each container counts at most once and
// only its last termination is known, so this is a lower bound.
func podOOMKills(pod *corev1.Pod, now time.Time) int {
	count := 0
	for _, statuses := range [][]corev1.ContainerStatus{
		pod.Status.InitContainerStatuses,
		pod.Status.ContainerStatuses,
		pod.Status.EphemeralContainerStatuses,
	} {
		for _, st := range statuses {
			t := st.State.Terminated
			if t == nil {
				t = st.LastTerminationState.Terminated
			}
			if t != nil && t.Reason == "OOMKilled" && now.Sub(t.FinishedAt.Time) <= oomKillsWindow {
				count++
			}
		}
	}
	return count
}

// unschedulableReason reports whether the pod is pending with
// PodScheduled=False and reason Unschedulable, and returns its top
// scheduler reason parsed from the condition message.
func unschedulableReason(pod *corev1.Pod) (string, bool) {
	if pod.Status.Phase != corev1.PodPending {
		return "", false
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse &&
			c.Reason == corev1.PodReasonUnschedulable {
			if reason := topSchedulerReason(c.Message); reason != "" {
				return reason, true
			}
			return c.Reason, true
		}
	}
	return "", false
}

// topSchedulerReason returns the reason with the highest node count from
// a scheduler message such as "0/20 nodes are available: 3 Insufficient
// memory, 1 node(s) had untolerated taint {key: value}. preemption: ...".
// Taint and label details in braces and the "node(s) had" prefix are
// dropped, so pods blocked for the same reason are counted together.
func topSchedulerReason(msg string) string {
	_, rest, found := strings.Cut(msg, "are available: ")
	if !found {
		return ""
	}
	rest, _, _ = strings.Cut(rest, ". preemption:")
	rest = strings.TrimSuffix(strings.TrimSpace(rest), ".")

	best, bestCount := "", -1
	for _, segment := range splitOutsideBraces(rest, ',') {
		segment = strings.TrimSpace(segment)
		count := 0
		if num, text, ok := strings.Cut(segment, " "); ok {
			if n, err := strconv.Atoi(num); err == nil {
				count = n
				segment = text
			}
		}
		reason := normalizeSchedulerReason(segment)
		if reason != "" && count > bestCount {
			best, bestCount = reason, count
		}
	}
	return best
}

// splitOutsideBraces splits s on sep, ignoring separators inside braces.
func splitOutsideBraces(s string, sep rune) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth = max(0, depth-1)
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// normalizeSchedulerReason drops the details in braces, the "(s)" plural
// markers and the "node(s) had" prefix from a scheduler reason.
func normalizeSchedulerReason(reason string) string {
	var b strings.Builder
	depth := 0
	for _, r := range reason {
		switch {
		case r == '{':
			depth++
		case r == '}':
			depth = max(0, depth-1)
		case depth == 0:
			b.WriteRune(r)
		}
	}
	reason = b.String()
	reason = strings.TrimPrefix(strings.TrimSpace(reason), "node(s) ")
	reason = strings.TrimPrefix(reason, "had ")
	reason = strings.ReplaceAll(reason, "(s)", "")
	return strings.Join(strings.Fields(reason), " ")
}

// podWorkload maps a pod to its workload through the owner references:
// ReplicaSet to Deployment (by the pod-template-hash suffix), StatefulSet,
// DaemonSet, Job to CronJob. Pods without an owner or with an owner that
// doesn't map (e.g. static pods owned by their Node, standalone ReplicaSets
// and Jobs) are returned as bare pods, which are not Flux-managed workloads.
func podWorkload(pod *corev1.Pod, jobOwner func(namespace, name string) string) (kind, name string) {
	ref := metav1.GetControllerOf(pod)
	if ref == nil {
		return "Pod", pod.Name
	}
	switch ref.Kind {
	case "ReplicaSet":
		if hash := pod.Labels["pod-template-hash"]; hash != "" {
			if deployment, ok := strings.CutSuffix(ref.Name, "-"+hash); ok && deployment != "" {
				return "Deployment", deployment
			}
		}
	case "StatefulSet", "DaemonSet":
		return ref.Kind, ref.Name
	case "Job":
		if jobOwner != nil {
			if cronJob := jobOwner(pod.Namespace, ref.Name); cronJob != "" {
				return "CronJob", cronJob
			}
		}
	}
	return "Pod", pod.Name
}

// percent returns the integer percentage of value over total, floored with
// a small epsilon so that float ratios such as 0.945/1.05 read 90, and
// false when the total is unset. Every threshold compares this integer.
func percent(value, total float64) (int, bool) {
	if total <= 0 {
		return 0, false
	}
	return int(math.Floor(value/total*100 + 1e-9)), true
}

// filterWorkloads returns the top workloads in the given namespaces.
func filterWorkloads(workloads []BurstingWorkload, namespaces []string, limit int) []BurstingWorkload {
	result := make([]BurstingWorkload, 0, min(limit, len(workloads)))
	for _, w := range workloads {
		if len(result) == limit {
			break
		}
		if _, found := slices.BinarySearch(namespaces, w.Namespace); found {
			result = append(result, w)
		}
	}
	return result
}
