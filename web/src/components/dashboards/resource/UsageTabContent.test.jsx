// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { UsageTabContent } from './UsageTabContent'

// The chart itself renders to canvas (unavailable in jsdom); it has its own
// lifecycle tests with a mocked uPlot, so this test stubs it out.
vi.mock('../workload/UsageChart', () => ({
  UsageChart: (props) => (
    <div
      data-testid={props.testId}
      data-annotation={props.annotation ? `${props.annotation.label}@${props.annotation.time}` : ''}
    >
      chart
    </div>
  )
}))

// Build workload samples with the given per-sample cpu/memory values.
function buildSamples(count, cpu, memory) {
  const samples = []
  for (let i = 0; i < count; i++) {
    samples.push({
      t: new Date(Date.parse('2026-07-28T10:00:00Z') + i * 60000).toISOString(),
      cpu,
      memory,
    })
  }
  return samples
}

const kustomization = {
  kind: 'Kustomization',
  metadata: { name: 'apps', namespace: 'flux-system' },
}

describe('UsageTabContent component', () => {
  it('shows the loading state until the first fetch resolves', () => {
    render(<UsageTabContent resourceData={kustomization} workloads={null} />)
    expect(screen.getByTestId('usage-tab-loading')).toHaveTextContent('Loading usage data')
  })

  it('shows the error state when the fetch failed before any data arrived', () => {
    render(<UsageTabContent resourceData={kustomization} workloads={null} error />)
    expect(screen.getByTestId('usage-tab-error')).toHaveTextContent('Failed to load usage data')
    expect(screen.queryByTestId('usage-tab-loading')).not.toBeInTheDocument()
  })

  it('keeps showing fetched data when a later fetch fails', () => {
    const workloads = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', status: 'Current', samples: buildSamples(10, 0.1, 1024) },
    ]
    render(<UsageTabContent resourceData={kustomization} workloads={workloads} error />)
    expect(screen.queryByTestId('usage-tab-error')).not.toBeInTheDocument()
    expect(screen.getAllByTestId('cpu-pod-bars-row')).toHaveLength(1)
  })

  it('shows the empty state when no workload has samples', () => {
    const workloads = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', status: 'Current' },
    ]
    render(<UsageTabContent resourceData={kustomization} workloads={workloads} />)
    expect(screen.getByTestId('usage-tab-empty')).toHaveTextContent('No usage data available')
  })

  it('sums the workload series into the aggregate headline', () => {
    const workloads = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', status: 'Current', samples: buildSamples(10, 0.1, 100 * 1024 * 1024) },
      { kind: 'Deployment', namespace: 'apps', name: 'backend', status: 'Current', samples: buildSamples(10, 0.2, 50 * 1024 * 1024) },
    ]
    render(<UsageTabContent resourceData={kustomization} workloads={workloads} />)

    expect(screen.getByTestId('cpu-usage-header')).toHaveTextContent('300m')
    expect(screen.getByTestId('memory-usage-header')).toHaveTextContent('150 MiB')
    // No requests/limits at resource level: absolute values only.
    expect(screen.queryByText(/of request/)).not.toBeInTheDocument()
    expect(screen.queryByText(/of limit/)).not.toBeInTheDocument()
    // One navigable bar row per workload, sorted by usage.
    const rows = screen.getAllByTestId('cpu-pod-bars-row')
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('apps/backend')
    expect(rows[0]).toHaveAttribute('href', '/workload/Deployment/apps/backend')
    expect(rows[1]).toHaveTextContent('apps/frontend')
  })

  it('excludes NotFound sentinel entries from the rows and the aggregate', () => {
    const workloads = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', status: 'Current', samples: buildSamples(10, 0.1, 100 * 1024 * 1024) },
      { kind: 'Deployment', namespace: 'apps', name: 'hidden', status: 'NotFound', statusMessage: 'User does not have access to the workload' },
    ]
    render(<UsageTabContent resourceData={kustomization} workloads={workloads} />)

    expect(screen.getAllByTestId('cpu-pod-bars-row')).toHaveLength(1)
    expect(screen.queryByText('apps/hidden')).not.toBeInTheDocument()
    expect(screen.getByTestId('cpu-usage-header')).toHaveTextContent('100m')
  })

  it('keeps sampleless workloads as N/A rows', () => {
    const workloads = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', status: 'Current', samples: buildSamples(10, 0.1, 1024) },
      { kind: 'Deployment', namespace: 'apps', name: 'quiet', status: 'Current' },
    ]
    render(<UsageTabContent resourceData={kustomization} workloads={workloads} />)

    const rows = screen.getAllByTestId('cpu-pod-bars-row')
    expect(rows).toHaveLength(2)
    expect(rows[1]).toHaveTextContent('apps/quiet')
    expect(rows[1]).toHaveTextContent('N/A')
  })

  it('draws the reconciled marker when the history timestamp is inside the window', () => {
    const resource = {
      ...kustomization,
      status: { history: [{ lastReconciled: '2026-07-28T10:05:00Z' }] },
    }
    const workloads = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', status: 'Current', samples: buildSamples(10, 0.1, 1024) },
    ]
    render(<UsageTabContent resourceData={resource} workloads={workloads} />)

    const expected = `reconciled@${Date.parse('2026-07-28T10:05:00Z') / 1000}`
    expect(screen.getByTestId('cpu-usage-chart')).toHaveAttribute('data-annotation', expected)
    expect(screen.getByTestId('memory-usage-chart')).toHaveAttribute('data-annotation', expected)
  })

  it('omits the reconciled marker when the history timestamp is outside the window', () => {
    const resource = {
      ...kustomization,
      status: { history: [{ lastReconciled: '2026-07-28T08:00:00Z' }] },
    }
    const workloads = [
      { kind: 'Deployment', namespace: 'apps', name: 'frontend', status: 'Current', samples: buildSamples(10, 0.1, 1024) },
    ]
    render(<UsageTabContent resourceData={resource} workloads={workloads} />)

    expect(screen.getByTestId('cpu-usage-chart')).toHaveAttribute('data-annotation', '')
  })

  it('uses lastDeployed for HelmRelease history entries', () => {
    const resource = {
      kind: 'HelmRelease',
      metadata: { name: 'nginx', namespace: 'default' },
      status: { history: [{ lastDeployed: '2026-07-28T10:03:00Z' }] },
    }
    const workloads = [
      { kind: 'Deployment', namespace: 'default', name: 'nginx', status: 'Current', samples: buildSamples(10, 0.1, 1024) },
    ]
    render(<UsageTabContent resourceData={resource} workloads={workloads} />)

    const expected = `reconciled@${Date.parse('2026-07-28T10:03:00Z') / 1000}`
    expect(screen.getByTestId('cpu-usage-chart')).toHaveAttribute('data-annotation', expected)
  })
})
