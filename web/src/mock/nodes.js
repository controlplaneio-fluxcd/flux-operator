// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock data for the nodes endpoints (GET /api/v1/nodes, GET /api/v1/nodes/workloads)
// and the report's spec.nodes summary.
//
// URL switches (combinable, read at page load):
//   ?healthy    a cluster without findings
//   ?onprem     OpenShift on bare metal (masters with the control-plane taint, no zone/instance labels)
//   ?nometrics  no metrics-server
//   ?nopodmetrics  node metrics only (no usage above requests, no workloads ranking)
//   ?viewer     a user who cannot list nodes (no dashboard link, 403 from the API)
//   ?large      120 nodes

import { buildSnapshot, buildSummary } from './nodesSnapshot'

const GiB = 1024 * 1024 * 1024
const now = Date.now()
const ago = (ms) => new Date(now - ms).toISOString()
const min = 60 * 1000
const day = 24 * 60 * min

/**
 * Build a deterministic 30 minute history with one sample per minute that
 * ends exactly at the given usage, so the node usage is the last sample.
 */
function history(seed, cpu, memory) {
  return Array.from({ length: 31 }, (_, i) => {
    const d = 30 - i
    return {
      t: new Date(now - d * min).toISOString(),
      cpu: Math.max(0, cpu * (1 + 0.12 * Math.sin((i + seed) / 3) - 0.12 * Math.sin((30 + seed) / 3))),
      memory: Math.round(memory * (1 - 0.0008 * d + 0.01 * (Math.cos((i + seed) / 5) - Math.cos((30 + seed) / 5))))
    }
  })
}

const GENERIC_OS = { osImage: 'Ubuntu 24.04.1 LTS', kernelVersion: '6.8.0-45-generic', containerRuntimeVersion: 'containerd://2.0.6' }
const OPENSHIFT_OS = { osImage: 'Red Hat Enterprise Linux CoreOS 418.94.202509100030-0', kernelVersion: '5.14.0-427.84.1.el9_4.x86_64', containerRuntimeVersion: 'cri-o://1.31.11-2.rhaos4.18.git9b2f45c.el9' }

const ready = (since = 9 * day) => ({ type: 'Ready', status: 'True', reason: 'KubeletReady', message: 'kubelet is posting ready status', lastTransitionTime: ago(since) })
const ok = (type) => ({
  type, status: 'False', lastTransitionTime: ago(9 * day),
  reason: `KubeletHas${type === 'PIDPressure' ? 'SufficientPID' : type === 'DiskPressure' ? 'NoDiskPressure' : 'SufficientMemory'}`
})
const okConditions = () => [ready(), ok('MemoryPressure'), ok('DiskPressure'), ok('PIDPressure')]

function rawNode(p, seed) {
  const unreachable = p.conditions?.[0]?.status === 'Unknown'
  const heartbeat = unreachable ? 372 : (p.heartbeat ?? 8)
  return {
    name: p.name,
    pool: p.pool,
    zone: p.zone,
    instanceType: p.instanceType,
    roles: p.roles,
    createdAt: ago(p.age),
    unschedulable: !!p.unschedulable,
    taints: p.taints,
    conditions: p.conditions || okConditions(),
    info: { kubeletVersion: p.kubelet, ...p.os, architecture: p.pool === 'batch' ? 'arm64' : 'amd64' },
    allocatable: { cpu: p.cpuAlloc, memory: p.memAlloc, pods: p.podsAlloc },
    requests: { cpu: p.cpuReq, memory: p.memReq },
    limits: { cpu: p.cpuLim, memory: p.memLim },
    pods: p.pods,
    podsWithoutMemoryRequest: p.noMemRequest || 0,
    podsWithoutMemoryLimit: p.noMemLimit || 0,
    oomKills: p.oomKills || 0,
    // Sum over pods of max(0, usage - request).
    aboveRequests: { cpu: Math.max(0, p.cpu - p.cpuReq) * 0.6, memory: p.burstMem ?? Math.max(0, p.memory - p.memReq) * 0.6 },
    leaseRenewTime: ago(heartbeat * 1000),
    heartbeatSeconds: heartbeat,
    readyTransitions: p.readyTransitions ?? 0,
    samples: p.metrics === false ? null : history(seed, p.cpu, p.memory)
  }
}

// Node shapes per pool: [instance type, allocatable CPU cores, allocatable memory GiB, pod slots].
const SHAPES = {
  system: ['standard-4', 3.92, 14.5, 58],
  general: ['standard-8', 7.91, 29.3, 110],
  batch: ['compute-16', 15.89, 27.1, 110]
}
const ZONES = ['zone-a', 'zone-b', 'zone-c']
const ONPREM_SHAPES = { master: [7.5, 30, 250], infra: [15.5, 60, 250], worker: [31.5, 124, 250] }

const repeat = (pool, n) => Array.from({ length: n }, (_, i) => [pool, i])
const layout = (onprem, large) => onprem
  ? [...repeat('master', 3), ...repeat('infra', 3), ...repeat('worker', large ? 114 : 14)]
  : [...repeat('system', 3), ...repeat('general', large ? 100 : 13), ...repeat('batch', large ? 17 : 4)]

/**
 * Healthy node parameters: usage 30-55% of allocatable, requests 50-70%,
 * limits within allocatable, pod slots under half used.
 */
function healthy(pool, i, onprem, large) {
  const [instanceType, cpuAlloc, memGiB, podsAlloc] = onprem ? [undefined, ...ONPREM_SHAPES[pool]] : SHAPES[pool]
  const memAlloc = memGiB * GiB
  const f = ((i * 37) % 10) / 100
  return {
    name: onprem ? `${pool}-${i}.ocp.lab.example` : `${pool}-${String(i + 1).padStart(large ? 3 : 2, '0')}`,
    pool,
    zone: onprem ? undefined : ZONES[i % 3],
    instanceType,
    roles: onprem ? [pool] : [],
    age: (5 + i * 3) * day,
    kubelet: onprem ? 'v1.31.6+67d3387' : 'v1.33.4',
    skewKubelet: onprem ? 'v1.27.10+a1b2c3d' : 'v1.29.10',
    os: onprem ? OPENSHIFT_OS : GENERIC_OS,
    taints: pool === 'master' ? [{ key: 'node-role.kubernetes.io/master', effect: 'NoSchedule' }] : [],
    cpuAlloc, memAlloc, podsAlloc,
    cpu: cpuAlloc * (0.3 + f), memory: memAlloc * (0.4 + f),
    cpuReq: cpuAlloc * (0.5 + f), cpuLim: cpuAlloc * 0.95,
    memReq: memAlloc * (0.55 + f), memLim: memAlloc * 0.9,
    pods: Math.round(podsAlloc * (0.3 + f))
  }
}

const notReadyConditions = (status, reason, message, since) => [
  { type: 'Ready', status, reason, message, lastTransitionTime: ago(since) },
  ...['MemoryPressure', 'DiskPressure', 'PIDPressure'].map(type => status === 'Unknown'
    ? { type, status: 'Unknown', reason: 'NodeStatusUnknown', lastTransitionTime: ago(since) }
    : ok(type))
]
const pressure = (type, reason, message, since) => [ready(),
  ...['MemoryPressure', 'DiskPressure', 'PIDPressure'].map(t => t === type
    ? { type, status: 'True', reason, message, lastTransitionTime: ago(since) }
    : ok(t))]
const extraCondition = (c) => [...okConditions(), { ...c, status: 'True' }]

// One error condition per node, in layout order (first 20 nodes).
const ERRORS = [
  (n) => ({ ...n, kubelet: n.skewKubelet }),
  (n) => ({ ...n, conditions: pressure('DiskPressure', 'KubeletHasDiskPressure', 'kubelet has disk pressure', 12 * min),
    taints: [{ key: 'node.kubernetes.io/disk-pressure', effect: 'NoSchedule', timeAdded: ago(12 * min) }] }),
  (n) => ({ ...n, memory: n.memAlloc * 0.74, memReq: n.memAlloc * 0.3, memLim: n.memAlloc * 2.4,
    burstMem: n.memAlloc * 0.42, noMemLimit: 3, noMemRequest: 2 }),
  (n) => ({ ...n, metrics: false, conditions: notReadyConditions('Unknown', 'NodeStatusUnknown', 'Kubelet stopped posting node status.', 5 * min),
    taints: [{ key: 'node.kubernetes.io/unreachable', effect: 'NoSchedule', timeAdded: ago(5 * min) }, { key: 'node.kubernetes.io/unreachable', effect: 'NoExecute', timeAdded: ago(5 * min) }] }),
  (n) => ({ ...n, metrics: false, conditions: notReadyConditions('False', 'KubeletNotReady', 'container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: cni plugin not initialized', 3 * min),
    taints: [{ key: 'node.kubernetes.io/not-ready', effect: 'NoSchedule', timeAdded: ago(3 * min) }] }),
  (n) => ({ ...n, conditions: extraCondition({ type: 'NetworkUnavailable', reason: 'NoRouteCreated', message: 'Node created without a route', lastTransitionTime: ago(9 * min) }) }),
  (n) => ({ ...n, memory: n.memAlloc * 0.93, memReq: n.memAlloc * 0.82,
    conditions: pressure('MemoryPressure', 'KubeletHasInsufficientMemory', 'kubelet has insufficient memory available', 7 * min),
    taints: [{ key: 'node.kubernetes.io/memory-pressure', effect: 'NoSchedule', timeAdded: ago(7 * min) }] }),
  (n) => ({ ...n, heartbeat: 27 }),
  (n) => ({ ...n, conditions: extraCondition({ type: 'ReadonlyFilesystem', reason: 'FilesystemIsReadOnly', message: 'Node filesystem /var/lib/containerd is read-only', lastTransitionTime: ago(23 * min) }) }),
  (n) => ({ ...n, conditions: extraCondition({ type: 'KernelDeadlock', reason: 'DockerHung', message: 'task containerd:1123 blocked for more than 120 seconds.', lastTransitionTime: ago(41 * min) }) }),
  (n) => ({ ...n, readyTransitions: 4, conditions: [ready(6 * min), ok('MemoryPressure'), ok('DiskPressure'), ok('PIDPressure')] }),
  (n) => ({ ...n, memory: n.memAlloc * 0.92, memReq: n.memAlloc * 0.85 }),
  (n) => ({ ...n, memory: n.memAlloc * 0.84, memReq: n.memAlloc * 0.8 }),
  (n) => ({ ...n, oomKills: 3 }),
  (n) => ({ ...n, cpuReq: n.cpuAlloc * 0.95 }),
  (n) => ({ ...n, unschedulable: true, pods: 12,
    taints: [{ key: 'node.kubernetes.io/unschedulable', effect: 'NoSchedule', timeAdded: ago(4 * min) }] }),
  (n) => ({ ...n, conditions: pressure('PIDPressure', 'KubeletHasInsufficientPID', 'kubelet has insufficient PID available', 2 * min),
    taints: [{ key: 'node.kubernetes.io/pid-pressure', effect: 'NoSchedule', timeAdded: ago(2 * min) }] }),
  (n) => ({ ...n, cpu: n.cpuAlloc * 0.96, cpuReq: n.cpuAlloc * 0.8 }),
  (n) => ({ ...n, podsAlloc: 58, pods: 56 }),
  (n) => ({ ...n, memory: n.memAlloc * 0.72, memReq: n.memAlloc * 0.35, memLim: n.memAlloc * 1.9, burstMem: n.memAlloc * 0.36 })
]

// In the large cluster, every fourth node past the first 20 runs high on memory.
const LARGE_ERROR = (n, idx) => ({ ...n, memory: n.memAlloc * (0.8 + (idx % 15) / 100), memReq: n.memAlloc * 0.78 })

/**
 * Build the raw cluster: healthy or failing, generic or OpenShift on bare
 * metal, 20 or 120 nodes. Error nodes keep their base taints (e.g. the
 * control-plane taint).
 */
function rawCluster({ failing, onprem, large, metrics, podMetrics }) {
  const params = layout(onprem, large).map(([pool, i], idx) => {
    const base = healthy(pool, i, onprem, large)
    if (!failing) return base
    const error = ERRORS[idx] || (idx % 4 === 0 ? (n) => LARGE_ERROR(n, idx) : null)
    if (!error) return base
    const n = error(base)
    return n.taints === base.taints ? n : { ...n, taints: [...base.taints, ...n.taints] }
  })
  return {
    metricsAvailable: metrics,
    podMetricsAvailable: podMetrics,
    controlPlaneVersion: onprem ? 'v1.31.6' : 'v1.33.4',
    unschedulablePods: failing ? 4 : 0,
    unschedulableReasons: failing ? [{ reason: 'Insufficient memory', pods: 3 }, { reason: 'untolerated taint', pods: 1 }] : [],
    unboundedNamespaces: failing
      ? { withoutRequest: [{ namespace: 'apps', pods: 2 }], withoutLimit: [{ namespace: 'monitoring', pods: 2 }, { namespace: 'data', pods: 1 }] }
      : { withoutRequest: [], withoutLimit: [] },
    nodes: params.map((p, i) => rawNode(p, i))
  }
}

const switches = () => new URLSearchParams(window.location.search)

let cached = null

/**
 * The mock nodes snapshot for the current URL switches. Built once per page
 * load, so the report summary and the dashboard share one history and agree.
 */
export function mockNodesScenario() {
  const params = switches()
  const key = ['healthy', 'onprem', 'nometrics', 'nopodmetrics', 'large'].map(k => params.has(k)).join()
  if (cached?.key !== key) {
    cached = {
      key,
      snapshot: buildSnapshot(rawCluster({
        failing: !params.has('healthy'),
        onprem: params.has('onprem'),
        large: params.has('large'),
        metrics: !params.has('nometrics'),
        podMetrics: !params.has('nometrics') && !params.has('nopodmetrics')
      }))
    }
  }
  return cached.snapshot
}

/**
 * The name-less summary injected into the mock report as spec.nodes.
 */
export function mockNodesSummary() {
  return buildSummary(mockNodesScenario())
}

/**
 * Whether the mock user can list nodes; ?viewer simulates a user without access.
 */
export function mockCanViewNodes() {
  return !switches().has('viewer')
}

function forbidden() {
  const err = new Error('forbidden: user cannot list nodes')
  err.status = 403
  return err
}

/**
 * Mock GET /api/v1/nodes: the snapshot, or a 403 for users who cannot list nodes.
 */
export function mockNodes() {
  if (!mockCanViewNodes()) throw forbidden()
  return mockNodesScenario()
}

const WORKLOADS = [
  { kind: 'StatefulSet', namespace: 'data', name: 'postgres', pods: 3, nodes: 3, usage: 12.4 * GiB, requests: 6 * GiB, limits: null, noLimit: true },
  { kind: 'Deployment', namespace: 'apps', name: 'checkout', pods: 6, nodes: 4, usage: 7.1 * GiB, requests: 3 * GiB, limits: 12 * GiB, noLimit: false },
  { kind: 'Deployment', namespace: 'monitoring', name: 'prometheus-server', pods: 1, nodes: 1, usage: 5.6 * GiB, requests: 2 * GiB, limits: null, noLimit: true },
  { kind: 'DaemonSet', namespace: 'logging', name: 'fluent-bit', pods: 20, nodes: 20, usage: 3.2 * GiB, requests: 1.25 * GiB, limits: 5 * GiB, noLimit: false },
  { kind: 'CronJob', namespace: 'batch', name: 'nightly-report', pods: 2, nodes: 2, usage: 2.9 * GiB, requests: 1 * GiB, limits: 4 * GiB, noLimit: false },
  { kind: 'Deployment', namespace: 'apps', name: 'search-indexer', pods: 1, nodes: 1, usage: 0.6 * GiB, requests: 0.25 * GiB, limits: null, noLimit: true }
]

/**
 * Mock GET /api/v1/nodes/workloads: the top workloads by memory above
 * requests, empty for a healthy cluster, unavailable without metrics.
 */
export function mockNodesWorkloads() {
  if (!mockCanViewNodes()) throw forbidden()
  const params = switches()
  if (params.has('nometrics') || params.has('nopodmetrics')) return { metricsAvailable: false, workloads: [] }
  if (params.has('healthy')) return { metricsAvailable: true, workloads: [] }
  return {
    metricsAvailable: true,
    workloads: WORKLOADS.map(w => ({ ...w, aboveRequests: Math.round((w.usage - w.requests) * 0.9) }))
  }
}
