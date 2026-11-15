// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Wording and formatting for the Cluster Nodes summary panel and the Nodes
// dashboard. The backend owns the rules (statuses, findings, checks, every
// number); this module only turns its codes and numbers into text.

import { formatCores, formatBytes, percentOf } from './metrics'

const SEVERITY_RANK = { critical: 0, warn: 1, info: 2 }

/**
 * Rank a finding severity, worst first; unknown severities sort last.
 */
export function severityRank(severity) {
  return SEVERITY_RANK[severity] ?? 3
}

/**
 * Pluralize a count: plural(1, 'node') is "1 node", plural(2, 'node') "2 nodes".
 */
export function plural(n, word) {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

/**
 * Format the age of a timestamp: 45s, 12m, 3h, 6d. Returns null when the
 * timestamp is missing or invalid.
 */
export function formatAge(iso) {
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return null
  const s = Math.max(0, Math.floor((Date.now() - t) / 1000))
  if (s < 60) return `${s}s`
  if (s < 3600) return `${Math.floor(s / 60)}m`
  if (s < 86400) return `${Math.floor(s / 3600)}h`
  return `${Math.floor(s / 86400)}d`
}

/**
 * Format CPU cores with a unit: "500m" below one core, "2.40 cores" above.
 */
export function formatCpu(cores) {
  const s = formatCores(cores)
  return s.endsWith('m') ? s : `${s} cores`
}

/**
 * Usage severity against allocatable, for coloring: memory is critical at
 * 90%, CPU at 95% (it throttles, it doesn't kill), both warn at 80%. These
 * match the backend thresholds of the memory-high and cpu-high findings.
 */
export function usageSeverity(kind, percent) {
  if (typeof percent !== 'number') return null
  if (percent >= (kind === 'cpu' ? 95 : 90)) return 'critical'
  if (percent >= 80) return 'warn'
  return null
}

/**
 * The DNS suffix shared by all node names (".ec2.internal"), or an empty
 * string, so the lists show the distinguishing part only.
 *
 * @param {Array<string>} names - Node names
 */
export function commonNodeSuffix(names) {
  if (names.length < 2) return ''
  const parts = names.map(n => n.split('.'))
  if (parts.some(p => p.length < 2)) return ''
  const suffix = []
  for (let i = 1; ; i++) {
    const seg = parts[0][parts[0].length - i]
    // Keep at least the first label of every name.
    if (seg === undefined || parts.some(p => p.length - i < 1 || p[p.length - i] !== seg)) break
    suffix.unshift(seg)
  }
  return suffix.length ? '.' + suffix.join('.') : ''
}

/**
 * Build a shortName function stripping the given suffix.
 */
export function shortNamer(suffix) {
  return (name) => (suffix && name.endsWith(suffix) ? name.slice(0, -suffix.length) : name)
}

// Names listed per line before the rest is counted.
const NAME_LIST_LIMIT = 5

/**
 * Join labels capped at NAME_LIST_LIMIT, followed by "+N more".
 */
export function cappedList(labels) {
  const names = labels.slice(0, NAME_LIST_LIMIT).join(', ')
  const rest = labels.length - NAME_LIST_LIMIT
  return rest > 0 ? `${names} +${rest} more` : names
}

// Name-less wording per finding code. Lines carry only numbers: nodes (how
// many distinct nodes a line covers), value (a count or percentage) and since
// (a timestamp, worded relative to now so it never goes stale in the cache).
const SUMMARY_WORDING = {
  'unreachable': (l) => {
    const age = l.nodes === 1 ? formatAge(l.since) : null
    return age ? `1 node unreachable for ${age}` : `${plural(l.nodes, 'node')} unreachable`
  },
  'not-ready': (l) => `${plural(l.nodes, 'node')} not ready`,
  'memory-pressure': (l) => `${plural(l.nodes, 'node')} under memory pressure`,
  'disk-pressure': (l) => `${plural(l.nodes, 'node')} under disk pressure`,
  'pid-pressure': (l) => `${plural(l.nodes, 'node')} under PID pressure`,
  'network-unavailable': (l) => `${plural(l.nodes, 'node')} with network unavailable`,
  'memory-overcommit': (l) => `${plural(l.nodes ?? l.value, 'node')} with overcommitted memory`,
  'node-condition': (l) => `${plural(l.nodes, 'node')} with failing conditions`,
  'memory-high': (l) => `${plural(l.nodes, 'node')} with high memory`,
  'cpu-high': (l) => `${plural(l.nodes, 'node')} with high CPU`,
  'oom-kills': (l) => `${plural(l.nodes, 'node')} with OOM kills`,
  'no-memory-request': (l) => `${plural(l.value, 'pod')} without memory request`,
  'unschedulable-pods': (l) => `${plural(l.value, 'pod')} unschedulable`,
  'no-headroom-memory': () => 'No memory headroom for node loss',
  'no-headroom-cpu': () => 'No CPU headroom for node loss',
  'pod-capacity': (l) => `${plural(l.nodes, 'node')} near pod capacity`,
  'heartbeat-lag': (l) => `${plural(l.nodes, 'node')} with late heartbeat`,
  'flapping': (l) => `${plural(l.nodes, 'node')} flapping`,
  'cordoned': (l) => `${plural(l.nodes, 'node')} cordoned`,
  'requests-full': (l) => `${plural(l.nodes, 'node')} fully requested`,
  'no-memory-limit': (l) => `${plural(l.value, 'pod')} without memory limit`,
  'kubelet-skew': (l) => `${plural(l.value, 'kubelet version')} in use`
}

/**
 * Word a finding line ({code, nodes, value, since}), used by both the
 * summary panel and the Health Checks list. Unknown codes get a generic
 * wording, so a newer backend never breaks an older UI.
 */
export function summaryTitle(line) {
  const word = SUMMARY_WORDING[line.code]
  if (word) return word(line)
  return typeof line.nodes === 'number' && line.nodes > 0 ? `${plural(line.nodes, 'node')} affected` : 'Cluster issue'
}

// Codes whose detail never lists node names.
const NODE_LESS_DETAIL = new Set(['kubelet-skew'])

/**
 * Detail of a Health Checks line: the affected node names worst first,
 * capped at five (with the condition type for failing conditions), then the
 * per-code extras (scheduler reasons, namespaces with pod counts, kubelet
 * versions, N-1 shortfall).
 *
 * @param {Object} line - Grouped finding line from GET /api/v1/nodes
 * @param {Object} data - The snapshot, for the cluster-wide extras
 * @param {Function} shortName - Strips the shared DNS suffix from node names
 */
export function lineDetail(line, data = {}, shortName = (n) => n) {
  const parts = []
  const affected = [...(line.affected || [])].sort((a, b) =>
    severityRank(a.severity) - severityRank(b.severity) || (b.value ?? 0) - (a.value ?? 0))
  // Kubelet skew is attributed to nodes but detailed by its versions; the
  // nodes show the finding in their own rows.
  if (affected.length > 0 && !NODE_LESS_DETAIL.has(line.code)) {
    parts.push(cappedList(affected.map(a => a.conditionType ? `${shortName(a.node)} ${a.conditionType}` : shortName(a.node))))
  }
  const reasons = line.reasons ?? (line.code === 'unschedulable-pods' ? data.unschedulableReasons : null)
  if (reasons?.length > 0) {
    parts.push(cappedList(reasons.map(r => `${r.reason} (${r.pods})`)))
  }
  const namespaces = line.namespaces ?? (line.code === 'no-memory-request'
    ? data.unboundedNamespaces?.withoutRequest
    : line.code === 'no-memory-limit' ? data.unboundedNamespaces?.withoutLimit : null)
  if (namespaces?.length > 0) {
    parts.push(cappedList([...namespaces].sort((a, b) => b.pods - a.pods).map(n => `${n.namespace} (${n.pods})`)))
  }
  if (line.versions?.length > 0) {
    parts.push(`${line.versions.join(', ')}${data.controlPlaneVersion ? ` (API server ${data.controlPlaneVersion})` : ''}`)
  }
  if (typeof line.shortBy === 'number') {
    const format = line.code === 'no-headroom-cpu' ? formatCpu : formatBytes
    parts.push(`requests ${format(line.shortBy)} over N-1 capacity`)
  }
  return parts.join(' · ')
}

// Pill labels for a node's worst finding, per code. Pills carry no figures,
// except pod slots.
const BADGE_LABELS = {
  'network-unavailable': () => 'Network unavailable',
  'memory-overcommit': () => 'Overcommitted',
  'node-condition': (f) => f.conditionType,
  'memory-high': () => 'Memory high',
  'cpu-high': () => 'CPU high',
  'oom-kills': () => 'OOM kills',
  'pod-capacity': (f) => (typeof f.value === 'number' ? `Pods ${f.value}%` : 'Pods full'),
  'heartbeat-lag': () => 'Heartbeat late',
  'flapping': () => 'Flapping',
  'requests-full': () => 'Requests full',
  'kubelet-skew': () => 'Kubelet skew'
}

const NEUTRAL_PILL = 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400'

// Kubernetes states shown on the pill as is.
const K8S_STATES = ['Unreachable', 'NotReady', 'Pressure', 'Cordoned']

/**
 * Status pill of a node: the Kubernetes state when the node is down, under
 * pressure or cordoned, else its worst critical/warn finding, else Ready.
 * A cordoned node keeps its Kubernetes label (Unreachable, the pressure
 * types or Cordoned) but is yellow, as its findings are advisories. A node
 * without a Ready condition reads Unknown in grey.
 *
 * @param {Object} node - Node from GET /api/v1/nodes (status, pressures,
 *   unschedulable and findings sorted worst first)
 * @returns {{label: string, class: string, severity: string}} severity is
 *   critical, warn, unknown or ok, the value the node list filters on
 */
export function nodeBadge(node) {
  const status = node.status
  if (K8S_STATES.includes(status)) {
    const label = status === 'Pressure' && node.pressures?.length > 0 ? node.pressures.join(', ') : status
    const warn = status === 'Cordoned' || node.unschedulable
    return { label, class: warn ? 'status-warning' : 'status-not-ready', severity: warn ? 'warn' : 'critical' }
  }
  const worst = (node.findings || [])
    .filter(f => f.severity === 'critical' || f.severity === 'warn')
    .sort((a, b) => severityRank(a.severity) - severityRank(b.severity))[0]
  if (worst) {
    const label = BADGE_LABELS[worst.code]?.(worst) || (worst.severity === 'critical' ? 'Critical' : 'Warning')
    return { label, class: worst.severity === 'critical' ? 'status-not-ready' : 'status-warning', severity: worst.severity }
  }
  if (status !== 'Ready') {
    return { label: status || 'Unknown', class: NEUTRAL_PILL, severity: 'unknown' }
  }
  return { label: 'Ready', class: 'status-ready', severity: 'ok' }
}

const PRESSURE_LABELS = { 'memory-pressure': 'Memory pressure', 'disk-pressure': 'Disk pressure', 'pid-pressure': 'PID pressure' }

/**
 * Title and detail of one of a node's findings for its expanded row. Raw
 * kubelet and node-problem-detector messages go in the detail.
 *
 * @param {Object} f - Node finding
 * @param {Object} node - The node, for its pod slots, limits and versions
 * @param {string} [controlPlaneVersion] - API server version
 * @returns {{title: string, detail: string}}
 */
export function nodeIssue(f, node, controlPlaneVersion) {
  const age = formatAge(f.since)
  const raw = f.message || f.reason || ''
  const join = (...parts) => parts.filter(Boolean).join(' · ')
  switch (f.code) {
  case 'unreachable':
    return { title: 'Unreachable', detail: join(age && `for ${age}`, raw) }
  case 'not-ready':
    return { title: 'Not ready', detail: join(age && `for ${age}`, raw) }
  case 'memory-pressure':
  case 'disk-pressure':
  case 'pid-pressure':
    return { title: PRESSURE_LABELS[f.code], detail: join(age && `for ${age}`, raw) }
  case 'network-unavailable':
    return { title: 'Network unavailable', detail: join(age && `for ${age}`, raw) }
  case 'node-condition':
    return { title: f.conditionType || 'Failing condition', detail: join(age && `for ${age}`, raw) }
  case 'memory-high':
    return { title: `Memory ${f.value}%`, detail: '' }
  case 'cpu-high':
    return { title: `CPU ${f.value}%`, detail: '' }
  case 'oom-kills':
    return { title: plural(f.value, 'OOM kill'), detail: 'last hour' }
  case 'pod-capacity':
    return { title: `Pods ${f.value}%`, detail: `${node.pods} / ${node.allocatable?.pods}` }
  case 'heartbeat-lag':
    return { title: `Heartbeat ${f.value}s ago`, detail: '' }
  case 'flapping':
    return { title: 'Flapping', detail: `${f.value} Ready transitions in 15m` }
  case 'requests-full':
    return { title: `Requests ${f.value}%`, detail: '' }
  case 'cordoned':
    return { title: 'Cordoned', detail: join(age && `for ${age}`, raw) }
  case 'memory-overcommit': {
    const limits = percentOf(node.limits?.memory, node.allocatable?.memory) ?? f.value
    const above = node.aboveRequests?.memory
    return {
      title: 'Memory overcommitted',
      detail: join(limits != null && `Limits ${limits}% of allocatable`, above > 0 && `${formatBytes(above)} used above requests`)
    }
  }
  case 'kubelet-skew':
    return { title: `Kubelet ${node.info?.kubeletVersion}`, detail: controlPlaneVersion ? `API server ${controlPlaneVersion}` : '' }
  default:
    return { title: f.conditionType || f.code, detail: raw }
  }
}
