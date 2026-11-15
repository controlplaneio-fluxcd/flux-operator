// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// readyNode returns a Ready node detail with round allocatable figures:
// 10 cores, 1000 bytes of memory and 100 pods, so that the usage and
// request amounts read as percentages.
func readyNode(name string, mutate ...func(*NodeDetail)) *NodeDetail {
	n := &NodeDetail{
		Name:        name,
		Status:      NodeStatusReady,
		Roles:       []string{},
		Pressures:   []string{},
		Taints:      []NodeTaint{},
		Info:        NodeSystemInfo{KubeletVersion: "v1.33.4"},
		Allocatable: NodeAllocatable{CPU: 10, Memory: 1000, Pods: 100},
		Conditions: []NodeConditionInfo{{
			Type:               string(corev1.NodeReady),
			Status:             string(corev1.ConditionTrue),
			LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
		}},
		HeartbeatSeconds: new(5),
		ReadyTransitions: new(0),
		readyTrue:        true,
	}
	for _, m := range mutate {
		m(n)
	}
	return n
}

// withCondition sets a node condition and updates the status fields
// derived from it.
func withCondition(conditionType corev1.NodeConditionType, status corev1.ConditionStatus) func(*NodeDetail) {
	return func(n *NodeDetail) {
		c := NodeConditionInfo{
			Type:               string(conditionType),
			Status:             string(status),
			Reason:             "TestReason",
			Message:            "test message",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}
		replaced := false
		for i := range n.Conditions {
			if n.Conditions[i].Type == c.Type {
				n.Conditions[i] = c
				replaced = true
			}
		}
		if !replaced {
			n.Conditions = append(n.Conditions, c)
		}
		switch {
		case conditionType == corev1.NodeReady && status == corev1.ConditionUnknown:
			n.Status, n.readyTrue = NodeStatusUnreachable, false
		case conditionType == corev1.NodeReady && status == corev1.ConditionFalse:
			n.Status, n.readyTrue = NodeStatusNotReady, false
		case pressureCodes[string(conditionType)] != "" && status == corev1.ConditionTrue:
			n.Pressures = append(n.Pressures, string(conditionType))
			if n.readyTrue {
				n.Status = NodeStatusPressure
			}
		}
	}
}

func withUsage(cpu float64, memory int64) func(*NodeDetail) {
	return func(n *NodeDetail) { n.Usage = &NodeResources{CPU: cpu, Memory: memory} }
}

func cordoned(n *NodeDetail) {
	n.Unschedulable = true
	if n.Status == NodeStatusReady {
		n.Status = NodeStatusCordoned
	}
}

// evaluate runs the rules over the given nodes with metrics available.
func evaluate(nodes ...*NodeDetail) *NodesSnapshot {
	snap := &NodesSnapshot{
		MetricsAvailable:    true,
		ControlPlaneVersion: "v1.33.4",
		Nodes:               nodes,
	}
	evaluateNodes(snap)
	snap.summary = newNodesSummary(snap)
	return snap
}

func findLine(snap *NodesSnapshot, code string) *NodeFindingLine {
	for i := range snap.Findings {
		if snap.Findings[i].Code == code {
			return &snap.Findings[i]
		}
	}
	return nil
}

func findNodeFinding(n *NodeDetail, code string) *NodeFinding {
	for i := range n.Findings {
		if n.Findings[i].Code == code {
			return &n.Findings[i]
		}
	}
	return nil
}

func findCheck(snap *NodesSnapshot, name string) NodeCheck {
	for _, c := range snap.Checks {
		if c.Name == name {
			return c
		}
	}
	return NodeCheck{}
}

func TestPercent(t *testing.T) {
	g := NewWithT(t)

	pct, ok := percent(0.945, 1.05)
	g.Expect(ok).To(BeTrue())
	g.Expect(pct).To(Equal(90))

	pct, _ = percent(79.9, 100)
	g.Expect(pct).To(Equal(79))

	pct, _ = percent(250, 100)
	g.Expect(pct).To(Equal(250))

	_, ok = percent(1, 0)
	g.Expect(ok).To(BeFalse())
}

func TestNodeFindings_UsageThresholds(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		cpu      float64
		memory   int64
		severity string
	}{
		{name: "memory 79%", code: CodeMemoryHigh, memory: 790, severity: ""},
		{name: "memory 80%", code: CodeMemoryHigh, memory: 800, severity: SeverityWarn},
		{name: "memory 89%", code: CodeMemoryHigh, memory: 890, severity: SeverityWarn},
		{name: "memory 90%", code: CodeMemoryHigh, memory: 900, severity: SeverityCritical},
		{name: "cpu 79%", code: CodeCPUHigh, cpu: 7.9, severity: ""},
		{name: "cpu 80%", code: CodeCPUHigh, cpu: 8, severity: SeverityWarn},
		{name: "cpu 94%", code: CodeCPUHigh, cpu: 9.4, severity: SeverityWarn},
		{name: "cpu 95%", code: CodeCPUHigh, cpu: 9.5, severity: SeverityCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			n := readyNode("node-1", withUsage(tt.cpu, tt.memory))
			snap := evaluate(n)

			f := findNodeFinding(n, tt.code)
			if tt.severity == "" {
				g.Expect(f).To(BeNil())
				g.Expect(findLine(snap, tt.code)).To(BeNil())
				return
			}
			g.Expect(f).NotTo(BeNil())
			g.Expect(f.Severity).To(Equal(tt.severity))
			g.Expect(findLine(snap, tt.code).Severity).To(Equal(tt.severity))
		})
	}
}

func TestNodeFindings_MissingMetrics(t *testing.T) {
	g := NewWithT(t)

	// Without metrics-server the usage checks are skipped, never passed.
	snap := &NodesSnapshot{Nodes: []*NodeDetail{readyNode("node-1")}, ControlPlaneVersion: "v1.33.4"}
	evaluateNodes(snap)
	for _, name := range []string{"Memory usage", "CPU usage"} {
		c := findCheck(snap, name)
		g.Expect(c.Status).To(Equal(CheckSkipped))
		g.Expect(c.Fact).To(Equal("no metrics-server"))
	}

	// With metrics-server but no node metrics, the checks are skipped too.
	snap = evaluate(readyNode("node-1"), readyNode("node-2"))
	g.Expect(findLine(snap, CodeMemoryHigh)).To(BeNil())
	g.Expect(findCheck(snap, "Memory usage")).To(Equal(NodeCheck{
		Name: "Memory usage", Codes: []string{CodeMemoryHigh}, Status: CheckSkipped, Fact: "no node metrics",
	}))

	// With partial coverage the fact says how many nodes were checked;
	// cordoned and not ready nodes are not candidates.
	snap = evaluate(readyNode("node-1", withUsage(1, 100)), readyNode("node-2"),
		readyNode("node-3", cordoned), readyNode("node-4", withCondition(corev1.NodeReady, corev1.ConditionFalse)))
	g.Expect(findCheck(snap, "Memory usage")).To(Equal(NodeCheck{
		Name: "Memory usage", Codes: []string{CodeMemoryHigh}, Status: CheckPassed, Fact: "1 of 2 nodes below 80%",
	}))
	g.Expect(findCheck(snap, "CPU usage").Fact).To(Equal("1 of 2 nodes below 80%"))

	snap = evaluate(readyNode("node-1", withUsage(1, 100)), readyNode("node-2", withUsage(1, 100)))
	g.Expect(findCheck(snap, "Memory usage").Fact).To(Equal("all nodes below 80%"))
}

func TestNodeFindings_HeartbeatCoverage(t *testing.T) {
	g := NewWithT(t)
	noLease := func(n *NodeDetail) { n.HeartbeatSeconds = nil }

	snap := evaluate(readyNode("node-1", noLease), readyNode("node-2", noLease))
	g.Expect(findCheck(snap, "Heartbeat")).To(Equal(NodeCheck{
		Name: "Heartbeat", Codes: []string{CodeHeartbeatLag}, Status: CheckSkipped, Fact: "no node leases",
	}))

	// Unreachable nodes are not candidates.
	snap = evaluate(readyNode("node-1"), readyNode("node-2", noLease),
		readyNode("node-3", withCondition(corev1.NodeReady, corev1.ConditionUnknown), noLease))
	g.Expect(findCheck(snap, "Heartbeat").Status).To(Equal(CheckPassed))
	g.Expect(findCheck(snap, "Heartbeat").Fact).To(Equal("1 of 2 leases renewed within 20s"))

	snap = evaluate(readyNode("node-1"), readyNode("node-2"))
	g.Expect(findCheck(snap, "Heartbeat").Fact).To(Equal("all leases renewed within 20s"))
}

func TestNodeFindings_FlappingCoverage(t *testing.T) {
	g := NewWithT(t)
	warmingUp := func(n *NodeDetail) { n.ReadyTransitions = nil }

	snap := evaluate(readyNode("node-1", warmingUp), readyNode("node-2", warmingUp))
	g.Expect(findCheck(snap, "Readiness flapping")).To(Equal(NodeCheck{
		Name: "Readiness flapping", Codes: []string{CodeFlapping}, Status: CheckSkipped, Fact: "collecting history",
	}))

	snap = evaluate(readyNode("node-1"), readyNode("node-2", warmingUp), readyNode("node-3"))
	g.Expect(findCheck(snap, "Readiness flapping").Status).To(Equal(CheckPassed))
	g.Expect(findCheck(snap, "Readiness flapping").Fact).To(Equal("2 of 3 nodes without flapping in the last 15m"))

	snap = evaluate(readyNode("node-1"))
	g.Expect(findCheck(snap, "Readiness flapping").Fact).To(Equal("no flapping in the last 15m"))
}

func TestNodeFindings_Heartbeat(t *testing.T) {
	tests := []struct {
		seconds  int
		severity string
	}{
		{seconds: 20, severity: ""},
		{seconds: 21, severity: SeverityWarn},
		{seconds: 40, severity: SeverityWarn},
		{seconds: 41, severity: SeverityCritical},
	}
	for _, tt := range tests {
		g := NewWithT(t)
		n := readyNode("node-1", func(n *NodeDetail) { n.HeartbeatSeconds = new(tt.seconds) })
		evaluate(n)
		f := findNodeFinding(n, CodeHeartbeatLag)
		if tt.severity == "" {
			g.Expect(f).To(BeNil(), "%ds", tt.seconds)
			continue
		}
		g.Expect(f).NotTo(BeNil(), "%ds", tt.seconds)
		g.Expect(f.Severity).To(Equal(tt.severity), "%ds", tt.seconds)
		g.Expect(*f.Value).To(Equal(tt.seconds))
	}

	// An unreachable node gets no heartbeat finding on top.
	g := NewWithT(t)
	n := readyNode("node-1", withCondition(corev1.NodeReady, corev1.ConditionUnknown),
		func(n *NodeDetail) { n.HeartbeatSeconds = new(300) })
	evaluate(n)
	g.Expect(findNodeFinding(n, CodeHeartbeatLag)).To(BeNil())
	g.Expect(findNodeFinding(n, CodeUnreachable)).NotTo(BeNil())
}

func TestNodeFindings_Flapping(t *testing.T) {
	g := NewWithT(t)

	two := readyNode("node-1", func(n *NodeDetail) { n.ReadyTransitions = new(2) })
	three := readyNode("node-2", func(n *NodeDetail) { n.ReadyTransitions = new(3) })
	unknown := readyNode("node-3", func(n *NodeDetail) { n.ReadyTransitions = nil })
	snap := evaluate(two, three, unknown)

	g.Expect(findNodeFinding(two, CodeFlapping)).To(BeNil())
	g.Expect(findNodeFinding(unknown, CodeFlapping)).To(BeNil())
	f := findNodeFinding(three, CodeFlapping)
	g.Expect(f).NotTo(BeNil())
	g.Expect(f.Severity).To(Equal(SeverityWarn))
	g.Expect(*f.Value).To(Equal(3))
	g.Expect(findLine(snap, CodeFlapping).Nodes).To(Equal(1))
}

func TestNodeFindings_Readiness(t *testing.T) {
	g := NewWithT(t)

	unreachable := readyNode("node-1", withCondition(corev1.NodeReady, corev1.ConditionUnknown))
	notReady := readyNode("node-2", withCondition(corev1.NodeReady, corev1.ConditionFalse))
	snap := evaluate(unreachable, notReady, readyNode("node-3"))

	line := findLine(snap, CodeUnreachable)
	g.Expect(line.Severity).To(Equal(SeverityCritical))
	g.Expect(line.Nodes).To(Equal(1))
	g.Expect(line.Since).NotTo(BeNil())

	f := findNodeFinding(notReady, CodeNotReady)
	g.Expect(f.Severity).To(Equal(SeverityCritical))
	g.Expect(f.Message).To(Equal("test message"))
	g.Expect(f.Reason).To(Equal("TestReason"))

	// Lines are sorted by severity then code priority.
	g.Expect(snap.Findings[0].Code).To(Equal(CodeUnreachable))
	g.Expect(snap.Findings[1].Code).To(Equal(CodeNotReady))
	g.Expect(snap.summary.Down).To(Equal(2))
	g.Expect(findCheck(snap, "Node readiness").Status).To(Equal(CheckRaised))

	// With more than one node, the line carries no since.
	snap = evaluate(
		readyNode("node-1", withCondition(corev1.NodeReady, corev1.ConditionUnknown)),
		readyNode("node-2", withCondition(corev1.NodeReady, corev1.ConditionUnknown)))
	g.Expect(findLine(snap, CodeUnreachable).Nodes).To(Equal(2))
	g.Expect(findLine(snap, CodeUnreachable).Since).To(BeNil())

	// A node without a Ready condition is Unknown during the grace
	// period: no readiness finding, and the check can't pass.
	now := time.Now()
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "new", CreationTimestamp: metav1.NewTime(now.Add(-2 * time.Minute))}}
	g.Expect(nodeStatus(node, now)).To(Equal(NodeStatusUnknown))
	unknown := newNodeDetail(node, now)
	snap = evaluate(unknown, readyNode("node-1"))
	g.Expect(snap.Findings).To(BeEmpty())
	g.Expect(snap.summary.Down).To(BeZero())
	g.Expect(findCheck(snap, "Node readiness")).To(Equal(NodeCheck{
		Name: "Node readiness", Codes: []string{CodeUnreachable, CodeNotReady}, Status: CheckSkipped,
		Fact: "1 node without Ready condition",
	}))

	// Past the grace period it is not ready.
	node.CreationTimestamp = metav1.NewTime(now.Add(-2*time.Minute - time.Second))
	g.Expect(nodeStatus(node, now)).To(Equal(NodeStatusNotReady))
	old := newNodeDetail(node, now)
	snap = evaluate(old)
	f = findNodeFinding(old, CodeNotReady)
	g.Expect(f).To(Equal(&NodeFinding{Code: CodeNotReady, Severity: SeverityCritical, Reason: "NoReadyCondition"}))
	g.Expect(snap.summary.Down).To(Equal(1))
	g.Expect(findCheck(snap, "Node readiness").Status).To(Equal(CheckRaised))

	// Unreachable findings keep the raw reason and message.
	unreachable = readyNode("node-2", withCondition(corev1.NodeReady, corev1.ConditionUnknown))
	evaluate(unreachable)
	f = findNodeFinding(unreachable, CodeUnreachable)
	g.Expect(f.Reason).To(Equal("TestReason"))
	g.Expect(f.Message).To(Equal("test message"))
	g.Expect(f.Since).NotTo(BeNil())
}

func TestNodeFindings_CordonedSuppression(t *testing.T) {
	g := NewWithT(t)

	notReady := readyNode("node-1", withCondition(corev1.NodeReady, corev1.ConditionUnknown), cordoned)
	pressure := readyNode("node-2", withCondition(corev1.NodeMemoryPressure, corev1.ConditionTrue), cordoned,
		withUsage(9.9, 990), func(n *NodeDetail) {
			n.Taints = []NodeTaint{
				{Key: corev1.TaintNodeUnschedulable, Effect: "NoSchedule", TimeAdded: new(metav1.NewTime(time.Now()))},
				{Key: "karpenter.sh/disrupted", Effect: "NoSchedule"},
			}
		})
	snap := evaluate(notReady, pressure)

	g.Expect(findNodeFinding(notReady, CodeUnreachable).Severity).To(Equal(SeverityInfo))
	g.Expect(findNodeFinding(pressure, CodeMemoryPressure).Severity).To(Equal(SeverityInfo))

	// Usage rules skip cordoned nodes.
	g.Expect(findNodeFinding(pressure, CodeMemoryHigh)).To(BeNil())
	g.Expect(findNodeFinding(pressure, CodeCPUHigh)).To(BeNil())

	cordonedFinding := findNodeFinding(pressure, CodeCordoned)
	g.Expect(cordonedFinding.Severity).To(Equal(SeverityInfo))
	g.Expect(cordonedFinding.Since).NotTo(BeNil())
	g.Expect(cordonedFinding.Reason).To(Equal("karpenter.sh/disrupted"))

	// A cordoned node counts only as cordoned.
	g.Expect(snap.summary.Down).To(BeZero())
	g.Expect(snap.summary.Pressure).To(BeZero())
	g.Expect(snap.summary.Cordoned).To(Equal(2))
	g.Expect(snap.summary.Findings.Critical).To(BeZero())
	g.Expect(snap.summary.Findings.Warn).To(BeZero())
	g.Expect(snap.summary.Findings.Top).To(BeEmpty())
}

func TestNodeFindings_PressureStillEvaluatesUsage(t *testing.T) {
	g := NewWithT(t)

	// A Ready=True node under memory pressure at 93% memory, 96% CPU and
	// 200% memory limits raises pressure, usage and overcommit findings.
	n := readyNode("node-1", withCondition(corev1.NodeMemoryPressure, corev1.ConditionTrue), withUsage(9.6, 930),
		func(n *NodeDetail) {
			n.Limits.Memory = 2000
			n.AboveRequests = &NodeResources{Memory: 500}
		})
	snap := evaluate(n)

	pressure := findNodeFinding(n, CodeMemoryPressure)
	g.Expect(pressure.Severity).To(Equal(SeverityCritical))
	g.Expect(pressure.ConditionType).To(Equal("MemoryPressure"))
	g.Expect(pressure.Message).To(Equal("test message"))
	g.Expect(*pressure.Value).To(Equal(93))
	g.Expect(findNodeFinding(n, CodeMemoryHigh).Severity).To(Equal(SeverityCritical))
	g.Expect(findNodeFinding(n, CodeCPUHigh).Severity).To(Equal(SeverityCritical))
	g.Expect(findNodeFinding(n, CodeMemoryOvercommit).Severity).To(Equal(SeverityWarn))
	g.Expect(snap.summary.Pressure).To(Equal(1))

	// Node findings are sorted by severity then code priority.
	codes := make([]string, 0, len(n.Findings))
	for _, f := range n.Findings {
		codes = append(codes, f.Code)
	}
	g.Expect(codes).To(Equal([]string{CodeMemoryPressure, CodeMemoryHigh, CodeCPUHigh, CodeMemoryOvercommit}))
}

func TestNodeFindings_NetworkAndConditions(t *testing.T) {
	g := NewWithT(t)

	n := readyNode("node-1",
		withCondition(corev1.NodeNetworkUnavailable, corev1.ConditionTrue),
		withCondition("ReadonlyFilesystem", corev1.ConditionTrue),
		withCondition("KernelDeadlock", corev1.ConditionTrue),
		withCondition("FrequentKubeletRestart", corev1.ConditionFalse))
	other := readyNode("node-2", withCondition(corev1.NodeNetworkUnavailable, corev1.ConditionFalse))
	snap := evaluate(n, other)

	g.Expect(findLine(snap, CodeNetworkUnavailable).Nodes).To(Equal(1))
	network := findNodeFinding(n, CodeNetworkUnavailable)
	g.Expect(network.Since).NotTo(BeNil())
	g.Expect(network.Reason).To(Equal("TestReason"))
	g.Expect(network.Message).To(Equal("test message"))
	g.Expect(findLine(snap, CodeNetworkUnavailable).Severity).To(Equal(SeverityCritical))

	// Two failing conditions on one node count as one node.
	line := findLine(snap, CodeNodeCondition)
	g.Expect(line.Severity).To(Equal(SeverityWarn))
	g.Expect(line.Nodes).To(Equal(1))
	g.Expect(line.Affected).To(HaveLen(1))
	g.Expect(line.Affected[0].ConditionType).To(Equal("ReadonlyFilesystem, KernelDeadlock"))

	var types []string
	for _, f := range n.Findings {
		if f.Code == CodeNodeCondition {
			types = append(types, f.ConditionType)
		}
	}
	g.Expect(types).To(ConsistOf("ReadonlyFilesystem", "KernelDeadlock"))
}

func TestNodeFindings_PodCapacityAndOOM(t *testing.T) {
	g := NewWithT(t)

	at89 := readyNode("node-1", func(n *NodeDetail) { n.Pods = 89; n.OOMKills = 2 })
	at90 := readyNode("node-2", func(n *NodeDetail) { n.Pods = 90; n.OOMKills = 3 })
	snap := evaluate(at89, at90)

	g.Expect(findNodeFinding(at89, CodePodCapacity)).To(BeNil())
	g.Expect(*findNodeFinding(at90, CodePodCapacity).Value).To(Equal(90))

	line := findLine(snap, CodeOOMKills)
	g.Expect(line.Nodes).To(Equal(2))
	g.Expect(*line.Value).To(Equal(5))
	g.Expect(line.Affected[0].Node).To(Equal("node-2"))
}

func TestNodeFindings_RequestsFull(t *testing.T) {
	g := NewWithT(t)

	at89 := readyNode("node-1", func(n *NodeDetail) { n.Requests = NodeResources{CPU: 8.9, Memory: 890} })
	at90 := readyNode("node-2", func(n *NodeDetail) { n.Requests = NodeResources{CPU: 5, Memory: 900} })
	float := readyNode("node-3", func(n *NodeDetail) {
		n.Allocatable.CPU = 1.05
		n.Requests.CPU = 0.945
	})
	placeholder := readyNode("node-4", func(n *NodeDetail) {
		n.Requests = NodeResources{CPU: 9.5}
		n.placeholderRequests = NodeResources{CPU: 2}
	})
	cordonedNode := readyNode("node-5", cordoned, func(n *NodeDetail) { n.Requests = NodeResources{CPU: 10} })
	evaluate(at89, at90, float, placeholder, cordonedNode)

	g.Expect(findNodeFinding(at89, CodeRequestsFull)).To(BeNil())
	g.Expect(*findNodeFinding(at90, CodeRequestsFull).Value).To(Equal(90))
	g.Expect(*findNodeFinding(float, CodeRequestsFull).Value).To(Equal(90))
	g.Expect(findNodeFinding(placeholder, CodeRequestsFull)).To(BeNil())
	g.Expect(findNodeFinding(cordonedNode, CodeRequestsFull)).To(BeNil())
}

func TestNodeFindings_Headroom(t *testing.T) {
	g := NewWithT(t)

	// A single capacity node skips the check.
	snap := evaluate(readyNode("node-1", func(n *NodeDetail) { n.Requests.CPU = 20 }))
	g.Expect(findLine(snap, CodeNoHeadroomCPU)).To(BeNil())
	g.Expect(findCheck(snap, "N-1 headroom")).To(Equal(NodeCheck{
		Name: "N-1 headroom", Codes: []string{CodeNoHeadroomCPU, CodeNoHeadroomMemory}, Status: CheckSkipped, Fact: "single node",
	}))

	// Requests fitting N-1 capacity pass.
	snap = evaluate(
		readyNode("node-1", func(n *NodeDetail) { n.Requests = NodeResources{CPU: 5, Memory: 500} }),
		readyNode("node-2", func(n *NodeDetail) { n.Requests = NodeResources{CPU: 5, Memory: 500} }))
	g.Expect(findCheck(snap, "N-1 headroom").Status).To(Equal(CheckPassed))

	// Cordoned and down nodes give no capacity, but their requests
	// need a new home; placeholder pods are left out.
	snap = evaluate(
		readyNode("node-1", func(n *NodeDetail) {
			n.Requests = NodeResources{CPU: 9, Memory: 100}
			n.placeholderRequests = NodeResources{CPU: 5}
		}),
		readyNode("node-2", func(n *NodeDetail) { n.Requests = NodeResources{CPU: 2, Memory: 100} }),
		readyNode("node-3", cordoned, func(n *NodeDetail) { n.Requests = NodeResources{CPU: 2, Memory: 900} }),
		readyNode("node-4", withCondition(corev1.NodeReady, corev1.ConditionFalse),
			func(n *NodeDetail) { n.Requests = NodeResources{CPU: 1} }),
		readyNode("cp", func(n *NodeDetail) {
			n.ControlPlane = true
			n.Requests = NodeResources{CPU: 10, Memory: 1000}
		}))
	// CPU capacity: 20 - 10 = 10, requests: (9 - 5) + 2 + 2 + 1 = 9, fits
	// only because the placeholder pods are left out.
	g.Expect(findLine(snap, CodeNoHeadroomCPU)).To(BeNil())
	// Memory capacity: 2000 - 1000 = 1000, requests: 100 + 100 + 900 = 1100.
	memory := findLine(snap, CodeNoHeadroomMemory)
	g.Expect(memory).NotTo(BeNil())
	g.Expect(memory.Severity).To(Equal(SeverityWarn))
	g.Expect(*memory.ShortBy).To(BeNumerically("~", 100, 1e-9))
	g.Expect(findCheck(snap, "N-1 headroom").Status).To(Equal(CheckRaised))
}

func TestNodeFindings_MemoryOvercommit(t *testing.T) {
	g := NewWithT(t)

	// Limits above allocatable without bursting and below 150%
	// raise nothing, and the passed fact stays true.
	snap := evaluate(readyNode("node-1", func(n *NodeDetail) { n.Limits.Memory = 1400 }))
	g.Expect(findLine(snap, CodeMemoryOvercommit)).To(BeNil())
	g.Expect(findCheck(snap, "Memory overcommitment").Fact).To(Equal("memory limits below 150% of allocatable"))

	snap = evaluate(readyNode("node-1", func(n *NodeDetail) { n.Limits.Memory = 1000 }))
	g.Expect(findCheck(snap, "Memory overcommitment").Fact).To(Equal("memory limits within allocatable"))

	// Limits at 150% or more raise an advisory.
	high := readyNode("node-1", func(n *NodeDetail) { n.Limits.Memory = 1600 })
	low := readyNode("node-2", func(n *NodeDetail) { n.Limits.Memory = 1200 })
	snap = evaluate(low, high)
	line := findLine(snap, CodeMemoryOvercommit)
	g.Expect(line.Severity).To(Equal(SeverityInfo))
	g.Expect(line.Nodes).To(Equal(2))
	g.Expect(*line.Value).To(Equal(160))
	g.Expect(line.Affected).To(HaveLen(2))
	g.Expect(line.Affected[0].Node).To(Equal("node-1"))
	g.Expect(*line.Affected[0].Value).To(Equal(160))
	g.Expect(findNodeFinding(low, CodeMemoryOvercommit).Severity).To(Equal(SeverityInfo))

	// A bursting node (70% used, 10% above requests) raises a warning
	// and comes first in the affected list.
	bursting := readyNode("node-3", withUsage(1, 700), func(n *NodeDetail) {
		n.Limits.Memory = 1100
		n.AboveRequests = &NodeResources{Memory: 100}
	})
	notBursting := readyNode("node-4", withUsage(1, 690), func(n *NodeDetail) {
		n.Limits.Memory = 1100
		n.AboveRequests = &NodeResources{Memory: 100}
	})
	snap = evaluate(notBursting, bursting, readyNode("node-5", func(n *NodeDetail) { n.Limits.Memory = 1300 }))
	line = findLine(snap, CodeMemoryOvercommit)
	g.Expect(line.Severity).To(Equal(SeverityWarn))
	g.Expect(line.Nodes).To(Equal(3))
	g.Expect(line.Affected[0].Node).To(Equal("node-3"))
	g.Expect(line.Affected[0].Severity).To(Equal(SeverityWarn))
	g.Expect(line.Affected[1].Node).To(Equal("node-5"))
	g.Expect(findNodeFinding(bursting, CodeMemoryOvercommit).Severity).To(Equal(SeverityWarn))
	g.Expect(findNodeFinding(notBursting, CodeMemoryOvercommit).Severity).To(Equal(SeverityInfo))
	g.Expect(snap.Findings[0].Code).To(Equal(CodeMemoryOvercommit))
}

func TestNodeFindings_OvercommitBoundaries(t *testing.T) {
	tests := []struct {
		limits int64
		raised bool
		fact   string
	}{
		{limits: 1000, fact: "memory limits within allocatable"},
		{limits: 1010, fact: "memory limits below 150% of allocatable"},
		{limits: 1490, fact: "memory limits below 150% of allocatable"},
		{limits: 1500, raised: true},
	}
	for _, tt := range tests {
		g := NewWithT(t)
		snap := evaluate(readyNode("node-1", func(n *NodeDetail) { n.Limits.Memory = tt.limits }))
		line := findLine(snap, CodeMemoryOvercommit)
		c := findCheck(snap, "Memory overcommitment")
		if !tt.raised {
			g.Expect(line).To(BeNil(), "limits %d", tt.limits)
			g.Expect(c.Status).To(Equal(CheckPassed))
			g.Expect(c.Fact).To(Equal(tt.fact))
			continue
		}
		g.Expect(line.Severity).To(Equal(SeverityInfo))
		g.Expect(*line.Value).To(Equal(150))
		g.Expect(line.Nodes).To(Equal(1))
		g.Expect(c.Status).To(Equal(CheckRaised))
	}
}

func TestNodeFindings_MemoryBounds(t *testing.T) {
	g := NewWithT(t)

	snap := evaluate(readyNode("node-1"))
	g.Expect(findCheck(snap, "Memory requests").Status).To(Equal(CheckPassed))
	g.Expect(findCheck(snap, "Memory limits").Fact).To(Equal("all pods set a memory limit"))

	// Pods without request only: the limits check passes with a true fact.
	// The line is cluster-wide only: no nodes, no affected and no node
	// findings, so it never changes the node status pills.
	noRequest := readyNode("node-1", func(n *NodeDetail) { n.PodsWithoutMemoryRequest = 2 })
	snap = &NodesSnapshot{
		MetricsAvailable:    true,
		ControlPlaneVersion: "v1.33.4",
		Nodes:               []*NodeDetail{noRequest, readyNode("node-2")},
		UnboundedNamespaces: UnboundedNamespaces{WithoutRequest: []NamespacePods{{Namespace: "apps", Pods: 2}}},
	}
	evaluateNodes(snap)
	line := findLine(snap, CodeNoMemoryRequest)
	g.Expect(line.Severity).To(Equal(SeverityInfo))
	g.Expect(newNodesSummary(snap).Findings.Warn).To(BeZero())
	g.Expect(*line.Value).To(Equal(2))
	g.Expect(line.Nodes).To(BeZero())
	g.Expect(line.Affected).To(BeEmpty())
	g.Expect(line.Namespaces).To(Equal([]NamespacePods{{Namespace: "apps", Pods: 2}}))
	g.Expect(findNodeFinding(noRequest, CodeNoMemoryRequest)).To(BeNil())
	g.Expect(noRequest.PodsWithoutMemoryRequest).To(Equal(2))
	b, err := json.Marshal(line)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).NotTo(ContainSubstring(`"nodes"`))
	g.Expect(string(b)).NotTo(ContainSubstring(`"affected"`))
	g.Expect(findCheck(snap, "Memory limits")).To(Equal(NodeCheck{
		Name: "Memory limits", Codes: []string{CodeNoMemoryLimit}, Status: CheckPassed, Fact: "all pods with a request set a limit",
	}))

	snap = evaluate(readyNode("node-1", func(n *NodeDetail) { n.PodsWithoutMemoryLimit = 3 }))
	line = findLine(snap, CodeNoMemoryLimit)
	g.Expect(line.Severity).To(Equal(SeverityInfo))
	g.Expect(*line.Value).To(Equal(3))
	g.Expect(line.Affected).To(BeEmpty())
	g.Expect(findCheck(snap, "Memory limits").Status).To(Equal(CheckRaised))
}

func TestNodeFindings_UnschedulablePods(t *testing.T) {
	g := NewWithT(t)

	snap := &NodesSnapshot{
		MetricsAvailable:     true,
		ControlPlaneVersion:  "v1.33.4",
		UnschedulablePods:    4,
		UnschedulableReasons: []UnschedulableReason{{Reason: "Insufficient memory", Pods: 3}, {Reason: "untolerated taint", Pods: 1}},
		Nodes:                []*NodeDetail{readyNode("node-1")},
	}
	evaluateNodes(snap)
	line := findLine(snap, CodeUnschedulablePods)
	g.Expect(line.Severity).To(Equal(SeverityWarn))
	g.Expect(*line.Value).To(Equal(4))
	g.Expect(line.Nodes).To(BeZero())
	g.Expect(line.Reasons).To(HaveLen(2))
}

func TestNodeFindings_KubeletSkew(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		severity string
		affected []string
	}{
		{name: "single version", versions: []string{"v1.33.4", "v1.33.4+abc"}, severity: ""},
		{name: "supported skew", versions: []string{"v1.33.4", "v1.30.1"}, severity: SeverityInfo},
		{name: "too old", versions: []string{"v1.33.4", "v1.29.4"}, severity: SeverityWarn, affected: []string{"node-1"}},
		{name: "newer than server", versions: []string{"v1.34.0"}, severity: SeverityWarn, affected: []string{"node-0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			var nodes []*NodeDetail
			for i, v := range tt.versions {
				nodes = append(nodes, readyNode("node-"+string(rune('0'+i)), func(n *NodeDetail) { n.Info.KubeletVersion = v }))
			}
			snap := evaluate(nodes...)
			line := findLine(snap, CodeKubeletSkew)
			if tt.severity == "" {
				g.Expect(line).To(BeNil())
				g.Expect(findCheck(snap, "Kubelet versions").Fact).To(Equal("v1.33.4"))
				return
			}
			g.Expect(line.Severity).To(Equal(tt.severity))
			g.Expect(*line.Value).To(Equal(len(line.Versions)))
			var affected []string
			for _, a := range line.Affected {
				affected = append(affected, a.Node)
			}
			g.Expect(affected).To(Equal(tt.affected))
			for _, n := range nodes {
				f := findNodeFinding(n, CodeKubeletSkew)
				g.Expect(f != nil).To(Equal(len(tt.affected) > 0 && n.Name == tt.affected[0]))
			}
		})
	}
}

func TestNodeFindings_ChecksCatalogue(t *testing.T) {
	g := NewWithT(t)

	snap := evaluate(readyNode("node-1", withUsage(1, 100)), readyNode("node-2", withUsage(1, 100)))
	g.Expect(snap.Findings).To(BeEmpty())
	g.Expect(snap.Checks).To(HaveLen(20))
	for _, c := range snap.Checks {
		g.Expect(c.Status).To(Equal(CheckPassed), c.Name)
		g.Expect(c.Fact).NotTo(BeEmpty(), c.Name)
	}
	g.Expect(snap.Checks[0]).To(Equal(NodeCheck{
		Name: "Node readiness", Codes: []string{CodeUnreachable, CodeNotReady}, Status: CheckPassed, Fact: "2 nodes Ready",
	}))
}

func TestNodesSummary_TopLines(t *testing.T) {
	g := NewWithT(t)

	snap := evaluate(
		readyNode("node-1", withCondition(corev1.NodeReady, corev1.ConditionUnknown)),
		readyNode("node-2", withCondition(corev1.NodeDiskPressure, corev1.ConditionTrue)),
		readyNode("node-3", withUsage(9, 950)),
		readyNode("node-4", func(n *NodeDetail) { n.OOMKills = 1 }),
		readyNode("node-5", cordoned),
	)
	s := snap.summary
	g.Expect(s.Findings.Critical).To(Equal(3))
	g.Expect(s.Findings.Warn).To(Equal(2))
	g.Expect(s.Findings.Info).To(Equal(1))
	g.Expect(s.Findings.Top).To(HaveLen(3))
	g.Expect(s.Findings.Top[0].Code).To(Equal(CodeUnreachable))
	g.Expect(s.Findings.Top[0].Since).NotTo(BeNil())
	g.Expect(s.Findings.Top[1].Code).To(Equal(CodeDiskPressure))
	g.Expect(s.Findings.Top[2].Code).To(Equal(CodeMemoryHigh))
	g.Expect(*s.Findings.Top[2].Value).To(Equal(95))
	g.Expect(s.Down).To(Equal(1))
	g.Expect(s.Pressure).To(Equal(1))
	g.Expect(s.Cordoned).To(Equal(1))

	// The summary carries no node-sourced strings.
	b, err := json.Marshal(s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).NotTo(ContainSubstring("node-"))
	g.Expect(string(b)).NotTo(ContainSubstring("test message"))
}
