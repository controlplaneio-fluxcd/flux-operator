// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { CapacityPanel } from './CapacityPanel'

// Mocked to avoid uPlot canvas rendering in jsdom.
vi.mock('../workload/UsageChart', () => ({
  UsageChart: (props) => <div data-testid={props.testId} data-points={props.data[0].length} data-limit={String(props.hasLimit)} data-limit-label={props.limitLabel} />
}))

const GiB = 1024 ** 3

function data(overrides = {}) {
  return {
    metricsAvailable: true,
    capacity: {
      cpu: { allocatable: 100, requested: 57, used: 91, aboveRequests: 1, limits: 150 },
      memory: { allocatable: 100 * GiB, requested: 61 * GiB, used: 56 * GiB, aboveRequests: 5 * GiB, limits: 120 * GiB },
      pods: { allocatable: 1772, running: 614 }
    },
    usage: {
      samples: [
        { t: '2026-10-05T11:59:00Z', cpu: 80, memory: 50 * GiB },
        { t: '2026-10-05T12:00:00Z', cpu: 91, memory: 56 * GiB }
      ]
    },
    ...overrides
  }
}

describe('CapacityPanel', () => {
  it('shows pod slots, used and requested shares and the charts', () => {
    render(<CapacityPanel data={data()} />)
    expect(screen.getByText('614 / 1772 pods')).toBeInTheDocument()
    const cpu = screen.getByTestId('nodes-cpu-header')
    expect(cpu).toHaveTextContent('91% used')
    expect(cpu).toHaveTextContent('57% requested')
    expect(screen.getByText('91% used').className).toContain('text-red-600')
    expect(screen.getByTestId('nodes-memory-header')).toHaveTextContent('56% used')
    expect(screen.getByTestId('nodes-cpu-chart')).toHaveAttribute('data-points', '2')
    expect(screen.getByTestId('nodes-memory-chart')).toBeInTheDocument()
  })

  it('labels the threshold line as allocatable', () => {
    render(<CapacityPanel data={data()} />)
    expect(screen.getByTestId('nodes-cpu-chart')).toHaveAttribute('data-limit-label', 'allocatable')
    expect(screen.getByTestId('nodes-memory-chart')).toHaveAttribute('data-limit-label', 'allocatable')
  })

  it('exposes the absolute amounts to screen readers', () => {
    render(<CapacityPanel data={data()} />)
    const header = screen.getByTestId('nodes-cpu-header').parentElement
    expect(header.querySelector('.sr-only')).toHaveTextContent('91.0 cores used · 57.0 cores requested · 100.0 cores allocatable')
  })

  it('shows request bars without metrics-server', () => {
    const d = data({ metricsAvailable: false, usage: { samples: [] } })
    render(<CapacityPanel data={d} />)
    expect(screen.queryByTestId('nodes-cpu-chart')).not.toBeInTheDocument()
    expect(screen.getByTestId('nodes-cpu-requests')).toHaveTextContent('57% requested')
    expect(screen.getByTestId('nodes-memory-requests')).toHaveTextContent('61% requested')
  })

  it('never renders null% without eligible capacity', () => {
    const empty = { allocatable: null, requested: null, used: null, aboveRequests: null, limits: null }
    render(<CapacityPanel data={data({ capacity: { cpu: empty, memory: empty, pods: { allocatable: null, running: null } } })} />)
    expect(screen.getByTestId('nodes-capacity-empty')).toHaveTextContent('No allocatable capacity')
    expect(document.body.textContent).not.toMatch(/null|NaN/)
  })
})
