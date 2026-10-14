// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { NodesSummaryPanel } from './NodesSummaryPanel'

const GiB = 1024 ** 3

function summary(overrides = {}) {
  return {
    down: 2,
    pressure: 3,
    cordoned: 1,
    unschedulablePods: 4,
    metricsAvailable: true,
    cpu: { allocatable: 100, requested: 57, used: 36, aboveRequests: 1, limits: 150 },
    memory: { allocatable: 100 * GiB, requested: 61 * GiB, used: 56 * GiB, aboveRequests: 5 * GiB, limits: 120 * GiB },
    pods: { allocatable: 1772, running: 614 },
    findings: {
      critical: 8,
      warn: 9,
      info: 2,
      top: [{ code: 'not-ready', severity: 'critical', nodes: 2 }, { code: 'oom-kills', severity: 'warn', nodes: 1, value: 3 }]
    },
    ...overrides
  }
}

describe('NodesSummaryPanel', () => {
  it('renders nothing without a summary', () => {
    const { container } = render(<NodesSummaryPanel summary={null} canViewNodes />)
    expect(container.firstChild).toBeNull()
  })

  it('renders the node count, facts and non-zero badges', () => {
    render(<NodesSummaryPanel summary={summary({ pressure: 0 })} nodeCount={20} canViewNodes />)
    expect(screen.getByText('Cluster Nodes')).toBeInTheDocument()
    expect(screen.getByText('20 nodes')).toBeInTheDocument()
    expect(screen.getByText('2 down')).toBeInTheDocument()
    expect(screen.getByText('1 cordoned')).toBeInTheDocument()
    expect(screen.queryByText(/under pressure/)).not.toBeInTheDocument()
    expect(screen.getByText('614 / 1772')).toBeInTheDocument()
    expect(screen.getByText('Unschedulable:').nextSibling).toHaveTextContent('4')
    expect(screen.getByText('Critical:').nextSibling).toHaveTextContent('8')
    expect(screen.getByText('Warnings:').nextSibling).toHaveTextContent('9')
  })

  it('renders no badges when all counts are zero', () => {
    render(<NodesSummaryPanel summary={summary({ down: 0, pressure: 0, cordoned: 0 })} nodeCount={1} canViewNodes />)
    expect(screen.getByText('1 node')).toBeInTheDocument()
    expect(document.querySelectorAll('.status-badge')).toHaveLength(0)
  })

  it('links the worst finding to the dashboard for users who can list nodes', () => {
    render(<NodesSummaryPanel summary={summary()} canViewNodes />)
    const status = screen.getByTestId('nodes-summary-status')
    expect(status.tagName).toBe('A')
    expect(status).toHaveAttribute('href', '/nodes')
    expect(status).toHaveTextContent('2 nodes not ready · 16 more issues')
  })

  it('renders the status line as plain text for other users', () => {
    render(<NodesSummaryPanel summary={summary()} canViewNodes={false} />)
    const status = screen.getByTestId('nodes-summary-status')
    expect(status.tagName).toBe('DIV')
    expect(document.querySelector('a[href="/nodes"]')).toBeNull()
  })

  it('shows the healthy line without critical or warn findings', () => {
    render(<NodesSummaryPanel summary={summary({ findings: { critical: 0, warn: 0, info: 2, top: [] } })} canViewNodes />)
    expect(screen.getByTestId('nodes-summary-status')).toHaveTextContent('All nodes healthy')
  })

  it('shows used and requested shares with amounts in the tooltip', () => {
    render(<NodesSummaryPanel summary={summary()} canViewNodes />)
    const cpu = screen.getByTestId('nodes-summary-cpu')
    expect(cpu).toHaveTextContent('36% used')
    expect(cpu).toHaveTextContent('57% requested')
    expect(cpu.getAttribute('title')).toBe('36.0 cores used · 57.0 cores requested · 100.0 cores allocatable')
    expect(cpu.querySelector('.sr-only')).toHaveTextContent(cpu.getAttribute('title'))
    expect(screen.getByTestId('nodes-summary-memory')).toHaveTextContent('56% used')
  })

  it('shows requested shares only without metrics-server', () => {
    const s = summary({ metricsAvailable: false })
    s.cpu = { ...s.cpu, used: null }
    s.memory = { ...s.memory, used: null }
    render(<NodesSummaryPanel summary={s} canViewNodes />)
    expect(screen.getByTestId('nodes-summary-cpu')).toHaveTextContent('57% requested')
    expect(screen.queryByText(/% used/)).not.toBeInTheDocument()
  })

  it('never renders null% when there is no eligible capacity', () => {
    const empty = { allocatable: null, requested: null, used: null, aboveRequests: null, limits: null }
    render(<NodesSummaryPanel summary={summary({ cpu: empty, memory: empty, pods: { allocatable: null, running: null } })} canViewNodes />)
    expect(document.body.textContent).not.toMatch(/null|NaN/)
    expect(screen.queryByTestId('nodes-summary-cpu')).not.toBeInTheDocument()
    expect(screen.getByText('0 / -')).toBeInTheDocument()
  })

  it('collapses and expands', () => {
    render(<NodesSummaryPanel summary={summary()} canViewNodes />)
    const toggle = screen.getByRole('button', { expanded: true })
    fireEvent.click(toggle)
    expect(screen.queryByTestId('nodes-summary-status')).not.toBeInTheDocument()
    fireEvent.click(toggle)
    expect(screen.getByTestId('nodes-summary-status')).toBeInTheDocument()
  })
})
