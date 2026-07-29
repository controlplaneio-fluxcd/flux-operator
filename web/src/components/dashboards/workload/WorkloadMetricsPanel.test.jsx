// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { WorkloadMetricsPanel } from './WorkloadMetricsPanel'

// The chart itself renders to canvas (unavailable in jsdom); it has its own
// lifecycle tests with a mocked uPlot, so the panel test stubs it out.
vi.mock('./UsageChart', () => ({
  UsageChart: (props) => <div data-testid={props.testId}>chart</div>
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
})
