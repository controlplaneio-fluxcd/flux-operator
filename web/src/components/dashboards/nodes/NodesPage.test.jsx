// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/preact'
import { NodesPage } from './NodesPage'
import { fetchWithMock } from '../../../utils/fetch'
import { POLL_INTERVAL_MS } from '../../../utils/constants'

vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mocked to avoid uPlot canvas rendering in jsdom; covered by CapacityPanel.test.jsx.
vi.mock('./CapacityPanel', () => ({
  CapacityPanel: () => <div data-testid="capacity-panel-mock" />
}))

const snapshot = {
  metricsAvailable: true,
  controlPlaneVersion: 'v1.33.4',
  capacity: { cpu: {}, memory: {}, pods: { allocatable: 110, running: 40 } },
  usage: { samples: [] },
  nodes: [
    { name: 'a.lab', status: 'Unreachable', allocatable: { cpu: 8, memory: 8, pods: 110 }, requests: { cpu: 1, memory: 1 }, findings: [{ code: 'unreachable', severity: 'critical' }] },
    { name: 'b.lab', status: 'Ready', allocatable: { cpu: 8, memory: 8, pods: 110 }, requests: { cpu: 1, memory: 1 }, findings: [{ code: 'oom-kills', severity: 'warn', value: 2 }] }
  ],
  findings: [
    { code: 'unreachable', severity: 'critical', nodes: 1, affected: [{ node: 'a.lab', severity: 'critical' }] },
    { code: 'oom-kills', severity: 'warn', nodes: 1, value: 2, affected: [{ node: 'b.lab', severity: 'warn', value: 2 }] },
    { code: 'kubelet-skew', severity: 'info', value: 2 }
  ],
  checks: [{ name: 'OOM kills', status: 'raised' }, { name: 'Network', status: 'passed', fact: 'available on all nodes' }]
}

const ranking = { metricsAvailable: true, workloads: [] }

function respond({ nodes = snapshot, workloads = ranking } = {}) {
  fetchWithMock.mockImplementation(({ endpoint }) => {
    const r = endpoint === '/api/v1/nodes' ? nodes : workloads
    return r instanceof Error ? Promise.reject(r) : Promise.resolve(r)
  })
}

function httpError(status, message) {
  const err = new Error(message)
  err.status = status
  return err
}

describe('NodesPage', () => {
  beforeEach(() => {
    fetchWithMock.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the header and panels from the snapshot', async () => {
    respond()
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('nodes-hero-title')).toHaveTextContent('Nodes Critical'))
    expect(screen.getByTestId('nodes-hero-subtitle')).toHaveTextContent('2 nodes · 1 critical · 1 warning')
    expect(screen.getByTestId('capacity-panel-mock')).toBeInTheDocument()
    expect(screen.getByText('Health Checks')).toBeInTheDocument()
    expect(screen.getByText('Nodes')).toBeInTheDocument()
    expect(screen.getByText('Bursting Workloads')).toBeInTheDocument()
    // Short names: the shared .lab suffix is stripped.
    expect(screen.getAllByText('a').length).toBeGreaterThan(0)
  })

  it('reads healthy when no critical or warn lines are raised', async () => {
    respond({ nodes: { ...snapshot, findings: [{ code: 'cordoned', severity: 'info', nodes: 1 }] } })
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('nodes-hero-title')).toHaveTextContent('Nodes Healthy'))
    expect(screen.getByTestId('nodes-hero-subtitle')).toHaveTextContent(/^2 nodes$/)
  })

  it('shows the access-restricted message on 403', async () => {
    respond({ nodes: httpError(403, 'forbidden'), workloads: httpError(403, 'forbidden') })
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('nodes-access-denied')).toBeInTheDocument())
    expect(screen.getByText('Requires permission to list nodes.')).toBeInTheDocument()
    expect(screen.queryByTestId('error-message')).not.toBeInTheDocument()
  })

  it('shows other errors as errors, not as access denied', async () => {
    respond({ nodes: new TypeError('Failed to fetch') })
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('error-message')).toHaveTextContent('Failed to load nodes: Failed to fetch'))
    expect(screen.queryByTestId('nodes-access-denied')).not.toBeInTheDocument()
  })

  it('treats HTTP errors other than 403 as errors', async () => {
    respond({ nodes: httpError(500, 'HTTP error! status: 500, error: boom') })
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('error-message')).toHaveTextContent('status: 500'))
  })

  it('shows the workloads panel as failed when its fetch fails', async () => {
    respond({ workloads: httpError(500, 'boom') })
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByText('Failed to load workloads')).toBeInTheDocument())
    expect(screen.getByTestId('nodes-hero-title')).toBeInTheDocument()
  })

  it('replaces stale workload rows with the failed state on a failed poll', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    respond({ workloads: { metricsAvailable: true, workloads: [{ kind: 'Deployment', namespace: 'apps', name: 'api', usage: 2, requests: 1, aboveRequests: 1 }] } })
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('bursting-workload-row')).toBeInTheDocument())

    respond({ workloads: new TypeError('Failed to fetch') })
    await act(async () => {
      vi.advanceTimersByTime(POLL_INTERVAL_MS)
    })
    await waitFor(() => expect(screen.getByText('Failed to load workloads')).toBeInTheDocument())
    expect(screen.queryByTestId('bursting-workload-row')).not.toBeInTheDocument()
  })

  it('does not wait for a hung workloads request to show the nodes', async () => {
    fetchWithMock.mockImplementation(({ endpoint }) =>
      endpoint === '/api/v1/nodes' ? Promise.resolve(snapshot) : new Promise(() => {}))
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('nodes-hero-title')).toHaveTextContent('Nodes Critical'))
    expect(screen.queryByText('Bursting Workloads')).not.toBeInTheDocument()
  })

  it('shows the real error when a 403 is followed by another failure, and clears both on success', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    respond({ nodes: httpError(403, 'forbidden') })
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('nodes-access-denied')).toBeInTheDocument())

    respond({ nodes: httpError(500, 'HTTP error! status: 500, error: boom') })
    await act(async () => {
      vi.advanceTimersByTime(POLL_INTERVAL_MS)
    })
    await waitFor(() => expect(screen.getByTestId('error-message')).toHaveTextContent('status: 500'))
    expect(screen.queryByTestId('nodes-access-denied')).not.toBeInTheDocument()

    respond()
    await act(async () => {
      vi.advanceTimersByTime(POLL_INTERVAL_MS)
    })
    await waitFor(() => expect(screen.getByTestId('nodes-hero-title')).toBeInTheDocument())
    expect(screen.queryByTestId('error-message')).not.toBeInTheDocument()
    expect(screen.queryByTestId('nodes-access-denied')).not.toBeInTheDocument()
  })

  it('polls both endpoints on the dashboard interval and keeps data on a failed poll', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    respond()
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('nodes-hero-title')).toBeInTheDocument())
    expect(fetchWithMock).toHaveBeenCalledTimes(2)
    const endpoints = fetchWithMock.mock.calls.map(c => c[0].endpoint)
    expect(endpoints).toEqual(['/api/v1/nodes', '/api/v1/nodes/workloads'])

    respond({ nodes: new TypeError('Failed to fetch') })
    await act(async () => {
      vi.advanceTimersByTime(POLL_INTERVAL_MS)
    })
    await waitFor(() => expect(screen.getByTestId('refresh-error')).toHaveTextContent('Failed to refresh nodes: Failed to fetch'))
    expect(screen.getByTestId('nodes-hero-title')).toBeInTheDocument()
    expect(screen.queryByTestId('error-message')).not.toBeInTheDocument()

    respond()
    await act(async () => {
      vi.advanceTimersByTime(POLL_INTERVAL_MS)
    })
    await waitFor(() => expect(screen.queryByTestId('refresh-error')).not.toBeInTheDocument())
    expect(screen.getByTestId('nodes-hero-title')).toBeInTheDocument()
  })

  it('switches to access restricted on a 403 after data has loaded', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    respond()
    render(<NodesPage />)
    await waitFor(() => expect(screen.getByTestId('nodes-hero-title')).toBeInTheDocument())

    respond({ nodes: httpError(403, 'forbidden') })
    await act(async () => {
      vi.advanceTimersByTime(POLL_INTERVAL_MS)
    })
    await waitFor(() => expect(screen.getByTestId('nodes-access-denied')).toBeInTheDocument())
    expect(screen.queryByTestId('nodes-hero-title')).not.toBeInTheDocument()
    expect(screen.queryByTestId('refresh-error')).not.toBeInTheDocument()
  })

  it('stops polling on unmount', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    respond()
    const { unmount } = render(<NodesPage />)
    await waitFor(() => expect(fetchWithMock).toHaveBeenCalledTimes(2))
    unmount()
    await act(async () => {
      vi.advanceTimersByTime(POLL_INTERVAL_MS * 2)
    })
    expect(fetchWithMock).toHaveBeenCalledTimes(2)
  })
})
