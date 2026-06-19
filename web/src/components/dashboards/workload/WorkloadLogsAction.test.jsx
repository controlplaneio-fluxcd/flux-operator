// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadLogsAction } from './WorkloadLogsAction'

// Mock the logs viewer so these tests stay focused on the dropdown. It resolves
// the pre-selected pod's containers from the live pods prop, mirroring how the
// real viewer reads them (so a restart-count change flows through).
vi.mock('./WorkloadLogsViewer', () => ({
  WorkloadLogsViewer: ({ namespace, workloadName, pods, initialPodName }) => {
    const sel = (pods || []).find(p => p.name === initialPodName)
    const containers = sel ? sel.containers : []
    return (
      <div
        data-testid="logs-viewer-mock"
        data-initialpod={initialPodName || ''}
        data-pods={(pods || []).map(p => p.name).join(',')}
        data-containers={containers.map(c => c.name).join(',')}
        data-restarts={containers.map(c => c.restartCount).join(',')}
      >
        {namespace}/{workloadName}
      </div>
    )
  }
}))

describe('WorkloadLogsAction component', () => {
  const pods = [
    {
      name: 'app-abc',
      status: 'Running',
      podStatus: {
        initContainerStatuses: [{ name: 'init' }],
        containerStatuses: [{ name: 'main' }]
      }
    },
    // No podStatus: cannot view logs, must be excluded from the list.
    { name: 'app-pending', status: 'Pending' }
  ]

  const defaultProps = {
    kind: 'Deployment',
    namespace: 'flux-system',
    name: 'my-workload',
    pods,
    userActions: ['restart', 'logs']
  }

  it('renders nothing without the logs user action', () => {
    const { container } = render(
      <WorkloadLogsAction {...defaultProps} userActions={['restart']} />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when no pod has container status', () => {
    const { container } = render(
      <WorkloadLogsAction {...defaultProps} pods={[{ name: 'p', status: 'Pending' }]} />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('lists only pods with container status', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-dropdown-button'))
    expect(screen.getByTestId('view-logs-pod-app-abc')).toBeInTheDocument()
    expect(screen.queryByTestId('view-logs-pod-app-pending')).not.toBeInTheDocument()
  })

  it('opens the viewer pre-selected on the chosen pod, with its containers (init first)', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-dropdown-button'))
    await user.click(screen.getByTestId('view-logs-pod-app-abc'))

    const viewer = screen.getByTestId('logs-viewer-mock')
    expect(viewer).toHaveTextContent('flux-system/my-workload')
    expect(viewer).toHaveAttribute('data-initialpod', 'app-abc')
    expect(viewer).toHaveAttribute('data-containers', 'init,main')
  })

  it('opens the "All pods" view (no pre-selected pod) from the All pods item', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-dropdown-button'))
    await user.click(screen.getByTestId('view-logs-all-pods'))

    const viewer = screen.getByTestId('logs-viewer-mock')
    expect(viewer).toHaveAttribute('data-initialpod', '')
    // Only pods with container status are passed to the viewer.
    expect(viewer).toHaveAttribute('data-pods', 'app-abc')
  })

  it('reflects live pod updates (e.g. a container restart) in the open viewer', async () => {
    const user = userEvent.setup()
    const initial = [{
      name: 'app-abc',
      status: 'Running',
      podStatus: { containerStatuses: [{ name: 'main', restartCount: 0 }] }
    }]
    const { rerender } = render(<WorkloadLogsAction {...defaultProps} pods={initial} />)

    await user.click(screen.getByTestId('view-logs-dropdown-button'))
    await user.click(screen.getByTestId('view-logs-pod-app-abc'))
    expect(screen.getByTestId('logs-viewer-mock')).toHaveAttribute('data-restarts', '0')

    // The container restarts and the parent polls fresh pod data; the open
    // viewer picks up the new restart count without being reopened.
    const updated = [{
      name: 'app-abc',
      status: 'Running',
      podStatus: { containerStatuses: [{ name: 'main', restartCount: 1 }] }
    }]
    rerender(<WorkloadLogsAction {...defaultProps} pods={updated} />)
    expect(screen.getByTestId('logs-viewer-mock')).toHaveAttribute('data-restarts', '1')
  })

  it('re-initialises the viewer on each open (All pods, then a specific pod)', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-dropdown-button'))
    await user.click(screen.getByTestId('view-logs-all-pods'))
    expect(screen.getByTestId('logs-viewer-mock')).toHaveAttribute('data-initialpod', '')

    // Reopening on a specific pod remounts the viewer pre-selected on that pod
    // (the session key changes), rather than keeping the prior "All pods" session.
    await user.click(screen.getByTestId('view-logs-dropdown-button'))
    await user.click(screen.getByTestId('view-logs-pod-app-abc'))
    expect(screen.getByTestId('logs-viewer-mock')).toHaveAttribute('data-initialpod', 'app-abc')
  })

  it('closes the dropdown on Escape', async () => {
    render(<WorkloadLogsAction {...defaultProps} />)

    fireEvent.click(screen.getByTestId('view-logs-dropdown-button'))
    expect(screen.getByTestId('view-logs-dropdown-menu')).toBeInTheDocument()

    fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByTestId('view-logs-dropdown-menu')).not.toBeInTheDocument())
  })
})
