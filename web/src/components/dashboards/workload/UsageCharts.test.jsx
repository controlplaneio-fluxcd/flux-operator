// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { UsageCharts } from './UsageCharts'

// The chart itself renders to canvas (unavailable in jsdom); it has its own
// lifecycle tests with a mocked uPlot, so this test stubs it out.
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
function buildMetrics(count) {
  const samples = []
  for (let i = 0; i < count; i++) {
    samples.push({
      t: new Date(Date.parse('2026-07-28T10:00:00Z') + i * 60000).toISOString(),
      cpu: 0.1 + i * 0.01,
      memory: (100 + i) * 1024 * 1024,
    })
  }
  return { samples }
}

describe('UsageCharts component', () => {
  it('renders every row without trim, with no elision or collecting rows', () => {
    const items = Array.from({ length: 20 }, (_, i) => ({
      kind: 'Deployment',
      namespace: 'apps',
      name: `app-${String(i).padStart(2, '0')}`,
      metrics: { cpu: (100 - i) / 1000, memory: (100 - i) * 1024 * 1024 },
    }))
    render(<UsageCharts metrics={buildMetrics(10)} items={items} />)

    expect(screen.getAllByTestId('cpu-pod-bars-row')).toHaveLength(20)
    expect(screen.queryByTestId('cpu-pod-bars-elision')).not.toBeInTheDocument()
    expect(screen.queryByTestId('cpu-pod-bars-collecting')).not.toBeInTheDocument()
    expect(screen.getAllByTestId('memory-pod-bars-row')).toHaveLength(20)
  })

  it('trims large lists to the row budget when trim is set', () => {
    const items = Array.from({ length: 20 }, (_, i) => ({
      name: `app-${String(i).padStart(2, '0')}`,
      metrics: { cpu: (100 - i) / 1000, memory: (100 - i) * 1024 * 1024 },
    }))
    render(<UsageCharts metrics={buildMetrics(10)} items={items} trim />)

    expect(screen.getAllByTestId('cpu-pod-bars-row')).toHaveLength(5)
    expect(screen.getByTestId('cpu-pod-bars-elision')).toHaveTextContent('+15 pods')
  })

  it('renders workload rows as links to their dashboard with the kind in the label', () => {
    const items = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', metrics: { cpu: 0.05, memory: 64 * 1024 * 1024 } },
      { kind: 'StatefulSet', namespace: 'data', name: 'frontend', metrics: { cpu: 0.02, memory: 32 * 1024 * 1024 } },
    ]
    render(<UsageCharts metrics={buildMetrics(10)} items={items} />)

    const rows = screen.getAllByTestId('cpu-pod-bars-row')
    expect(rows).toHaveLength(2)
    // Same-name workloads render as distinct rows with distinct hrefs,
    // labelled namespace/name with the kind in the tooltip and the kind
    // and usage value in the accessible name (the aria-label overrides
    // the descendant text).
    expect(rows[0]).toHaveAttribute('href', '/workload/Deployment/apps/frontend')
    expect(rows[0]).toHaveAttribute('aria-label', 'Deployment apps/frontend: 50m')
    expect(rows[0]).toHaveTextContent('apps/frontend')
    expect(rows[1]).toHaveAttribute('href', '/workload/StatefulSet/data/frontend')
    expect(rows[1]).toHaveAttribute('aria-label', 'StatefulSet data/frontend: 20m')
  })

  it('renders sampleless workload rows as navigable N/A rows', () => {
    const items = [
      { kind: 'Deployment', namespace: 'apps', name: 'busy', metrics: { cpu: 0.05, memory: 1024 } },
      { kind: 'Deployment', namespace: 'apps', name: 'quiet' },
    ]
    render(<UsageCharts metrics={buildMetrics(10)} items={items} />)

    const rows = screen.getAllByTestId('cpu-pod-bars-row')
    expect(rows[1]).toHaveTextContent('apps/quiet')
    expect(rows[1]).toHaveTextContent('N/A')
    expect(rows[1]).toHaveAttribute('href', '/workload/Deployment/apps/quiet')
    expect(rows[1]).toHaveAttribute('aria-label', 'Deployment apps/quiet: N/A')
    expect(rows[1].querySelector('.usage-bar-track-na')).not.toBeNull()
  })

  it('shows no percent text or severity classes for rows without limits', () => {
    const items = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', metrics: { cpu: 0.09, memory: 64 * 1024 * 1024 } },
    ]
    render(<UsageCharts metrics={buildMetrics(10)} items={items} />)

    expect(screen.queryByText(/of request/)).not.toBeInTheDocument()
    expect(screen.queryByText(/of limit/)).not.toBeInTheDocument()
    const row = screen.getAllByTestId('cpu-pod-bars-row')[0]
    expect(row.querySelector('.usage-bar-fill-cpu')).not.toBeNull()
    expect(row.querySelector('.usage-bar-fill-critical')).toBeNull()
    expect(row.querySelector('.usage-bar-fill-warn')).toBeNull()
  })

  it('keeps pod rows without identity as plain non-link rows', () => {
    const items = [
      { name: 'pod-a', metrics: { cpu: 0.05, memory: 1024 } },
    ]
    render(<UsageCharts metrics={buildMetrics(10)} items={items} />)

    const row = screen.getAllByTestId('cpu-pod-bars-row')[0]
    expect(row.tagName).toBe('DIV')
    expect(row).not.toHaveAttribute('href')
  })
})
