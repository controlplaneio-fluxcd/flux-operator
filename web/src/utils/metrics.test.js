// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import {
  formatCores, formatBytes, percentOf, percentText,
  buildChartData, latestSample, hasChartableMetrics,
} from './metrics'

describe('formatCores', () => {
  it('formats sub-core values as millicores', () => {
    expect(formatCores(0.12)).toBe('120m')
    expect(formatCores(0.001)).toBe('1m')
    expect(formatCores(0.9995)).toBe('1000m')
  })

  it('formats whole cores with decimals', () => {
    expect(formatCores(1.254)).toBe('1.25')
    expect(formatCores(12.34)).toBe('12.3')
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
  it('computes rounded percentages', () => {
    expect(percentOf(0.05, 0.1)).toBe(50)
    expect(percentOf(1, 3)).toBe(33)
  })

  it('returns null for unset denominators', () => {
    expect(percentOf(0.5, 0)).toBeNull()
    expect(percentOf(0.5, undefined)).toBeNull()
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
