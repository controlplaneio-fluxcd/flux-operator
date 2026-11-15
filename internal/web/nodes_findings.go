// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Finding severities.
const (
	SeverityCritical = "critical"
	SeverityWarn     = "warn"
	SeverityInfo     = "info"
)

// Health check results.
const (
	CheckRaised  = "raised"
	CheckSkipped = "skipped"
	CheckPassed  = "passed"
)

// Finding codes.
const (
	CodeUnreachable        = "unreachable"
	CodeNotReady           = "not-ready"
	CodeMemoryPressure     = "memory-pressure"
	CodeDiskPressure       = "disk-pressure"
	CodePIDPressure        = "pid-pressure"
	CodeNetworkUnavailable = "network-unavailable"
	CodeMemoryOvercommit   = "memory-overcommit"
	CodeNodeCondition      = "node-condition"
	CodeMemoryHigh         = "memory-high"
	CodeCPUHigh            = "cpu-high"
	CodeOOMKills           = "oom-kills"
	CodeNoMemoryRequest    = "no-memory-request"
	CodeUnschedulablePods  = "unschedulable-pods"
	CodeNoHeadroomMemory   = "no-headroom-memory"
	CodeNoHeadroomCPU      = "no-headroom-cpu"
	CodePodCapacity        = "pod-capacity"
	CodeHeartbeatLag       = "heartbeat-lag"
	CodeFlapping           = "flapping"
	CodeCordoned           = "cordoned"
	CodeRequestsFull       = "requests-full"
	CodeNoMemoryLimit      = "no-memory-limit"
	CodeKubeletSkew        = "kubelet-skew"
)

// Thresholds of the health checks, in percent of allocatable unless noted.
const (
	usageWarnPercent           = 80
	memoryCriticalPercent      = 90
	cpuCriticalPercent         = 95
	podCapacityWarnPercent     = 90
	requestsFullPercent        = 90
	heartbeatWarnSeconds       = 20
	heartbeatCriticalSeconds   = 40
	flappingTransitions        = 2
	overcommitLimitsPercent    = 100
	overcommitInfoPercent      = 150
	overcommitUsedPercent      = 70
	overcommitAboveReqPercent  = 10
	kubeletSupportedMinorsSkew = 3
)

// codePriority ranks the finding codes from most to least urgent within a
// severity: nodes down first, then pressure, then memory overcommitment,
// then the rest. Unknown codes go last.
var codePriority = []string{
	CodeUnreachable, CodeNotReady, CodeMemoryPressure, CodeDiskPressure, CodePIDPressure,
	CodeNetworkUnavailable, CodeMemoryOvercommit, CodeNodeCondition, CodeMemoryHigh, CodeCPUHigh,
	CodeOOMKills, CodeNoMemoryRequest, CodeUnschedulablePods, CodeNoHeadroomMemory, CodeNoHeadroomCPU,
	CodePodCapacity, CodeHeartbeatLag, CodeFlapping, CodeCordoned, CodeRequestsFull,
	CodeNoMemoryLimit, CodeKubeletSkew,
}

// pressureCodes maps the pressure conditions to their finding codes.
var pressureCodes = map[string]string{
	string(corev1.NodeMemoryPressure): CodeMemoryPressure,
	string(corev1.NodeDiskPressure):   CodeDiskPressure,
	string(corev1.NodePIDPressure):    CodePIDPressure,
}

// autoscalerTaints are the taints set by autoscalers on nodes they drain.
var autoscalerTaints = []string{
	"karpenter.sh/disrupted",
	"ToBeDeletedByClusterAutoscaler",
	"DeletionCandidateOfClusterAutoscaler",
}

// NodeFinding is a finding raised on a node.
type NodeFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`

	// Value is a percent, a count or seconds, depending on the code.
	Value *int `json:"value,omitempty"`

	// Since is the time the condition behind the finding started.
	Since *metav1.Time `json:"since,omitempty"`

	// ConditionType is set for the node-condition and pressure codes.
	ConditionType string `json:"conditionType,omitempty"`

	// Reason and Message are the raw kubelet or agent text.
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// NodeAffected is a node covered by a finding line.
type NodeAffected struct {
	Node          string `json:"node"`
	Severity      string `json:"severity"`
	Value         *int   `json:"value,omitempty"`
	ConditionType string `json:"conditionType,omitempty"`
}

// NodeFindingLine is a finding type raised over one or more nodes or the
// cluster, as shown on a single line.
type NodeFindingLine struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`

	// Nodes is the number of distinct nodes, for per-node codes.
	Nodes int `json:"nodes,omitempty"`

	// Value is the max percent, the summed OOM kills, the pod count
	// or the version count, depending on the code.
	Value *int `json:"value,omitempty"`

	// Since is set for unreachable and not-ready with one node.
	Since *metav1.Time `json:"since,omitempty"`

	// Affected lists the nodes, worst first.
	Affected []NodeAffected `json:"affected,omitempty"`

	// Per-code extras.
	Reasons    []UnschedulableReason `json:"reasons,omitempty"`
	Namespaces []NamespacePods       `json:"namespaces,omitempty"`
	Versions   []string              `json:"versions,omitempty"`
	ShortBy    *float64              `json:"shortBy,omitempty"`
}

// NodeCheck is the result of a health check.
type NodeCheck struct {
	Name  string   `json:"name"`
	Codes []string `json:"codes"`

	// Status is one of raised, skipped or passed.
	Status string `json:"status"`

	// Fact is set for skipped and passed checks.
	Fact string `json:"fact,omitempty"`
}

// NodesSummary is the name-less nodes summary injected into the report as
// spec.nodes for all users. It carries only counts, totals, codes and
// numbers: no node-sourced string can reach it.
type NodesSummary struct {
	Down              int                  `json:"down"`
	Pressure          int                  `json:"pressure"`
	Cordoned          int                  `json:"cordoned"`
	UnschedulablePods int                  `json:"unschedulablePods"`
	MetricsAvailable  bool                 `json:"metricsAvailable"`
	CPU               CapacityResource     `json:"cpu"`
	Memory            CapacityResource     `json:"memory"`
	Pods              CapacityPods         `json:"pods"`
	Findings          NodesSummaryFindings `json:"findings"`
}

// NodesSummaryFindings holds the finding line counts per severity and the
// worst critical/warn lines.
type NodesSummaryFindings struct {
	Critical int                `json:"critical"`
	Warn     int                `json:"warn"`
	Info     int                `json:"info"`
	Top      []NodesSummaryLine `json:"top"`
}

// NodesSummaryLine is a finding line made of codes and numbers only.
type NodesSummaryLine struct {
	Code     string       `json:"code"`
	Severity string       `json:"severity"`
	Nodes    int          `json:"nodes,omitempty"`
	Value    *int         `json:"value,omitempty"`
	Since    *metav1.Time `json:"since,omitempty"`
}

// severityRank ranks the severities, worst first.
func severityRank(severity string) int {
	switch severity {
	case SeverityCritical:
		return 0
	case SeverityWarn:
		return 1
	default:
		return 2
	}
}

// codeRank ranks a finding code by priority, unknown codes last.
func codeRank(code string) int {
	if i := slices.Index(codePriority, code); i >= 0 {
		return i
	}
	return len(codePriority)
}

// worseSeverity returns the worst of two severities.
func worseSeverity(a, b string) string {
	if severityRank(b) < severityRank(a) {
		return b
	}
	return a
}

// intValue returns the value of an optional int, or -1 when unset.
func intValue(v *int) int {
	if v == nil {
		return -1
	}
	return *v
}

// condition returns the node condition of the given type or nil.
func (n *NodeDetail) condition(conditionType corev1.NodeConditionType) *NodeConditionInfo {
	for i := range n.Conditions {
		if n.Conditions[i].Type == string(conditionType) {
			return &n.Conditions[i]
		}
	}
	return nil
}

// usageSeverity returns the severity of a usage percent: memory is
// critical at 90% (close to kubelet eviction), CPU only throttles so it is
// critical at 95%. Both warn at 80%.
func usageSeverity(resource corev1.ResourceName, pct int) string {
	critical := memoryCriticalPercent
	if resource == corev1.ResourceCPU {
		critical = cpuCriticalPercent
	}
	switch {
	case pct >= critical:
		return SeverityCritical
	case pct >= usageWarnPercent:
		return SeverityWarn
	default:
		return ""
	}
}

// conditionFinding returns a finding for a node condition with its
// transition time and raw reason and message.
func conditionFinding(code, severity string, c *NodeConditionInfo) NodeFinding {
	f := NodeFinding{Code: code, Severity: severity}
	if c != nil {
		f.Reason = c.Reason
		f.Message = c.Message
		if !c.LastTransitionTime.IsZero() {
			f.Since = new(c.LastTransitionTime)
		}
	}
	return f
}

// nodeFindings evaluates the per-node rules. A cordoned node is expected to
// go away (drain, autoscaler), so its health findings are capped at info,
// like the kubernetes-mixin alerts, and the usage, requests and overcommit
// rules skip it.
func nodeFindings(n *NodeDetail) []NodeFinding {
	findings := []NodeFinding{}
	health := func(severity string) string {
		if n.Unschedulable {
			return SeverityInfo
		}
		return severity
	}

	ready := n.condition(corev1.NodeReady)
	switch n.Status {
	case NodeStatusUnreachable:
		findings = append(findings, conditionFinding(CodeUnreachable, health(SeverityCritical), ready))
	case NodeStatusNotReady:
		f := conditionFinding(CodeNotReady, health(SeverityCritical), ready)
		if ready == nil {
			f.Reason = reasonNoReadyCondition
		}
		findings = append(findings, f)
	}

	if c := n.condition(corev1.NodeNetworkUnavailable); c != nil && c.Status == string(corev1.ConditionTrue) {
		findings = append(findings, conditionFinding(CodeNetworkUnavailable, health(SeverityCritical), c))
	}

	// Conditions reported by node-problem-detector and similar agents.
	for i := range n.Conditions {
		c := &n.Conditions[i]
		if c.Status != string(corev1.ConditionTrue) ||
			slices.Contains(standardConditions, corev1.NodeConditionType(c.Type)) {
			continue
		}
		f := conditionFinding(CodeNodeCondition, health(SeverityWarn), c)
		f.ConditionType = c.Type
		findings = append(findings, f)
	}

	// The kubelet renews its Lease every ~10s and the node is marked
	// Unknown after the grace period, so a late Lease is the earliest
	// sign of a node going away.
	if n.Status != NodeStatusUnreachable && n.HeartbeatSeconds != nil && *n.HeartbeatSeconds > heartbeatWarnSeconds {
		severity := SeverityWarn
		if *n.HeartbeatSeconds > heartbeatCriticalSeconds {
			severity = SeverityCritical
		}
		findings = append(findings, NodeFinding{Code: CodeHeartbeatLag, Severity: health(severity), Value: new(*n.HeartbeatSeconds)})
	}

	// Readiness flapping (kubernetes-mixin KubeNodeReadinessFlapping).
	if n.ReadyTransitions != nil && *n.ReadyTransitions > flappingTransitions {
		findings = append(findings, NodeFinding{Code: CodeFlapping, Severity: health(SeverityWarn), Value: new(*n.ReadyTransitions)})
	}

	for _, p := range n.Pressures {
		f := conditionFinding(pressureCodes[p], health(SeverityCritical), n.condition(corev1.NodeConditionType(p)))
		f.ConditionType = p
		if p == string(corev1.NodeMemoryPressure) && n.Usage != nil {
			if pct, ok := percent(float64(n.Usage.Memory), float64(n.Allocatable.Memory)); ok {
				f.Value = new(pct)
			}
		}
		findings = append(findings, f)
	}

	findings = append(findings, capacityFindings(n)...)

	if n.Unschedulable {
		f := NodeFinding{Code: CodeCordoned, Severity: SeverityInfo}
		for _, t := range n.Taints {
			if t.Key == corev1.TaintNodeUnschedulable && t.TimeAdded != nil {
				f.Since = new(*t.TimeAdded)
			}
			// The autoscaler taint is shown as is; the reason is not inferred.
			if f.Reason == "" && slices.Contains(autoscalerTaints, t.Key) {
				f.Reason = t.Key
			}
		}
		findings = append(findings, f)
	}

	return findings
}

// capacityFindings evaluates the per-node usage, OOM, pod capacity and
// requests rules. Usage and requests rules apply to Ready=True nodes
// whatever the pressure, and skip cordoned nodes.
func capacityFindings(n *NodeDetail) []NodeFinding {
	var findings []NodeFinding
	if n.readyTrue && !n.Unschedulable && n.Usage != nil {
		if pct, ok := percent(float64(n.Usage.Memory), float64(n.Allocatable.Memory)); ok {
			if severity := usageSeverity(corev1.ResourceMemory, pct); severity != "" {
				findings = append(findings, NodeFinding{Code: CodeMemoryHigh, Severity: severity, Value: new(pct)})
			}
		}
		if pct, ok := percent(n.Usage.CPU, n.Allocatable.CPU); ok {
			if severity := usageSeverity(corev1.ResourceCPU, pct); severity != "" {
				findings = append(findings, NodeFinding{Code: CodeCPUHigh, Severity: severity, Value: new(pct)})
			}
		}
	}

	if n.OOMKills > 0 {
		findings = append(findings, NodeFinding{Code: CodeOOMKills, Severity: SeverityWarn, Value: new(n.OOMKills)})
	}

	if pct, ok := percent(float64(n.Pods), float64(n.Allocatable.Pods)); ok && pct >= podCapacityWarnPercent {
		findings = append(findings, NodeFinding{Code: CodePodCapacity, Severity: SeverityWarn, Value: new(pct)})
	}

	// A node with requests near allocatable is full for the scheduler,
	// whatever the usage. Placeholder pods are preempted, so they don't count.
	if n.readyTrue && !n.Unschedulable {
		full := -1
		if pct, ok := percent(n.Requests.CPU-n.placeholderRequests.CPU, n.Allocatable.CPU); ok && pct >= requestsFullPercent {
			full = max(full, pct)
		}
		if pct, ok := percent(float64(n.Requests.Memory-n.placeholderRequests.Memory), float64(n.Allocatable.Memory)); ok && pct >= requestsFullPercent {
			full = max(full, pct)
		}
		if full >= 0 {
			findings = append(findings, NodeFinding{Code: CodeRequestsFull, Severity: SeverityWarn, Value: new(full)})
		}
	}
	return findings
}

// evaluateNodes runs the per-node and cluster rules, sets the findings of
// every node, and the grouped finding lines and check results of the
// snapshot.
func evaluateNodes(snap *NodesSnapshot) {
	// Per-node findings grouped into one line per code, counting
	// distinct nodes.
	lines := make(map[string]*NodeFindingLine)
	var order []string
	for _, n := range snap.Nodes {
		n.Findings = nodeFindings(n)
		for _, f := range n.Findings {
			line, ok := lines[f.Code]
			if !ok {
				line = &NodeFindingLine{Code: f.Code, Severity: f.Severity}
				lines[f.Code] = line
				order = append(order, f.Code)
			}
			line.Severity = worseSeverity(line.Severity, f.Severity)
			if f.Value != nil {
				switch {
				case f.Code == CodeOOMKills:
					sum := *f.Value
					if line.Value != nil {
						sum += *line.Value
					}
					line.Value = new(sum)
				case line.Value == nil || *f.Value > *line.Value:
					line.Value = new(*f.Value)
				}
			}

			// One affected entry per node: a node with two failing
			// conditions counts once, with both condition types.
			idx := slices.IndexFunc(line.Affected, func(a NodeAffected) bool { return a.Node == n.Name })
			if idx < 0 {
				line.Affected = append(line.Affected, NodeAffected{
					Node: n.Name, Severity: f.Severity, Value: f.Value, ConditionType: f.ConditionType,
				})
				if line.Nodes == 0 {
					line.Since = f.Since
				}
				line.Nodes++
				continue
			}
			a := &line.Affected[idx]
			a.Severity = worseSeverity(a.Severity, f.Severity)
			if f.Value != nil && intValue(f.Value) > intValue(a.Value) {
				a.Value = f.Value
			}
			if f.ConditionType != "" {
				a.ConditionType = strings.Join([]string{a.ConditionType, f.ConditionType}, ", ")
			}
		}
	}

	result := make([]NodeFindingLine, 0, len(order)+6)
	for _, code := range order {
		line := lines[code]
		if line.Nodes != 1 || (code != CodeUnreachable && code != CodeNotReady) {
			line.Since = nil
		}
		result = append(result, *line)
	}

	// Cluster-wide rules.
	result = append(result, clusterFindings(snap)...)

	for i := range result {
		sortAffected(result[i].Affected)
	}
	slices.SortStableFunc(result, func(a, b NodeFindingLine) int {
		return cmp.Or(cmp.Compare(severityRank(a.Severity), severityRank(b.Severity)),
			cmp.Compare(codeRank(a.Code), codeRank(b.Code)))
	})
	snap.Findings = result

	for _, n := range snap.Nodes {
		slices.SortStableFunc(n.Findings, func(a, b NodeFinding) int {
			return cmp.Or(cmp.Compare(severityRank(a.Severity), severityRank(b.Severity)),
				cmp.Compare(codeRank(a.Code), codeRank(b.Code)),
				cmp.Compare(intValue(b.Value), intValue(a.Value)))
		})
	}

	snap.Checks = checkResults(snap)
}

// sortAffected sorts the affected nodes worst first:
// severity, then value descending, then name.
func sortAffected(affected []NodeAffected) {
	slices.SortStableFunc(affected, func(a, b NodeAffected) int {
		return cmp.Or(cmp.Compare(severityRank(a.Severity), severityRank(b.Severity)),
			cmp.Compare(intValue(b.Value), intValue(a.Value)),
			cmp.Compare(a.Node, b.Node))
	})
}

// clusterFindings evaluates the cluster-wide rules. The lines that are
// attributable to nodes (memory overcommit, unsupported kubelet versions)
// carry the affected nodes and are also added to the findings of those
// nodes. The pods without memory request line is cluster-wide only: it
// carries the pod count and namespaces, not the nodes.
func clusterFindings(snap *NodesSnapshot) []NodeFindingLine {
	var lines []NodeFindingLine
	attribute := func(n *NodeDetail, code, severity string, value *int) {
		n.Findings = append(n.Findings, NodeFinding{Code: code, Severity: severity, Value: value})
	}

	if snap.UnschedulablePods > 0 {
		lines = append(lines, NodeFindingLine{
			Code:     CodeUnschedulablePods,
			Severity: SeverityWarn,
			Value:    new(snap.UnschedulablePods),
			Reasons:  snap.UnschedulableReasons,
		})
	}

	// N-1 headroom (kubernetes-mixin KubeCPUOvercommit/KubeMemoryOvercommit):
	// can the Ready, schedulable, non-control-plane nodes absorb every
	// workload request if the largest of them is lost. Pods on down or
	// cordoned nodes need a new home too, so the requests of every
	// non-control-plane node count, except the preemptible placeholders.
	if h := headroom(snap.Nodes); h.nodes >= 2 {
		if short := h.requests.CPU - (h.allocatable.CPU - h.largest.CPU); short > 0 {
			lines = append(lines, NodeFindingLine{Code: CodeNoHeadroomCPU, Severity: SeverityWarn, ShortBy: new(short)})
		}
		if short := h.requests.Memory - (h.allocatable.Memory - h.largest.Memory); short > 0 {
			lines = append(lines, NodeFindingLine{Code: CodeNoHeadroomMemory, Severity: SeverityWarn, ShortBy: new(float64(short))})
		}
	}

	// Memory overcommit: low requests let the scheduler pack pods, high
	// limits let them grow past what was reserved. Memory can't be
	// throttled, so bursting ends in evictions and OOM kills.
	type overcommit struct {
		node     *NodeDetail
		limits   int
		bursting bool
	}
	var overcommitted []overcommit
	highest := 0
	anyBursting := false
	for _, n := range snap.Nodes {
		if !n.readyTrue || n.Unschedulable {
			continue
		}
		limits, ok := percent(float64(n.Limits.Memory), float64(n.Allocatable.Memory))
		if !ok || limits <= overcommitLimitsPercent {
			continue
		}
		bursting := false
		if n.Usage != nil && n.AboveRequests != nil {
			used, _ := percent(float64(n.Usage.Memory), float64(n.Allocatable.Memory))
			above, _ := percent(float64(n.AboveRequests.Memory), float64(n.Allocatable.Memory))
			bursting = used >= overcommitUsedPercent && above >= overcommitAboveReqPercent
		}
		overcommitted = append(overcommitted, overcommit{node: n, limits: limits, bursting: bursting})
		highest = max(highest, limits)
		anyBursting = anyBursting || bursting
	}
	if anyBursting || highest >= overcommitInfoPercent {
		line := NodeFindingLine{
			Code:     CodeMemoryOvercommit,
			Severity: SeverityInfo,
			Nodes:    len(overcommitted),
			Value:    new(highest),
		}
		if anyBursting {
			line.Severity = SeverityWarn
		}
		for _, o := range overcommitted {
			severity := SeverityInfo
			if o.bursting {
				severity = SeverityWarn
			}
			line.Affected = append(line.Affected, NodeAffected{Node: o.node.Name, Severity: severity, Value: new(o.limits)})
			attribute(o.node, CodeMemoryOvercommit, severity, new(o.limits))
		}
		lines = append(lines, line)
	}

	// Pods without memory bounds, two disjoint advisories: without request
	// (the scheduler reserves nothing for them) and without limit. Both are
	// often deliberate, so they never change the cluster status.
	noRequest, noLimit := 0, 0
	for _, n := range snap.Nodes {
		noRequest += n.PodsWithoutMemoryRequest
		noLimit += n.PodsWithoutMemoryLimit
	}
	if noRequest > 0 {
		lines = append(lines, NodeFindingLine{
			Code:       CodeNoMemoryRequest,
			Severity:   SeverityInfo,
			Value:      new(noRequest),
			Namespaces: snap.UnboundedNamespaces.WithoutRequest,
		})
	}
	if noLimit > 0 {
		lines = append(lines, NodeFindingLine{
			Code:       CodeNoMemoryLimit,
			Severity:   SeverityInfo,
			Value:      new(noLimit),
			Namespaces: snap.UnboundedNamespaces.WithoutLimit,
		})
	}

	// Kubelet version skew: kubelets may be up to 3 minors older than the
	// API server and never newer; outside that is unsupported.
	versions := kubeletVersions(snap.Nodes)
	serverMajor, serverMinor, serverOK := parseMinorVersion(snap.ControlPlaneVersion)
	unsupported := func(v string) bool {
		major, minor, ok := parseMinorVersion(v)
		if !ok || !serverOK {
			return false
		}
		return major != serverMajor || minor > serverMinor || serverMinor-minor > kubeletSupportedMinorsSkew
	}
	anyUnsupported := slices.ContainsFunc(versions, unsupported)
	if len(versions) > 1 || anyUnsupported {
		line := NodeFindingLine{
			Code:     CodeKubeletSkew,
			Severity: SeverityInfo,
			Value:    new(len(versions)),
			Versions: versions,
		}
		if anyUnsupported {
			line.Severity = SeverityWarn
			for _, n := range snap.Nodes {
				if unsupported(baseVersion(n.Info.KubeletVersion)) {
					line.Nodes++
					line.Affected = append(line.Affected, NodeAffected{Node: n.Name, Severity: SeverityWarn})
					attribute(n, CodeKubeletSkew, SeverityWarn, nil)
				}
			}
		}
		lines = append(lines, line)
	}

	return lines
}

// headroomTotals holds the N-1 headroom inputs.
type headroomTotals struct {
	nodes       int
	allocatable NodeResources
	largest     NodeResources
	requests    NodeResources
}

// headroom sums the N-1 headroom inputs: the capacity of the Ready,
// schedulable, non-control-plane nodes with the largest of them, and the
// requests of every non-control-plane node without the placeholder pods.
func headroom(nodes []*NodeDetail) headroomTotals {
	var h headroomTotals
	for _, n := range nodes {
		if n.ControlPlane {
			continue
		}
		h.requests.CPU += n.Requests.CPU - n.placeholderRequests.CPU
		h.requests.Memory += n.Requests.Memory - n.placeholderRequests.Memory
		if !n.readyTrue || n.Unschedulable {
			continue
		}
		h.nodes++
		h.allocatable.CPU += n.Allocatable.CPU
		h.allocatable.Memory += n.Allocatable.Memory
		h.largest.CPU = max(h.largest.CPU, n.Allocatable.CPU)
		h.largest.Memory = max(h.largest.Memory, n.Allocatable.Memory)
	}
	return h
}

// baseVersion strips the build metadata from a Kubernetes version:
// v1.31.6+67d3387 -> v1.31.6.
func baseVersion(v string) string {
	v, _, _ = strings.Cut(v, "+")
	return v
}

// kubeletVersions returns the sorted distinct kubelet base versions.
func kubeletVersions(nodes []*NodeDetail) []string {
	versions := []string{}
	for _, n := range nodes {
		if v := baseVersion(n.Info.KubeletVersion); v != "" && !slices.Contains(versions, v) {
			versions = append(versions, v)
		}
	}
	slices.Sort(versions)
	return versions
}

// parseMinorVersion returns the major and minor numbers of a Kubernetes
// version such as v1.33.4, v1.33.4-eks-1 or v1.31.6+67d3387.
func parseMinorVersion(v string) (major, minor int, ok bool) {
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	digits := parts[1]
	if i := strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }); i >= 0 {
		digits = digits[:i]
	}
	minor, err = strconv.Atoi(digits)
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// plural returns "1 node" or "N nodes".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// nodeCheck is a health check of the catalogue: it owns the finding codes
// it can raise and passes when none of them is raised, with a fact.
// A check is skipped, with a fact, when its data is missing; missing data
// never passes.
type nodeCheck struct {
	name   string
	codes  []string
	skip   func(snap *NodesSnapshot) string
	passed func(snap *NodesSnapshot) string
}

// coverage counts the candidate nodes and those with data.
func coverage(snap *NodesSnapshot, candidate, hasData func(n *NodeDetail) bool) (withData, total int) {
	for _, n := range snap.Nodes {
		if !candidate(n) {
			continue
		}
		total++
		if hasData(n) {
			withData++
		}
	}
	return withData, total
}

// coverageFact returns the passed fact for all candidates, or prefixed
// with "N of M" when only some candidates have data.
func coverageFact(withData, total int, all, partial string) string {
	if withData == total {
		return all
	}
	return fmt.Sprintf("%d of %d %s", withData, total, partial)
}

// usageCandidate reports whether the usage rules apply to the node.
func usageCandidate(n *NodeDetail) bool { return n.readyTrue && !n.Unschedulable }

// hasUsage reports whether the node has metrics.
func hasUsage(n *NodeDetail) bool { return n.Usage != nil }

// heartbeatCandidate reports whether the heartbeat rule applies to the node.
func heartbeatCandidate(n *NodeDetail) bool { return n.Status != NodeStatusUnreachable }

// hasLease reports whether the node has a Lease.
func hasLease(n *NodeDetail) bool { return n.HeartbeatSeconds != nil }

// anyNode matches every node.
func anyNode(*NodeDetail) bool { return true }

// hasTransitions reports whether the node has 15m of Ready history.
func hasTransitions(n *NodeDetail) bool { return n.ReadyTransitions != nil }

// skipUsage skips the usage checks when no node has metrics.
func skipUsage(snap *NodesSnapshot) string {
	if withData, _ := coverage(snap, usageCandidate, hasUsage); withData > 0 {
		return ""
	}
	if !snap.MetricsAvailable {
		return "no metrics-server"
	}
	return "no node metrics"
}

// passedUsage returns the passed fact of the usage checks.
func passedUsage(snap *NodesSnapshot) string {
	withData, total := coverage(snap, usageCandidate, hasUsage)
	return coverageFact(withData, total, "all nodes below 80%", "nodes below 80%")
}

// fixedFact returns a passed fact function returning the given fact.
func fixedFact(fact string) func(*NodesSnapshot) string {
	return func(*NodesSnapshot) string { return fact }
}

// nodeChecks is the health check catalogue in display order.
var nodeChecks = []nodeCheck{
	{name: "Node readiness", codes: []string{CodeUnreachable, CodeNotReady},
		skip: func(snap *NodesSnapshot) string {
			unknown := 0
			for _, n := range snap.Nodes {
				if n.Status == NodeStatusUnknown {
					unknown++
				}
			}
			if unknown > 0 {
				return plural(unknown, "node") + " without Ready condition"
			}
			return ""
		},
		passed: func(snap *NodesSnapshot) string {
			ready := 0
			for _, n := range snap.Nodes {
				if n.readyTrue {
					ready++
				}
			}
			return plural(ready, "node") + " Ready"
		}},
	{name: "Memory pressure", codes: []string{CodeMemoryPressure}, passed: fixedFact("none")},
	{name: "Disk pressure", codes: []string{CodeDiskPressure}, passed: fixedFact("none")},
	{name: "PID pressure", codes: []string{CodePIDPressure}, passed: fixedFact("none")},
	{name: "Network", codes: []string{CodeNetworkUnavailable}, passed: fixedFact("available on all nodes")},
	{name: "Node conditions", codes: []string{CodeNodeCondition}, passed: fixedFact("no failing conditions")},
	{name: "Memory usage", codes: []string{CodeMemoryHigh}, skip: skipUsage, passed: passedUsage},
	{name: "CPU usage", codes: []string{CodeCPUHigh}, skip: skipUsage, passed: passedUsage},
	{name: "Memory overcommitment", codes: []string{CodeMemoryOvercommit}, passed: func(snap *NodesSnapshot) string {
		for _, n := range snap.Nodes {
			if !n.readyTrue || n.Unschedulable {
				continue
			}
			if pct, ok := percent(float64(n.Limits.Memory), float64(n.Allocatable.Memory)); ok && pct > overcommitLimitsPercent {
				return "memory limits below 150% of allocatable"
			}
		}
		return "memory limits within allocatable"
	}},
	{name: "OOM kills", codes: []string{CodeOOMKills}, passed: fixedFact("none in the last hour")},
	{name: "Memory requests", codes: []string{CodeNoMemoryRequest}, passed: fixedFact("all pods set a memory request")},
	{name: "Pod scheduling", codes: []string{CodeUnschedulablePods}, passed: fixedFact("no unschedulable pods")},
	{name: "N-1 headroom", codes: []string{CodeNoHeadroomCPU, CodeNoHeadroomMemory},
		skip: func(snap *NodesSnapshot) string {
			if headroom(snap.Nodes).nodes < 2 {
				return "single node"
			}
			return ""
		},
		passed: fixedFact("requests fit with one node down")},
	{name: "Pod capacity", codes: []string{CodePodCapacity}, passed: fixedFact("all nodes below 90% of pod slots")},
	{name: "Heartbeat", codes: []string{CodeHeartbeatLag},
		skip: func(snap *NodesSnapshot) string {
			if withData, _ := coverage(snap, heartbeatCandidate, hasLease); withData == 0 {
				return "no node leases"
			}
			return ""
		},
		passed: func(snap *NodesSnapshot) string {
			withData, total := coverage(snap, heartbeatCandidate, hasLease)
			return coverageFact(withData, total, "all leases renewed within 20s", "leases renewed within 20s")
		}},
	{name: "Readiness flapping", codes: []string{CodeFlapping},
		skip: func(snap *NodesSnapshot) string {
			if withData, _ := coverage(snap, anyNode, hasTransitions); withData == 0 {
				return "collecting history"
			}
			return ""
		},
		passed: func(snap *NodesSnapshot) string {
			withData, total := coverage(snap, anyNode, hasTransitions)
			return coverageFact(withData, total, "no flapping in the last 15m", "nodes without flapping in the last 15m")
		}},
	{name: "Requests", codes: []string{CodeRequestsFull}, passed: fixedFact("all nodes below 90% requested")},
	{name: "Cordoned nodes", codes: []string{CodeCordoned}, passed: fixedFact("none")},
	{name: "Memory limits", codes: []string{CodeNoMemoryLimit}, passed: func(snap *NodesSnapshot) string {
		for _, n := range snap.Nodes {
			if n.PodsWithoutMemoryRequest > 0 {
				return "all pods with a request set a limit"
			}
		}
		return "all pods set a memory limit"
	}},
	{name: "Kubelet versions", codes: []string{CodeKubeletSkew}, passed: func(snap *NodesSnapshot) string {
		return strings.Join(kubeletVersions(snap.Nodes), ", ")
	}},
}

// checkResults returns the result of every check of the catalogue,
// in catalogue order: raised when any of its codes is raised, else
// skipped when its data is missing, else passed.
func checkResults(snap *NodesSnapshot) []NodeCheck {
	raised := make(map[string]bool, len(snap.Findings))
	for _, l := range snap.Findings {
		raised[l.Code] = true
	}

	results := make([]NodeCheck, 0, len(nodeChecks))
	for _, c := range nodeChecks {
		result := NodeCheck{Name: c.name, Codes: c.codes}
		switch {
		case slices.ContainsFunc(c.codes, func(code string) bool { return raised[code] }):
			result.Status = CheckRaised
		case c.skip != nil && c.skip(snap) != "":
			result.Status = CheckSkipped
			result.Fact = c.skip(snap)
		default:
			result.Status = CheckPassed
			result.Fact = c.passed(snap)
		}
		results = append(results, result)
	}
	return results
}

// newNodesSummary builds the name-less summary from the snapshot: counts,
// capacity totals and the finding lines reduced to codes and numbers.
// A cordoned node counts only as cordoned, never also as down or under
// pressure.
func newNodesSummary(snap *NodesSnapshot) *NodesSummary {
	s := &NodesSummary{
		UnschedulablePods: snap.UnschedulablePods,
		MetricsAvailable:  snap.MetricsAvailable,
		CPU:               snap.Capacity.CPU,
		Memory:            snap.Capacity.Memory,
		Pods:              snap.Capacity.Pods,
		Findings:          NodesSummaryFindings{Top: []NodesSummaryLine{}},
	}
	for _, n := range snap.Nodes {
		switch {
		case n.Unschedulable:
			s.Cordoned++
		case n.Status == NodeStatusUnreachable || n.Status == NodeStatusNotReady:
			s.Down++
		case n.Status == NodeStatusPressure:
			s.Pressure++
		}
	}

	// Only codes and numbers leave the lines.
	for _, l := range snap.Findings {
		switch l.Severity {
		case SeverityCritical:
			s.Findings.Critical++
		case SeverityWarn:
			s.Findings.Warn++
		default:
			s.Findings.Info++
			continue
		}
		if len(s.Findings.Top) < 3 {
			s.Findings.Top = append(s.Findings.Top, NodesSummaryLine{
				Code:     l.Code,
				Severity: l.Severity,
				Nodes:    l.Nodes,
				Value:    l.Value,
				Since:    l.Since,
			})
		}
	}
	return s
}
