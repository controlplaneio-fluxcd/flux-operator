// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { WorkloadMetricsPanel } from './WorkloadMetricsPanel'

// The chart itself renders to canvas (unavailable in jsdom); it has its own
// lifecycle tests with a mocked uPlot, so the panel test stubs it out.
vi.mock('./UsageChart', () => ({
  UsageChart: (props) => (
    <div
      data-testid={props.testId}
      data-annotation={props.annotation ? `${props.annotation.label}@${props.annotation.time}` : ''}
    >
      chart
    </div>
  )
}))

// Build a metrics object with the given number of one-minute samples.
function buildMetrics(count, overrides = {}) {
  const samples = []
  for (let i = 0; i < count; i++) {
    samples.push({
      t: new Date(Date.parse('2026-07-28T10:00:00Z') + i * 60000).toISOString(),
      cpu: 0.1 + i * 0.01,
      memory: (100 + i) * 1024 * 1024,
    })
  }
  return {
    samples,
    cpuRequests: 0,
    cpuLimits: 0,
    memoryRequests: 0,
    memoryLimits: 0,
    ...overrides,
  }
}

describe('WorkloadMetricsPanel component', () => {
  it('renders nothing without metrics', () => {
    const { container } = render(<WorkloadMetricsPanel metrics={undefined} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing with a single sample', () => {
    const { container } = render(<WorkloadMetricsPanel metrics={buildMetrics(1)} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders CPU and Memory charts with current absolute values', () => {
    render(<WorkloadMetricsPanel metrics={buildMetrics(10)} />)

    expect(screen.getByText('Resource Usage')).toBeInTheDocument()
    expect(screen.getByTestId('cpu-usage-chart')).toBeInTheDocument()
    expect(screen.getByTestId('memory-usage-chart')).toBeInTheDocument()

    // Latest sample: cpu 0.19 cores -> 190m, memory 109 MiB.
    expect(screen.getByTestId('cpu-usage-header')).toHaveTextContent('190m')
    expect(screen.getByTestId('memory-usage-header')).toHaveTextContent('109 MiB')
  })

  it('omits percentages when requests and limits are unset', () => {
    render(<WorkloadMetricsPanel metrics={buildMetrics(10)} />)

    expect(screen.queryByText(/of request/)).not.toBeInTheDocument()
    expect(screen.queryByText(/of limit/)).not.toBeInTheDocument()
  })

  it('shows percentages of requests and limits when set', () => {
    render(
      <WorkloadMetricsPanel
        metrics={buildMetrics(10, {
          cpuRequests: 0.38,
          cpuLimits: 0.95,
          memoryRequests: 218 * 1024 * 1024,
        })}
      />
    )

    // cpu 0.19 of 0.38 request = 50%, of 0.95 limit = 20%.
    expect(screen.getByTestId('cpu-usage-header')).toHaveTextContent('50% of request · 20% of limit')
    // memory 109 MiB of 218 MiB request = 50%, no limit fragment.
    expect(screen.getByTestId('memory-usage-header')).toHaveTextContent('50% of request')
    expect(screen.getByTestId('memory-usage-header')).not.toHaveTextContent('of limit')
  })

  it('renders per-pod bars sorted by usage when multiple pods report metrics', () => {
    render(
      <WorkloadMetricsPanel
        metrics={buildMetrics(10)}
        pods={[
          { name: 'app-1', metrics: { cpu: 0.02, memory: 64 * 1024 * 1024 } },
          { name: 'app-2', metrics: { cpu: 0.05, memory: 32 * 1024 * 1024 } },
        ]}
      />
    )

    const cpuRows = screen.getAllByTestId('cpu-pod-bars-row')
    expect(cpuRows).toHaveLength(2)
    // Sorted descending: the busiest pod first.
    expect(cpuRows[0]).toHaveTextContent('app-2')
    expect(cpuRows[0]).toHaveTextContent('50m')
    expect(cpuRows[1]).toHaveTextContent('app-1')
    expect(cpuRows[1]).toHaveTextContent('20m')

    const memoryRows = screen.getAllByTestId('memory-pod-bars-row')
    expect(memoryRows[0]).toHaveTextContent('app-1')
    expect(memoryRows[0]).toHaveTextContent('64 MiB')
  })

  it('omits the per-pod bars for a single pod', () => {
    render(
      <WorkloadMetricsPanel
        metrics={buildMetrics(10)}
        pods={[{ name: 'app-1', metrics: { cpu: 0.02, memory: 64 } }]}
      />
    )

    expect(screen.queryByTestId('cpu-pod-bars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('memory-pod-bars')).not.toBeInTheDocument()
  })

  it('trims large workloads to the extremes with an elision row', () => {
    const pods = Array.from({ length: 20 }, (_, i) => ({
      name: `app-${String(i).padStart(2, '0')}`,
      metrics: { cpu: (100 - i) / 1000, memory: (100 - i) * 1024 * 1024 },
    }))
    render(<WorkloadMetricsPanel metrics={buildMetrics(10)} pods={pods} />)

    // Top 4 and bottom 2 pods stay visible.
    const cpuRows = screen.getAllByTestId('cpu-pod-bars-row')
    expect(cpuRows).toHaveLength(6)
    expect(cpuRows[0]).toHaveTextContent('app-00')
    expect(cpuRows[3]).toHaveTextContent('app-03')
    expect(cpuRows[4]).toHaveTextContent('app-18')
    expect(cpuRows[5]).toHaveTextContent('app-19')

    // The middle collapses into an aggregate row: count label, muted bar
    // and average value, with the range in the tooltip.
    const elision = screen.getByTestId('cpu-pod-bars-elision')
    expect(elision).toHaveTextContent('+14 pods')
    expect(elision).toHaveTextContent('90m')
    expect(elision).toHaveAttribute('title', '14 pods, 83m – 96m')
    expect(elision.querySelector('.usage-bar-fill-na')).not.toBeNull()
  })

  it('collapses the elision tooltip range when the endpoints format equal', () => {
    const pods = [
      ...Array.from({ length: 4 }, (_, i) => ({
        name: `hot-${i}`,
        metrics: { cpu: 0.5 - i / 100, memory: 1024 },
      })),
      ...Array.from({ length: 4 }, (_, i) => ({
        name: `mid-${i}`,
        metrics: { cpu: 0.1, memory: 1024 },
      })),
      ...Array.from({ length: 2 }, (_, i) => ({
        name: `cold-${i}`,
        metrics: { cpu: 0.01, memory: 1024 },
      })),
    ]
    render(<WorkloadMetricsPanel metrics={buildMetrics(10)} pods={pods} />)

    expect(screen.getByTestId('cpu-pod-bars-elision')).toHaveAttribute('title', '4 pods, 100m each')
  })

  it('collapses many sampleless pods into a collecting row', () => {
    const pods = [
      ...Array.from({ length: 4 }, (_, i) => ({
        name: `app-${i}`,
        metrics: { cpu: 0.01, memory: 1024 },
      })),
      ...Array.from({ length: 8 }, (_, i) => ({ name: `new-${i}` })),
    ]
    render(<WorkloadMetricsPanel metrics={buildMetrics(10)} pods={pods} />)

    expect(screen.getByTestId('cpu-pod-bars-collecting')).toHaveTextContent('collecting metrics for 8 pods')
    expect(screen.getByTestId('cpu-pod-bars-collecting')).toHaveTextContent('N/A')
  })

  it('keeps pods without a usage sample as N/A rows', () => {
    render(
      <WorkloadMetricsPanel
        metrics={buildMetrics(10)}
        pods={[
          { name: 'app-old', metrics: { cpu: 0.02, memory: 64 * 1024 * 1024 } },
          { name: 'app-new' },
        ]}
      />
    )

    const cpuRows = screen.getAllByTestId('cpu-pod-bars-row')
    expect(cpuRows).toHaveLength(2)
    // The sampleless pod stays visible, sorted last, with an N/A value.
    expect(cpuRows[1]).toHaveTextContent('app-new')
    expect(cpuRows[1]).toHaveTextContent('N/A')
    expect(cpuRows[1].querySelector('.usage-bar-track-na')).not.toBeNull()
    expect(cpuRows[1].querySelector('.usage-bar-fill')).toBeNull()
  })

  it('passes the rollout annotation to both charts when inside the window', () => {
    // buildMetrics samples start at 10:00:00Z, one minute apart.
    render(
      <WorkloadMetricsPanel
        metrics={buildMetrics(10)}
        rolledOutAt="2026-07-28T10:05:00Z"
      />
    )

    const expected = `rolled out@${Date.parse('2026-07-28T10:05:00Z') / 1000}`
    expect(screen.getByTestId('cpu-usage-chart')).toHaveAttribute('data-annotation', expected)
    expect(screen.getByTestId('memory-usage-chart')).toHaveAttribute('data-annotation', expected)
  })

  it('omits the rollout annotation when outside the window', () => {
    render(
      <WorkloadMetricsPanel
        metrics={buildMetrics(10)}
        rolledOutAt="2026-07-28T08:00:00Z"
      />
    )

    expect(screen.getByTestId('cpu-usage-chart')).toHaveAttribute('data-annotation', '')
  })
})
