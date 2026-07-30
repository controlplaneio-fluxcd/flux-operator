// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { ResourceMetric, sumResourceUsage } from './ResourceMetric'

describe('ResourceMetric', () => {
  it('renders label, value and percent label', () => {
    render(<ResourceMetric label="CPU" value="120m" percentLabel="12% of limit" barPercent={12} />)
    expect(screen.getByText('CPU')).toBeInTheDocument()
    expect(screen.getByText('120m')).toBeInTheDocument()
    expect(screen.getByText(/12% of limit/)).toBeInTheDocument()
  })

  it('omits the bar and percent label when no limit is set', () => {
    const { container } = render(<ResourceMetric label="Memory" value="128 MiB" percentLabel={null} barPercent={null} />)
    expect(screen.getByText('128 MiB')).toBeInTheDocument()
    expect(container.querySelector('.rounded-full')).toBeNull()
  })

  it('colors the bar by utilization thresholds', () => {
    const bar = percent => {
      const { container } = render(<ResourceMetric label="CPU" value="1" barPercent={percent} />)
      return container.querySelector('.h-2.rounded-full > div').className
    }
    expect(bar(50)).toContain('bg-green-500')
    expect(bar(75)).toContain('bg-yellow-500')
    expect(bar(90)).toContain('bg-red-500')
  })
})

describe('sumResourceUsage', () => {
  it('sums usage, requests and limits across entries', () => {
    const sum = sumResourceUsage([
      { cpu: 0.1, cpuRequests: 0.2, cpuLimits: 1, memory: 100, memoryRequests: 200, memoryLimits: 300 },
      { cpu: 0.3, memory: 50 },
    ])
    expect(sum.cpu).toBeCloseTo(0.4)
    expect(sum.cpuRequests).toBeCloseTo(0.2)
    expect(sum.cpuLimits).toBeCloseTo(1)
    expect(sum.memory).toBe(150)
    expect(sum.memoryRequests).toBe(200)
    expect(sum.memoryLimits).toBe(300)
  })

  it('returns null for empty input', () => {
    expect(sumResourceUsage([])).toBeNull()
    expect(sumResourceUsage(undefined)).toBeNull()
  })
})
