// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import {
  formatCores, formatBytes, percentOf, percentText, limitSeverity, percentSeverity,
  buildChartData, latestSample, hasChartableMetrics,
  usageSeries, trimPodUsage, usageAnnotation, sumSeries,
} from './metrics'

describe('formatCores', () => {
  it('formats sub-core values as millicores', () => {
    expect(formatCores(0.12)).toBe('120m')
    expect(formatCores(0.001)).toBe('1m')
  })

  it('formats whole cores with decimals', () => {
    expect(formatCores(1.254)).toBe('1.25')
    expect(formatCores(12.34)).toBe('12.3')
  })

  it('formats values rounding up to a full core as cores', () => {
    expect(formatCores(0.9995)).toBe('1.00')
    expect(formatCores(0.9994)).toBe('999m')
  })

  it('handles zero and invalid input', () => {
    expect(formatCores(0)).toBe('0m')
    expect(formatCores(-1)).toBe('0m')
    expect(formatCores(NaN)).toBe('0m')
    expect(formatCores(undefined)).toBe('0m')
  })
})

describe('formatBytes', () => {
  it('formats binary units', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(2048)).toBe('2 KiB')
    expect(formatBytes(128 * 1024 * 1024)).toBe('128 MiB')
    expect(formatBytes(1.25 * 1024 * 1024 * 1024)).toBe('1.25 GiB')
  })

  it('scales decimals with magnitude', () => {
    expect(formatBytes(150.5 * 1024 * 1024)).toBe('151 MiB')
    expect(formatBytes(15.55 * 1024 * 1024)).toBe('15.6 MiB')
  })

  it('handles zero and invalid input', () => {
    expect(formatBytes(0)).toBe('0')
    expect(formatBytes(-5)).toBe('0')
    expect(formatBytes(undefined)).toBe('0')
  })
})

describe('percentOf', () => {
  it('computes truncated percentages', () => {
    expect(percentOf(0.05, 0.1)).toBe(50)
    expect(percentOf(1, 3)).toBe(33)
  })

  it('truncates so the display agrees with the severity thresholds', () => {
    // 89.5% must not display as "90%" next to a non-critical color.
    expect(percentOf(0.179, 0.2)).toBe(89)
    expect(percentOf(0.1799, 0.2)).toBe(89)
    // An exact threshold ratio is not floored away by double artifacts.
    expect(percentOf(0.18, 0.2)).toBe(90)
  })

  it('clamps negative usage to zero', () => {
    expect(percentOf(-100, 1024)).toBe(0)
  })

  it('returns null for unset denominators', () => {
    expect(percentOf(0.5, 0)).toBeNull()
    expect(percentOf(0.5, undefined)).toBeNull()
  })

  it('returns null for non-finite values', () => {
    expect(percentOf(NaN, 1)).toBeNull()
    expect(percentOf(Infinity, 1)).toBeNull()
    expect(percentOf(0.5, NaN)).toBeNull()
  })
})

describe('percentText', () => {
  it('joins request and limit fragments', () => {
    expect(percentText(0.05, 0.1, 0.5)).toBe('50% of request · 10% of limit')
  })

  it('omits unset denominators', () => {
    expect(percentText(0.05, 0.1, 0)).toBe('50% of request')
    expect(percentText(0.05, 0, 0.5)).toBe('10% of limit')
    expect(percentText(0.05, 0, 0)).toBeNull()
  })
})

describe('limitSeverity', () => {
  it('classifies usage by proximity to the limit', () => {
    expect(limitSeverity(0.5, 1)).toBeNull()
    expect(limitSeverity(0.75, 1)).toBeNull()
    expect(limitSeverity(0.8, 1)).toBe('warn')
    expect(limitSeverity(0.9, 1)).toBe('critical')
    expect(limitSeverity(1.2, 1)).toBe('critical')
  })

  it('returns null when no limit is set', () => {
    expect(limitSeverity(0.9, 0)).toBeNull()
    expect(limitSeverity(0.9, undefined)).toBeNull()
    expect(limitSeverity(NaN, 1)).toBeNull()
  })

  it('classifies the exact ratio, not the rounded display percentage', () => {
    expect(limitSeverity(0.795, 1)).toBeNull()
    expect(limitSeverity(0.895, 1)).toBe('warn')
  })
})

describe('percentSeverity', () => {
  it('classifies percentages against the 80/90 thresholds', () => {
    expect(percentSeverity(79)).toBeNull()
    expect(percentSeverity(80)).toBe('warn')
    expect(percentSeverity(89)).toBe('warn')
    expect(percentSeverity(90)).toBe('critical')
    expect(percentSeverity(null)).toBeNull()
  })
})

describe('buildChartData', () => {
  const samples = [
    { t: '2026-07-28T10:00:00Z', cpu: 0.1, memory: 1000 },
    { t: '2026-07-28T10:01:00Z', cpu: 0.2, memory: 2000 },
  ]

  it('builds aligned data with timestamps in seconds', () => {
    const { data, hasLimit } = buildChartData(samples, 'cpu', 0)
    expect(hasLimit).toBe(false)
    expect(data).toHaveLength(2)
    expect(data[0]).toEqual([
      Math.floor(Date.parse(samples[0].t) / 1000),
      Math.floor(Date.parse(samples[1].t) / 1000),
    ])
    expect(data[1]).toEqual([0.1, 0.2])
  })

  it('adds a threshold series when the limit is near the usage', () => {
    const { data, hasLimit } = buildChartData(samples, 'cpu', 0.3)
    expect(hasLimit).toBe(true)
    expect(data[2]).toEqual([0.3, 0.3])
  })

  it('omits a far-off limit that would flatten the curve', () => {
    const { hasLimit } = buildChartData(samples, 'cpu', 10)
    expect(hasLimit).toBe(false)
  })

  it('skips malformed samples', () => {
    const { data } = buildChartData(
      [...samples, { t: 'not-a-date', cpu: 1, memory: 1 }, { t: '2026-07-28T10:02:00Z' }],
      'memory', 0,
    )
    expect(data[1]).toEqual([1000, 2000])
  })

  it('handles empty input', () => {
    const { data, hasLimit } = buildChartData([], 'cpu', 1)
    expect(data).toEqual([[], []])
    expect(hasLimit).toBe(false)
  })
})

describe('latestSample', () => {
  it('returns the last sample', () => {
    const metrics = { samples: [{ cpu: 1 }, { cpu: 2 }] }
    expect(latestSample(metrics).cpu).toBe(2)
  })

  it('returns null when empty', () => {
    expect(latestSample({ samples: [] })).toBeNull()
    expect(latestSample(undefined)).toBeNull()
  })
})

describe('hasChartableMetrics', () => {
  it('requires at least two samples', () => {
    expect(hasChartableMetrics({ samples: [{}, {}] })).toBe(true)
    expect(hasChartableMetrics({ samples: [{}] })).toBe(false)
    expect(hasChartableMetrics({})).toBe(false)
    expect(hasChartableMetrics(undefined)).toBe(false)
  })
})

describe('usageSeries', () => {
  const pods = [
    { name: 'app-1', metrics: { cpu: 0.02, memory: 64 } },
    { name: 'app-2', metrics: { cpu: 0.05, memory: 32 } },
    { name: 'app-3' },
  ]

  it('sorts pods by usage descending with sampleless pods last', () => {
    expect(usageSeries(pods, 'cpu')).toEqual([
      { name: 'app-2', value: 0.05, limit: 0 },
      { name: 'app-1', value: 0.02, limit: 0 },
      { name: 'app-3', value: null, limit: 0 },
    ])
    expect(usageSeries(pods, 'memory')).toEqual([
      { name: 'app-1', value: 64, limit: 0 },
      { name: 'app-2', value: 32, limit: 0 },
      { name: 'app-3', value: null, limit: 0 },
    ])
  })

  it('carries the pod spec limit for the field', () => {
    const withLimits = [
      { name: 'app-1', metrics: { cpu: 0.1, memory: 64 }, resources: { cpuLimits: 0.5, memoryLimits: 128 } },
    ]
    expect(usageSeries(withLimits, 'cpu')).toEqual([
      { name: 'app-1', value: 0.1, limit: 0.5 },
    ])
    expect(usageSeries(withLimits, 'memory')).toEqual([
      { name: 'app-1', value: 64, limit: 128 },
    ])
  })

  it('keeps pods without a usage sample or with invalid values as null', () => {
    expect(usageSeries([{ name: 'b', metrics: { cpu: NaN } }, { name: 'a' }], 'cpu')).toEqual([
      { name: 'a', value: null, limit: 0 },
      { name: 'b', value: null, limit: 0 },
    ])
    expect(usageSeries(undefined, 'cpu')).toEqual([])
  })

  it('keeps zero-usage pods in the list', () => {
    expect(usageSeries([{ name: 'idle', metrics: { cpu: 0 } }], 'cpu')).toEqual([
      { name: 'idle', value: 0, limit: 0 },
    ])
  })

  it('carries kind and namespace through for workload items', () => {
    const workloads = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', metrics: { cpu: 0.02, memory: 64 } },
      { kind: 'StatefulSet', namespace: 'data', name: 'frontend' },
    ]
    expect(usageSeries(workloads, 'cpu')).toEqual([
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', value: 0.02, limit: 0 },
      { kind: 'StatefulSet', namespace: 'data', name: 'frontend', value: null, limit: 0 },
    ])
  })

  it('breaks usage ties on namespace/name', () => {
    const workloads = [
      { kind: 'Deployment', namespace: 'zoo', name: 'app', metrics: { cpu: 0.1, memory: 1 } },
      { kind: 'Deployment', namespace: 'alpha', name: 'app', metrics: { cpu: 0.1, memory: 1 } },
    ]
    const sorted = usageSeries(workloads, 'cpu')
    expect(sorted[0].namespace).toBe('alpha')
    expect(sorted[1].namespace).toBe('zoo')
    // Sampleless items compare namespace/name too.
    const sampleless = usageSeries(
      [
        { namespace: 'zoo', name: 'app', kind: 'Deployment' },
        { namespace: 'alpha', name: 'app', kind: 'Deployment' },
      ],
      'cpu',
    )
    expect(sampleless[0].namespace).toBe('alpha')
  })
})

describe('sumSeries', () => {
  it('sums values that share a timestamp and sorts chronologically', () => {
    const a = [
      { t: '2026-07-28T10:01:00Z', cpu: 0.1, memory: 100 },
      { t: '2026-07-28T10:02:00Z', cpu: 0.2, memory: 200 },
    ]
    const b = [
      { t: '2026-07-28T10:02:00Z', cpu: 0.3, memory: 300 },
      { t: '2026-07-28T10:00:00Z', cpu: 0.4, memory: 400 },
    ]
    expect(sumSeries([a, b])).toEqual([
      { t: '2026-07-28T10:00:00Z', cpu: 0.4, memory: 400 },
      { t: '2026-07-28T10:01:00Z', cpu: 0.1, memory: 100 },
      { t: '2026-07-28T10:02:00Z', cpu: 0.5, memory: 500 },
    ])
  })

  it('tolerates missing and empty series', () => {
    expect(sumSeries(undefined)).toEqual([])
    expect(sumSeries([])).toEqual([])
    expect(sumSeries([undefined, []])).toEqual([])
    expect(sumSeries([[{ t: '2026-07-28T10:00:00Z', cpu: 0.1, memory: 1 }], undefined])).toEqual([
      { t: '2026-07-28T10:00:00Z', cpu: 0.1, memory: 1 },
    ])
  })
})

describe('usageAnnotation', () => {
  const metrics = {
    samples: [
      { t: '2026-07-28T10:00:00Z', cpu: 0.1, memory: 1 },
      { t: '2026-07-28T10:30:00Z', cpu: 0.2, memory: 2 },
    ],
  }

  it('returns the event time in seconds when inside the window', () => {
    expect(usageAnnotation(metrics, '2026-07-28T10:15:00Z')).toBe(Date.parse('2026-07-28T10:15:00Z') / 1000)
  })

  it('includes the window boundaries', () => {
    expect(usageAnnotation(metrics, '2026-07-28T10:00:00Z')).toBe(Date.parse('2026-07-28T10:00:00Z') / 1000)
    expect(usageAnnotation(metrics, '2026-07-28T10:30:00Z')).toBe(Date.parse('2026-07-28T10:30:00Z') / 1000)
  })

  it('returns null when the event is outside the window', () => {
    expect(usageAnnotation(metrics, '2026-07-28T09:59:00Z')).toBeNull()
    expect(usageAnnotation(metrics, '2026-07-28T10:31:00Z')).toBeNull()
  })

  it('returns null without an event, valid timestamp or chartable metrics', () => {
    expect(usageAnnotation(metrics, undefined)).toBeNull()
    expect(usageAnnotation(metrics, 'not-a-date')).toBeNull()
    expect(usageAnnotation({ samples: [] }, '2026-07-28T10:15:00Z')).toBeNull()
    expect(usageAnnotation(undefined, '2026-07-28T10:15:00Z')).toBeNull()
  })
})

describe('trimPodUsage', () => {
  // Sorted descending, as usageSeries returns.
  const measured = (count, base = 100) =>
    Array.from({ length: count }, (_, i) => ({ name: `pod-${i}`, value: base - i }))

  it('passes small workloads through unchanged', () => {
    const items = [...measured(7), { name: 'pod-new', value: null }]
    expect(trimPodUsage(items)).toEqual(items.map(i => ({ type: 'pod', ...i })))
    expect(trimPodUsage([])).toEqual([])
    expect(trimPodUsage(undefined)).toEqual([])
  })

  it('keeps the extremes and collapses the middle into an aggregate row', () => {
    const rows = trimPodUsage(measured(20))
    expect(rows).toHaveLength(6)
    // Top 3 by usage.
    expect(rows.slice(0, 3).map(r => r.name)).toEqual(['pod-0', 'pod-1', 'pod-2'])
    // The middle 15 pods collapse with their value range and average.
    expect(rows[3]).toEqual({ type: 'elision', count: 15, min: 100 - 17, max: 100 - 3, avg: 90 })
    // Bottom 2 stay visible (cold outliers).
    expect(rows.slice(4).map(r => r.name)).toEqual(['pod-18', 'pod-19'])
  })

  it('shows a single middle pod instead of eliding it', () => {
    // 3 top + 1 middle + 2 bottom = 6 measured, plus 4 N/A to exceed the budget.
    const items = [...measured(6), ...['a', 'b', 'c', 'd'].map(n => ({ name: n, value: null }))]
    const rows = trimPodUsage(items)
    expect(rows.filter(r => r.type === 'elision')).toHaveLength(0)
    expect(rows.filter(r => r.type === 'pod' && r.value !== null)).toHaveLength(6)
    expect(rows[rows.length - 1]).toEqual({ type: 'collecting', count: 4 })
  })

  it('keeps up to two sampleless pods as individual rows', () => {
    const items = [...measured(10), { name: 'new-1', value: null }, { name: 'new-2', value: null }]
    const rows = trimPodUsage(items)
    expect(rows.filter(r => r.type === 'collecting')).toHaveLength(0)
    expect(rows.slice(-2)).toEqual([
      { type: 'pod', name: 'new-1', value: null },
      { type: 'pod', name: 'new-2', value: null },
    ])
  })

  it('collapses a mass rollout into a single collecting row', () => {
    const items = [
      ...measured(3),
      ...Array.from({ length: 17 }, (_, i) => ({ name: `new-${i}`, value: null })),
    ]
    const rows = trimPodUsage(items)
    // 3 measured pods (fewer than top+bottom) and one collecting row.
    expect(rows).toHaveLength(4)
    expect(rows[3]).toEqual({ type: 'collecting', count: 17 })
  })

  it('handles a workload with no samples at all', () => {
    const items = Array.from({ length: 30 }, (_, i) => ({ name: `new-${i}`, value: null }))
    expect(trimPodUsage(items)).toEqual([{ type: 'collecting', count: 30 }])
  })
})
