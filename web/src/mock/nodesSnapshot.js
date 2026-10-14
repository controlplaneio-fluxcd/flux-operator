// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock-only builder of the GET /api/v1/nodes snapshot and the report's
// spec.nodes summary from raw node data. The backend owns these rules
// (internal/web/nodes_findings.go); this is a simplified copy so the dev
// server can serve realistic, internally consistent data. Never import it
// from application code.

const PRESSURE_CODES = { MemoryPressure: 'memory-pressure', DiskPressure: 'disk-pressure', PIDPressure: 'pid-pressure' }
const STANDARD_CONDITIONS = ['Ready', 'MemoryPressure', 'DiskPressure', 'PIDPressure', 'NetworkUnavailable']
const CONTROL_PLANE_TAINTS = ['node-role.kubernetes.io/control-plane', 'node-role.kubernetes.io/master']
const SEVERITY_RANK = { critical: 0, warn: 1, info: 2 }
const CODE_PRIORITY = [
  'unreachable', 'not-ready', 'memory-pressure', 'disk-pressure', 'pid-pressure',
  'network-unavailable', 'memory-overcommit', 'node-condition', 'memory-high', 'cpu-high',
  'oom-kills', 'no-memory-request', 'unschedulable-pods', 'no-headroom-memory', 'no-headroom-cpu',
  'pod-capacity', 'heartbeat-lag', 'flapping', 'cordoned', 'requests-full',
  'no-memory-limit', 'kubelet-skew'
]
// Codes computed cluster-wide rather than per node.
const CLUSTER_CODES = ['unschedulable-pods', 'no-headroom-cpu', 'no-headroom-memory', 'memory-overcommit',
  'no-memory-request', 'no-memory-limit', 'kubelet-skew']

const rank = (code) => {
  const i = CODE_PRIORITY.indexOf(code)
  return i === -1 ? CODE_PRIORITY.length : i
}
const byWorst = (a, b) => SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] || rank(a.code) - rank(b.code)
const percent = (v, total) => (typeof v === 'number' && total > 0 ? Math.floor((v / total) * 100 + 1e-9) : null)
const condition = (n, type) => (n.conditions || []).find(c => c.type === type)
const minor = (v) => Number((v || '').replace(/^v/, '').split('.')[1])
const baseVersion = (v) => (v || '').split('+')[0]
// Ready=True and schedulable, whatever the pressure.
const schedulableReady = (n) => condition(n, 'Ready')?.status === 'True' && !n.unschedulable

function nodeStatus(n) {
  const ready = condition(n, 'Ready')
  // A node without a Ready condition is NotReady once past the registration grace.
  if (!ready) return Date.now() - Date.parse(n.createdAt) > 2 * 60 * 1000 ? 'NotReady' : 'Unknown'
  if (ready.status === 'Unknown') return 'Unreachable'
  if (ready.status !== 'True') return 'NotReady'
  if (n.pressures.length > 0) return 'Pressure'
  if (n.unschedulable) return 'Cordoned'
  return 'Ready'
}

// Per-node rules.
function nodeFindings(n) {
  const out = []
  const health = (severity) => (n.unschedulable ? 'info' : severity)
  const ready = condition(n, 'Ready')
  const readyTrue = ready?.status === 'True'
  if (n.status === 'Unreachable') out.push({ code: 'unreachable', severity: health('critical'), since: ready.lastTransitionTime, reason: ready.reason, message: ready.message })
  if (n.status === 'NotReady') {
    out.push(ready
      ? { code: 'not-ready', severity: health('critical'), since: ready.lastTransitionTime, reason: ready.reason, message: ready.message }
      : { code: 'not-ready', severity: health('critical'), reason: 'NoReadyCondition' })
  }
  for (const c of n.conditions) {
    if (c.status !== 'True') continue
    if (PRESSURE_CODES[c.type]) {
      out.push({ code: PRESSURE_CODES[c.type], severity: health('critical'), conditionType: c.type, since: c.lastTransitionTime, reason: c.reason, message: c.message })
    } else if (c.type === 'NetworkUnavailable') {
      out.push({ code: 'network-unavailable', severity: health('critical'), since: c.lastTransitionTime, reason: c.reason, message: c.message })
    } else if (!STANDARD_CONDITIONS.includes(c.type)) {
      out.push({ code: 'node-condition', severity: health('warn'), conditionType: c.type, since: c.lastTransitionTime, reason: c.reason, message: c.message })
    }
  }
  if (n.status !== 'Unreachable' && n.heartbeatSeconds > 20) {
    out.push({ code: 'heartbeat-lag', severity: health(n.heartbeatSeconds > 40 ? 'critical' : 'warn'), value: n.heartbeatSeconds })
  }
  if (n.readyTransitions > 2) out.push({ code: 'flapping', severity: health('warn'), value: n.readyTransitions })
  // Usage rules skip cordoned nodes, which are expected to go away.
  if (readyTrue && !n.unschedulable && n.usage) {
    const mem = percent(n.usage.memory, n.allocatable.memory)
    if (mem >= 80) out.push({ code: 'memory-high', severity: mem >= 90 ? 'critical' : 'warn', value: mem })
    const cpu = percent(n.usage.cpu, n.allocatable.cpu)
    if (cpu >= 80) out.push({ code: 'cpu-high', severity: cpu >= 95 ? 'critical' : 'warn', value: cpu })
  }
  if (n.oomKills > 0) out.push({ code: 'oom-kills', severity: 'warn', value: n.oomKills })
  const pods = percent(n.pods, n.allocatable.pods)
  if (pods >= 90) out.push({ code: 'pod-capacity', severity: 'warn', value: pods })
  const requested = Math.max(percent(n.requests.cpu, n.allocatable.cpu), percent(n.requests.memory, n.allocatable.memory))
  if (readyTrue && !n.unschedulable && requested >= 90) out.push({ code: 'requests-full', severity: 'warn', value: requested })
  if (n.unschedulable) {
    const taint = n.taints.find(t => t.key === 'node.kubernetes.io/unschedulable')
    out.push({ code: 'cordoned', severity: 'info', since: taint?.timeAdded })
  }
  return out
}

// Group per-node findings with the same code into one line.
function groupNodeFindings(nodes) {
  const lines = new Map()
  for (const n of nodes) {
    for (const f of n.findings) {
      if (CLUSTER_CODES.includes(f.code)) continue
      if (!lines.has(f.code)) lines.set(f.code, { code: f.code, severity: f.severity, value: 0, affected: [] })
      const line = lines.get(f.code)
      if (SEVERITY_RANK[f.severity] < SEVERITY_RANK[line.severity]) line.severity = f.severity
      if (typeof f.value === 'number') line.value = f.code === 'oom-kills' ? line.value + f.value : Math.max(line.value, f.value)
      line.affected.push({ node: n.name, severity: f.severity, value: f.value, conditionType: f.code === 'node-condition' ? f.conditionType : undefined })
      if (f.since) line.since = f.since
    }
  }
  return [...lines.values()].map(l => {
    const nodes = new Set(l.affected.map(a => a.node)).size
    return { ...l, nodes, since: nodes === 1 ? l.since : undefined, value: l.value || undefined }
  })
}

// Cluster-wide rules; lines with node attribution also land on their nodes.
function clusterLines(raw, nodes) {
  const lines = []
  const attach = (line, affected) => {
    line.affected = affected
    line.nodes = affected.length
    for (const a of affected) nodes.find(n => n.name === a.node).findings.push({ code: line.code, severity: a.severity, value: a.value })
  }
  if (raw.unschedulablePods > 0) {
    lines.push({ code: 'unschedulable-pods', severity: 'warn', value: raw.unschedulablePods, reasons: raw.unschedulableReasons })
  }
  // N-1: capacity of Ready=True, schedulable workload nodes (pressured
  // nodes included) against the requests of every workload node.
  const workload = nodes.filter(n => !n.controlPlane)
  const capacity = workload.filter(schedulableReady)
  if (capacity.length > 1) {
    for (const k of ['memory', 'cpu']) {
      const alloc = capacity.reduce((s, n) => s + n.allocatable[k], 0)
      const largest = Math.max(...capacity.map(n => n.allocatable[k]))
      const requests = workload.reduce((s, n) => s + n.requests[k], 0)
      const shortBy = requests - (alloc - largest)
      if (shortBy > 0) lines.push({ code: `no-headroom-${k}`, severity: 'warn', shortBy })
    }
  }
  const over = nodes.filter(n => condition(n, 'Ready')?.status === 'True' && !n.unschedulable && percent(n.limits.memory, n.allocatable.memory) > 100)
  if (over.length > 0) {
    const bursting = (n) => percent(n.usage?.memory, n.allocatable.memory) >= 70 && percent(n.aboveRequests?.memory, n.allocatable.memory) >= 10
    const hi = Math.max(...over.map(n => percent(n.limits.memory, n.allocatable.memory)))
    if (over.some(bursting) || hi >= 150) {
      const line = { code: 'memory-overcommit', severity: over.some(bursting) ? 'warn' : 'info' }
      attach(line, over.map(n => ({ node: n.name, severity: bursting(n) ? 'warn' : 'info', value: percent(n.limits.memory, n.allocatable.memory) })))
      line.value = hi
      lines.push(line)
    }
  }
  const noRequest = nodes.filter(n => n.podsWithoutMemoryRequest > 0)
  if (noRequest.length > 0) {
    // Cluster-wide only: never attributed to nodes.
    lines.push({
      code: 'no-memory-request', severity: 'info',
      value: noRequest.reduce((s, n) => s + n.podsWithoutMemoryRequest, 0),
      namespaces: raw.unboundedNamespaces.withoutRequest
    })
  }
  const noLimit = nodes.reduce((s, n) => s + n.podsWithoutMemoryLimit, 0)
  if (noLimit > 0) lines.push({ code: 'no-memory-limit', severity: 'info', value: noLimit, namespaces: raw.unboundedNamespaces.withoutLimit })
  const versions = [...new Set(nodes.map(n => baseVersion(n.info.kubeletVersion)))].sort().reverse()
  const server = minor(raw.controlPlaneVersion)
  const unsupported = nodes.filter(n => {
    const m = minor(baseVersion(n.info.kubeletVersion))
    return m > server || server - m > 3
  })
  if (versions.length > 1 || unsupported.length > 0) {
    const line = { code: 'kubelet-skew', severity: unsupported.length > 0 ? 'warn' : 'info', versions }
    attach(line, unsupported.map(n => ({ node: n.name, severity: 'warn' })))
    line.value = versions.length
    if (line.nodes === 0) delete line.nodes
    lines.push(line)
  }
  return lines
}

// Health check catalogue: name, codes, and the fact when skipped or passed.
const CHECKS = [
  { name: 'Node readiness', codes: ['unreachable', 'not-ready'], fact: (d) => `${d.nodes.length} nodes Ready` },
  { name: 'Memory pressure', codes: ['memory-pressure'], fact: () => 'none' },
  { name: 'Disk pressure', codes: ['disk-pressure'], fact: () => 'none' },
  { name: 'PID pressure', codes: ['pid-pressure'], fact: () => 'none' },
  { name: 'Network', codes: ['network-unavailable'], fact: () => 'available on all nodes' },
  { name: 'Node conditions', codes: ['node-condition'], fact: () => 'no failing conditions' },
  { name: 'Memory usage', codes: ['memory-high'], skip: (d) => !d.metricsAvailable && 'no metrics-server', fact: () => 'all nodes below 80%' },
  { name: 'CPU usage', codes: ['cpu-high'], skip: (d) => !d.metricsAvailable && 'no metrics-server', fact: () => 'all nodes below 80%' },
  {
    name: 'Memory overcommitment', codes: ['memory-overcommit'],
    fact: (d) => d.nodes.some(n => percent(n.limits.memory, n.allocatable.memory) > 100) ? 'memory limits below 150% of allocatable' : 'memory limits within allocatable'
  },
  { name: 'OOM kills', codes: ['oom-kills'], fact: () => 'none in the last hour' },
  { name: 'Memory requests', codes: ['no-memory-request'], fact: () => 'all pods set a memory request' },
  { name: 'Pod scheduling', codes: ['unschedulable-pods'], fact: () => 'no unschedulable pods' },
  {
    name: 'N-1 headroom', codes: ['no-headroom-cpu', 'no-headroom-memory'],
    skip: (d) => d.nodes.filter(n => !n.controlPlane && schedulableReady(n)).length < 2 && 'single node',
    fact: () => 'requests fit with one node down'
  },
  { name: 'Pod capacity', codes: ['pod-capacity'], fact: () => 'all nodes below 90% of pod slots' },
  { name: 'Heartbeat', codes: ['heartbeat-lag'], skip: (d) => d.nodes.every(n => n.heartbeatSeconds == null) && 'no node leases', fact: () => 'all leases renewed within 20s' },
  { name: 'Readiness flapping', codes: ['flapping'], skip: (d) => d.nodes.every(n => n.readyTransitions == null) && 'collecting history', fact: () => 'no flapping in the last 15m' },
  { name: 'Requests', codes: ['requests-full'], fact: () => 'all nodes below 90% requested' },
  { name: 'Cordoned nodes', codes: ['cordoned'], fact: () => 'none' },
  {
    name: 'Memory limits', codes: ['no-memory-limit'],
    fact: (d) => d.nodes.some(n => n.podsWithoutMemoryRequest > 0) ? 'all pods with a request set a limit' : 'all pods set a memory limit'
  },
  { name: 'Kubelet versions', codes: ['kubelet-skew'], fact: (d) => [...new Set(d.nodes.map(n => n.info.kubeletVersion))].join(', ') }
]

function checks(snapshot) {
  const raised = new Set(snapshot.findings.map(l => l.code))
  return CHECKS.map(c => {
    if (c.codes.some(code => raised.has(code))) return { name: c.name, codes: c.codes, status: 'raised' }
    const skipped = c.skip?.(snapshot)
    return skipped
      ? { name: c.name, codes: c.codes, status: 'skipped', fact: skipped }
      : { name: c.name, codes: c.codes, status: 'passed', fact: c.fact(snapshot) }
  })
}

function capacity(nodes, samples, metricsAvailable, podMetricsAvailable) {
  const eligible = nodes.filter(n => !n.controlPlane && ['Ready', 'Pressure', 'Cordoned'].includes(n.status))
  const none = eligible.length === 0
  const sum = (fn) => (none ? null : eligible.reduce((s, n) => s + fn(n), 0))
  const last = samples.length ? samples[samples.length - 1] : null
  const resource = (k) => ({
    allocatable: sum(n => n.allocatable[k]),
    requested: sum(n => n.requests[k]),
    used: metricsAvailable && last && !none ? last[k] : null,
    aboveRequests: podMetricsAvailable && !none ? sum(n => n.aboveRequests?.[k] || 0) : null,
    limits: sum(n => n.limits[k])
  })
  return {
    eligible,
    capacity: { cpu: resource('cpu'), memory: resource('memory'), pods: { allocatable: sum(n => n.allocatable.pods), running: sum(n => n.pods) } }
  }
}

/**
 * Build the GET /api/v1/nodes snapshot from raw nodes. Each raw node may
 * carry a `samples` array (30m usage history); its last sample is the node
 * usage, and the cluster series sums the eligible nodes per tick.
 */
export function buildSnapshot(raw) {
  const metricsAvailable = raw.metricsAvailable
  const podMetricsAvailable = raw.podMetricsAvailable ?? metricsAvailable
  const nodes = raw.nodes.map(r => {
    const { samples, ...n } = r
    const latest = metricsAvailable && samples?.length ? samples[samples.length - 1] : null
    const node = {
      ...n,
      pressures: n.conditions.filter(c => PRESSURE_CODES[c.type] && c.status === 'True').map(c => c.type),
      controlPlane: n.taints.some(t => CONTROL_PLANE_TAINTS.includes(t.key) && t.effect !== 'PreferNoSchedule'),
      usage: latest ? { cpu: latest.cpu, memory: latest.memory } : null,
      aboveRequests: podMetricsAvailable ? n.aboveRequests : null
    }
    node.status = nodeStatus(node)
    node.findings = nodeFindings(node)
    return { node, samples: latest ? samples : null }
  })
  const list = nodes.map(x => x.node)
  const eligibleNames = new Set(capacity(list, [], false, false).eligible.map(n => n.name))
  const metered = nodes.filter(x => x.samples && eligibleNames.has(x.node.name))
  const samples = metricsAvailable && metered.length
    ? metered[0].samples.map((s, i) => metered.reduce((acc, x) => {
      acc.cpu += x.samples[i].cpu
      acc.memory += x.samples[i].memory
      return acc
    }, { t: s.t, cpu: 0, memory: 0 }))
    : []

  const lines = [...groupNodeFindings(list), ...clusterLines(raw, list)].sort(byWorst)
  for (const n of list) n.findings.sort(byWorst)
  const snapshot = {
    metricsAvailable,
    podMetricsAvailable,
    controlPlaneVersion: raw.controlPlaneVersion,
    unschedulablePods: raw.unschedulablePods,
    unschedulableReasons: raw.unschedulableReasons,
    unboundedNamespaces: raw.unboundedNamespaces,
    capacity: capacity(list, samples, metricsAvailable, podMetricsAvailable).capacity,
    usage: { samples },
    nodes: list,
    findings: lines
  }
  snapshot.checks = checks(snapshot)
  return snapshot
}

/**
 * Build the name-less report summary (spec.nodes) from a snapshot: counts,
 * capacity totals and finding lines made of codes and numbers only.
 */
export function buildSummary(snapshot) {
  const nodes = snapshot.nodes
  const count = (sev) => snapshot.findings.filter(l => l.severity === sev).length
  return {
    down: nodes.filter(n => !n.unschedulable && (n.status === 'Unreachable' || n.status === 'NotReady')).length,
    pressure: nodes.filter(n => !n.unschedulable && n.status === 'Pressure').length,
    cordoned: nodes.filter(n => n.unschedulable).length,
    unschedulablePods: snapshot.unschedulablePods,
    metricsAvailable: snapshot.metricsAvailable,
    ...snapshot.capacity,
    findings: {
      critical: count('critical'),
      warn: count('warn'),
      info: count('info'),
      top: snapshot.findings.filter(l => l.severity !== 'info').slice(0, 3).map(l => ({
        code: l.code,
        severity: l.severity,
        ...(l.nodes ? { nodes: l.nodes } : {}),
        ...(typeof l.value === 'number' ? { value: l.value } : {}),
        ...(l.since ? { since: l.since } : {})
      }))
    }
  }
}
