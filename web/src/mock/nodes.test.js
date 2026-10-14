// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, afterEach } from 'vitest'
import { mockNodes, mockNodesScenario, mockNodesSummary, mockNodesWorkloads, mockCanViewNodes } from './nodes'

const at = (search) => window.history.replaceState({}, '', `/nodes${search}`)

describe('nodes mock', () => {
  afterEach(() => at(''))

  it('serves the snapshot shape with findings and checks', () => {
    at('')
    const s = mockNodes()
    expect(s.nodes).toHaveLength(20)
    expect(s.findings.length).toBeGreaterThan(0)
    expect(s.checks).toHaveLength(20)
    expect(s.findings[0]).toMatchObject({ code: 'unreachable', severity: 'critical', nodes: 1 })
    for (const n of s.nodes) {
      expect(n.status).toBeDefined()
      expect(Array.isArray(n.findings)).toBe(true)
    }
  })

  it('ends every node history on its usage, and the cluster history on the capacity used', () => {
    at('')
    const s = mockNodesScenario()
    const last = s.usage.samples[s.usage.samples.length - 1]
    expect(s.capacity.cpu.used).toBe(last.cpu)
    expect(s.capacity.memory.used).toBe(last.memory)
  })

  it('agrees with the report summary', () => {
    at('')
    const summary = mockNodesSummary()
    const s = mockNodesScenario()
    expect(summary.cpu).toEqual(s.capacity.cpu)
    expect(summary.findings.critical).toBe(s.findings.filter(l => l.severity === 'critical').length)
    expect(summary.findings.top[0].code).toBe(s.findings[0].code)
    expect(JSON.stringify(summary)).not.toMatch(/general-|system-|batch-|zone-|standard-/)
  })

  it('words memory overcommit with the node count and the highest limits share', () => {
    at('')
    const s = mockNodesScenario()
    const line = s.findings.find(l => l.code === 'memory-overcommit')
    expect(line.nodes).toBe(line.affected.length)
    expect(line.value).toBe(Math.max(...line.affected.map(a => a.value)))
    expect(line.value).toBeGreaterThan(100)
    const affected = new Set(line.affected.map(a => a.node))
    expect(s.nodes.filter(n => affected.has(n.name)).every(n => !n.unschedulable)).toBe(true)
  })

  it('keeps pods without memory request as a cluster-wide line only', () => {
    at('')
    const s = mockNodesScenario()
    const line = s.findings.find(l => l.code === 'no-memory-request')
    expect(line).toMatchObject({ severity: 'info', value: 2 })
    expect(line.affected).toBeUndefined()
    expect(line.nodes).toBeUndefined()
    expect(s.nodes.some(n => n.findings.some(f => f.code === 'no-memory-request'))).toBe(false)
  })

  it('excludes control-plane nodes from on-prem usage', () => {
    at('?onprem&healthy')
    const s = mockNodesScenario()
    const workers = s.nodes.filter(n => !n.controlPlane)
    const used = workers.reduce((sum, n) => sum + n.usage.memory, 0)
    expect(s.nodes.some(n => n.controlPlane)).toBe(true)
    expect(s.capacity.memory.used).toBeCloseTo(used, -3)
  })

  it('has no findings when healthy', () => {
    at('?healthy')
    const s = mockNodesScenario()
    expect(s.findings).toHaveLength(0)
    expect(s.checks.every(c => c.status === 'passed')).toBe(true)
    expect(mockNodesWorkloads().workloads).toHaveLength(0)
  })

  it('skips usage checks without metrics', () => {
    at('?nometrics')
    const s = mockNodesScenario()
    expect(s.capacity.cpu.used).toBeNull()
    expect(s.checks.find(c => c.name === 'Memory usage').status).toBe('skipped')
    expect(mockNodesWorkloads().metricsAvailable).toBe(false)
  })

  it('separates pod metrics from node metrics', () => {
    at('?nopodmetrics')
    const s = mockNodesScenario()
    expect(s.metricsAvailable).toBe(true)
    expect(s.podMetricsAvailable).toBe(false)
    expect(s.usage.samples.length).toBeGreaterThan(0)
    expect(s.capacity.memory.used).not.toBeNull()
    expect(s.capacity.memory.aboveRequests).toBeNull()
    expect(s.nodes.every(n => n.aboveRequests === null)).toBe(true)
    expect(mockNodesWorkloads()).toEqual({ metricsAvailable: false, workloads: [] })
  })

  it('skips usage findings on cordoned nodes', () => {
    at('')
    const s = mockNodesScenario()
    const cordoned = s.nodes.filter(n => n.unschedulable)
    expect(cordoned.length).toBeGreaterThan(0)
    for (const n of cordoned) {
      expect(n.findings.some(f => ['memory-high', 'cpu-high'].includes(f.code))).toBe(false)
    }
  })

  it('reads healthy with only advisories in the default cluster counts', () => {
    at('?healthy')
    expect(mockNodesSummary().findings).toMatchObject({ critical: 0, warn: 0, top: [] })
    at('')
    const summary = mockNodesSummary()
    expect(summary.findings.critical).toBeGreaterThan(0)
    expect(summary.findings.top.every(l => l.code !== 'no-memory-request')).toBe(true)
  })

  it('serves 120 nodes in the large cluster', () => {
    at('?large')
    const s = mockNodesScenario()
    expect(s.nodes).toHaveLength(120)
    expect(s.findings.find(l => l.code === 'memory-high').nodes).toBeGreaterThan(5)
  })

  it('ranks only workload kinds with a dashboard', () => {
    at('')
    const kinds = new Set(mockNodesWorkloads().workloads.map(w => w.kind))
    expect([...kinds].every(k => ['Deployment', 'StatefulSet', 'DaemonSet', 'CronJob'].includes(k))).toBe(true)
  })

  it('answers 403 to viewers', () => {
    at('?viewer')
    expect(mockCanViewNodes()).toBe(false)
    expect(() => mockNodes()).toThrow(expect.objectContaining({ status: 403 }))
    expect(() => mockNodesWorkloads()).toThrow(expect.objectContaining({ status: 403 }))
  })
})
