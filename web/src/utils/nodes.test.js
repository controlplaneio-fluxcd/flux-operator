// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  summaryTitle, lineDetail, nodeBadge, nodeIssue, commonNodeSuffix, shortNamer,
  cappedList, formatAge, usageSeverity, plural
} from './nodes'

describe('summaryTitle', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-05T12:00:00Z'))
  })
  afterEach(() => vi.useRealTimers())

  it('words per-node lines with the node count', () => {
    expect(summaryTitle({ code: 'memory-high', nodes: 2 })).toBe('2 nodes with high memory')
    expect(summaryTitle({ code: 'cpu-high', nodes: 1 })).toBe('1 node with high CPU')
    expect(summaryTitle({ code: 'node-condition', nodes: 2 })).toBe('2 nodes with failing conditions')
    expect(summaryTitle({ code: 'memory-overcommit', nodes: 2, value: 2 })).toBe('2 nodes with overcommitted memory')
  })

  it('adds the age to a single unreachable node', () => {
    expect(summaryTitle({ code: 'unreachable', nodes: 1, since: '2026-10-05T11:55:00Z' })).toBe('1 node unreachable for 5m')
    expect(summaryTitle({ code: 'unreachable', nodes: 2, since: '2026-10-05T11:55:00Z' })).toBe('2 nodes unreachable')
    expect(summaryTitle({ code: 'unreachable', nodes: 1 })).toBe('1 node unreachable')
  })

  it('words cluster-wide lines with their value', () => {
    expect(summaryTitle({ code: 'unschedulable-pods', value: 4 })).toBe('4 pods unschedulable')
    expect(summaryTitle({ code: 'kubelet-skew', value: 2 })).toBe('2 kubelet versions in use')
    expect(summaryTitle({ code: 'no-memory-request', value: 1 })).toBe('1 pod without memory request')
    expect(summaryTitle({ code: 'no-headroom-cpu' })).toBe('No CPU headroom for node loss')
  })

  it('falls back to a generic wording for unknown codes', () => {
    expect(summaryTitle({ code: 'from-the-future', nodes: 3 })).toBe('3 nodes affected')
    expect(summaryTitle({ code: 'from-the-future', value: 3 })).toBe('Cluster issue')
  })
})

describe('lineDetail', () => {
  it('lists affected nodes worst first, capped at five', () => {
    const affected = Array.from({ length: 8 }, (_, i) => ({ node: `n-${i}.example.com`, severity: i === 7 ? 'critical' : 'warn', value: 80 + i }))
    const detail = lineDetail({ code: 'memory-high', affected }, {}, shortNamer('.example.com'))
    expect(detail).toBe('n-7, n-6, n-5, n-4, n-3 +3 more')
  })

  it('adds the condition type for failing conditions', () => {
    const detail = lineDetail({ code: 'node-condition', affected: [{ node: 'a', severity: 'warn', conditionType: 'KernelDeadlock' }] })
    expect(detail).toBe('a KernelDeadlock')
  })

  it('lists scheduler reasons and namespaces with pod counts', () => {
    const data = {
      unschedulableReasons: [{ reason: 'Insufficient memory', pods: 3 }, { reason: 'untolerated taint', pods: 1 }],
      unboundedNamespaces: { withoutRequest: [], withoutLimit: [{ namespace: 'data', pods: 1 }, { namespace: 'monitoring', pods: 2 }] }
    }
    expect(lineDetail({ code: 'unschedulable-pods', value: 4 }, data)).toBe('Insufficient memory (3), untolerated taint (1)')
    expect(lineDetail({ code: 'no-memory-limit', value: 3 }, data)).toBe('monitoring (2), data (1)')
    expect(lineDetail({ code: 'no-memory-limit', namespaces: [{ namespace: 'apps', pods: 2 }] }, data)).toBe('apps (2)')
  })

  it('formats versions and the N-1 shortfall', () => {
    expect(lineDetail({ code: 'kubelet-skew', versions: ['v1.33.4', 'v1.29.10'] }, { controlPlaneVersion: 'v1.33.4' }))
      .toBe('v1.33.4, v1.29.10 (API server v1.33.4)')
    expect(lineDetail({ code: 'no-headroom-cpu', shortBy: 2.5 })).toBe('requests 2.50 cores over N-1 capacity')
    expect(lineDetail({ code: 'no-headroom-memory', shortBy: 2 * 1024 ** 3 })).toBe('requests 2 GiB over N-1 capacity')
  })

  it('leaves node names out of version details', () => {
    const affected = [{ node: 'system-03', severity: 'warn', value: 2 }]
    expect(lineDetail({ code: 'kubelet-skew', affected, versions: ['v1.33.4', 'v1.29.10'] })).toBe('v1.33.4, v1.29.10')
  })

  it('returns an empty detail when nothing is known', () => {
    expect(lineDetail({ code: 'from-the-future' })).toBe('')
  })
})

describe('nodeBadge', () => {
  it('shows the Kubernetes state of down and pressured nodes in red', () => {
    expect(nodeBadge({ status: 'Unreachable', findings: [] })).toEqual({ label: 'Unreachable', class: 'status-not-ready', severity: 'critical' })
    expect(nodeBadge({ status: 'Pressure', pressures: ['MemoryPressure', 'DiskPressure'] }).label).toBe('MemoryPressure, DiskPressure')
  })

  it('keeps the Kubernetes label of a cordoned node in yellow', () => {
    expect(nodeBadge({ status: 'Cordoned', unschedulable: true })).toEqual({ label: 'Cordoned', class: 'status-warning', severity: 'warn' })
    const unreachable = nodeBadge({ status: 'Unreachable', unschedulable: true, findings: [{ code: 'unreachable', severity: 'info' }] })
    expect(unreachable).toEqual({ label: 'Unreachable', class: 'status-warning', severity: 'warn' })
    expect(nodeBadge({ status: 'Pressure', pressures: ['PIDPressure'], unschedulable: true }).severity).toBe('warn')
  })

  it('shows the worst critical or warn finding of a Ready node', () => {
    const node = {
      status: 'Ready',
      findings: [
        { code: 'cordoned', severity: 'info' },
        { code: 'oom-kills', severity: 'warn', value: 3 },
        { code: 'memory-high', severity: 'critical', value: 93 }
      ]
    }
    expect(nodeBadge(node)).toEqual({ label: 'Memory high', class: 'status-not-ready', severity: 'critical' })
    expect(nodeBadge({ status: 'Ready', findings: [{ code: 'node-condition', severity: 'warn', conditionType: 'ReadonlyFilesystem' }] }).label)
      .toBe('ReadonlyFilesystem')
    expect(nodeBadge({ status: 'Ready', findings: [{ code: 'pod-capacity', severity: 'warn', value: 96 }] }).label).toBe('Pods 96%')
  })

  it('falls back to Critical or Warning for unknown codes', () => {
    expect(nodeBadge({ status: 'Ready', findings: [{ code: 'x', severity: 'critical' }] }).label).toBe('Critical')
    expect(nodeBadge({ status: 'Ready', findings: [{ code: 'x', severity: 'warn' }] }).label).toBe('Warning')
    // Cluster-wide codes have no pill label of their own.
    expect(nodeBadge({ status: 'Ready', findings: [{ code: 'no-memory-request', severity: 'warn' }] }).label).toBe('Warning')
  })

  it('reads Ready when only advisories are raised', () => {
    expect(nodeBadge({ status: 'Ready', findings: [{ code: 'kubelet-skew', severity: 'info' }] }))
      .toEqual({ label: 'Ready', class: 'status-ready', severity: 'ok' })
  })

  it('shows a node without a Ready condition as Unknown', () => {
    expect(nodeBadge({ status: 'Unknown', findings: [] })).toMatchObject({ label: 'Unknown', severity: 'unknown' })
  })
})

describe('nodeIssue', () => {
  it('words a node finding with its figures and raw message', () => {
    const node = { pods: 56, allocatable: { pods: 58, memory: 100 }, limits: { memory: 240 }, aboveRequests: { memory: 6 * 1024 ** 3 }, info: { kubeletVersion: 'v1.29.10' } }
    expect(nodeIssue({ code: 'memory-high', value: 93 }, node)).toEqual({ title: 'Memory 93%', detail: '' })
    expect(nodeIssue({ code: 'pod-capacity', value: 96 }, node)).toEqual({ title: 'Pods 96%', detail: '56 / 58' })
    expect(nodeIssue({ code: 'memory-overcommit', value: 240 }, node))
      .toEqual({ title: 'Memory overcommitted', detail: 'Limits 240% of allocatable · 6 GiB used above requests' })
    expect(nodeIssue({ code: 'network-unavailable', message: 'no route' }, node)).toEqual({ title: 'Network unavailable', detail: 'no route' })
    expect(nodeIssue({ code: 'kubelet-skew' }, node, 'v1.33.4')).toEqual({ title: 'Kubelet v1.29.10', detail: 'API server v1.33.4' })
    expect(nodeIssue({ code: 'oom-kills', value: 1 }, node).title).toBe('1 OOM kill')
  })

  it('shows the age and raw message of unreachable and network findings', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-05T12:00:00Z'))
    const since = '2026-10-05T11:55:00Z'
    expect(nodeIssue({ code: 'unreachable', since, message: 'Kubelet stopped posting node status.' }, {}))
      .toEqual({ title: 'Unreachable', detail: 'for 5m · Kubelet stopped posting node status.' })
    expect(nodeIssue({ code: 'unreachable', reason: 'NodeStatusUnknown' }, {}))
      .toEqual({ title: 'Unreachable', detail: 'NodeStatusUnknown' })
    expect(nodeIssue({ code: 'network-unavailable', since, reason: 'NoRouteCreated' }, {}))
      .toEqual({ title: 'Network unavailable', detail: 'for 5m · NoRouteCreated' })
    vi.useRealTimers()
  })

  it('falls back to the code for unknown findings', () => {
    expect(nodeIssue({ code: 'from-the-future', message: 'm' }, {})).toEqual({ title: 'from-the-future', detail: 'm' })
    expect(nodeIssue({ code: 'no-memory-request', value: 2 }, {})).toEqual({ title: 'no-memory-request', detail: '' })
  })
})

describe('commonNodeSuffix', () => {
  it('returns the DNS suffix shared by all names', () => {
    expect(commonNodeSuffix(['a.eu.internal', 'b.eu.internal'])).toBe('.eu.internal')
    expect(commonNodeSuffix(['master-0.ocp.lab', 'worker-1.ocp.lab'])).toBe('.ocp.lab')
  })

  it('never strips a whole name', () => {
    expect(commonNodeSuffix(['x.example', 'a.x.example'])).toBe('.example')
  })

  it('returns an empty suffix for plain or single names', () => {
    expect(commonNodeSuffix(['a', 'b'])).toBe('')
    expect(commonNodeSuffix(['a.example'])).toBe('')
    expect(commonNodeSuffix(['a.one', 'b.two'])).toBe('')
  })

  it('builds a name shortener', () => {
    expect(shortNamer('.lab')('n1.lab')).toBe('n1')
    expect(shortNamer('')('n1.lab')).toBe('n1.lab')
  })
})

describe('formatting', () => {
  it('caps lists at five names', () => {
    expect(cappedList(['a', 'b'])).toBe('a, b')
    expect(cappedList(['a', 'b', 'c', 'd', 'e', 'f', 'g'])).toBe('a, b, c, d, e +2 more')
  })

  it('formats ages and never renders NaN', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-05T12:00:00Z'))
    expect(formatAge('2026-10-05T11:59:15Z')).toBe('45s')
    expect(formatAge('2026-10-05T11:48:00Z')).toBe('12m')
    expect(formatAge('2026-10-05T09:00:00Z')).toBe('3h')
    expect(formatAge('2026-09-29T12:00:00Z')).toBe('6d')
    expect(formatAge('2026-10-05T12:00:30Z')).toBe('0s')
    expect(formatAge(undefined)).toBeNull()
    expect(formatAge('garbage')).toBeNull()
    vi.useRealTimers()
  })

  it('colors usage at the node thresholds', () => {
    expect(usageSeverity('memory', 79)).toBeNull()
    expect(usageSeverity('memory', 80)).toBe('warn')
    expect(usageSeverity('memory', 90)).toBe('critical')
    expect(usageSeverity('cpu', 94)).toBe('warn')
    expect(usageSeverity('cpu', 95)).toBe('critical')
    expect(usageSeverity('cpu', null)).toBeNull()
  })

  it('pluralizes', () => {
    expect(plural(1, 'node')).toBe('1 node')
    expect(plural(0, 'node')).toBe('0 nodes')
  })
})
