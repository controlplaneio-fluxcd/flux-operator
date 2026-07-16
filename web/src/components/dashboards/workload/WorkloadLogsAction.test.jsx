// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadLogsAction } from './WorkloadLogsAction'

// Mock the logs viewer to keep these tests on the action button. It resolves the
// pre-selected pod's containers from the live pods prop (mirroring the real viewer,
// so a restart-count change flows through) and exposes buttons for onClose and
// onPodChange.
vi.mock('./WorkloadLogsViewer', () => ({
  WorkloadLogsViewer: ({ namespace, workloadName, pods, initialPodName, onClose, onPodChange }) => {
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
        <button data-testid="logs-viewer-close" onClick={onClose}>close</button>
        {/* Simulate the viewer's own pod selector reporting a switch. */}
        <button data-testid="logs-viewer-switch-pod" onClick={() => onPodChange && onPodChange('app-abc')}>switch</button>
        <button data-testid="logs-viewer-switch-all" onClick={() => onPodChange && onPodChange(null)}>switch all</button>
      </div>
    )
  }
}))

// The deep-link/share-link tests drive window.location; reset it before each test
// so a session opened by one test does not leak into the next via the URL.
beforeEach(() => {
  window.history.replaceState(null, '', '/')
})

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
    // No podStatus: cannot view logs, must be excluded from the viewer's pod list.
    { name: 'app-pending', status: 'Pending' }
  ]

  const defaultProps = {
    kind: 'Deployment',
    namespace: 'flux-system',
    name: 'my-workload',
    pods,
    userActions: ['restart', 'logs'],
    userActionsEnabled: true
  }

  it('shows a disabled button without the logs user action', () => {
    render(
      <WorkloadLogsAction {...defaultProps} userActions={['restart']} userActionsEnabled={true} />
    )
    expect(screen.getByTestId('view-logs-button')).toBeDisabled()
    expect(screen.getByTestId('view-logs-button').parentElement).toHaveAttribute('title', "You don't have permission to view pod logs")
  })

  it('shows authentication tooltip when user actions are disabled', () => {
    render(
      <WorkloadLogsAction {...defaultProps} userActions={[]} userActionsEnabled={false} />
    )
    expect(screen.getByTestId('view-logs-button').parentElement).toHaveAttribute('title', 'Authentication is not configured')
  })

  it('shows the button disabled when there are no inspectable pods (scaled to zero)', () => {
    // A pod without container status cannot be inspected.
    const { unmount } = render(<WorkloadLogsAction {...defaultProps} pods={[{ name: 'p', status: 'Pending' }]} />)
    expect(screen.getByTestId('view-logs-button')).toBeDisabled()
    unmount()

    // An empty pod list (scaled to zero) too.
    render(<WorkloadLogsAction {...defaultProps} pods={[]} />)
    expect(screen.getByTestId('view-logs-button')).toBeDisabled()
  })

  it('does not open the viewer when the disabled button is clicked', () => {
    render(<WorkloadLogsAction {...defaultProps} pods={[]} />)
    fireEvent.click(screen.getByTestId('view-logs-button'))
    expect(screen.queryByTestId('logs-viewer-mock')).not.toBeInTheDocument()
  })

  it('does not deep-link open when there are no pods, leaving the button disabled', () => {
    window.history.replaceState(null, '', '/workload/Deployment/flux-system/my-workload?logs=*')
    render(<WorkloadLogsAction {...defaultProps} pods={[]} />)
    expect(screen.getByTestId('view-logs-button')).toBeDisabled()
    expect(screen.queryByTestId('logs-viewer-mock')).not.toBeInTheDocument()
  })

  it('opens deep link after pods arrive on a later render', async () => {
    window.history.replaceState(null, '', '/workload/Deployment/flux-system/my-workload?logs=*')
    const podsLater = [{
      name: 'app-abc',
      status: 'Running',
      podStatus: { containerStatuses: [{ name: 'main' }] }
    }]
    const { rerender } = render(<WorkloadLogsAction {...defaultProps} pods={[]} />)

    expect(screen.queryByTestId('logs-viewer-mock')).not.toBeInTheDocument()

    rerender(<WorkloadLogsAction {...defaultProps} pods={podsLater} />)

    expect(await screen.findByTestId('logs-viewer-mock')).toBeInTheDocument()
  })

  it('opens the viewer on the All pods view when the button is clicked', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsAction {...defaultProps} />)

    // No dropdown: the button opens the viewer directly.
    expect(screen.queryByTestId('logs-viewer-mock')).not.toBeInTheDocument()
    await user.click(screen.getByTestId('view-logs-button'))

    const viewer = screen.getByTestId('logs-viewer-mock')
    expect(viewer).toHaveTextContent('flux-system/my-workload')
    // No pre-selected pod (All pods); only pods with container status are passed.
    expect(viewer).toHaveAttribute('data-initialpod', '')
    expect(viewer).toHaveAttribute('data-pods', 'app-abc')
    // The All pods session is reflected in the URL.
    expect(new URLSearchParams(window.location.search).get('logs')).toBe('*')
  })

  it('reflects live pod updates (e.g. a container restart) in the open viewer', async () => {
    // Deep-link to a specific pod so the mock can surface its container restart count.
    window.history.replaceState(null, '', '/workload/Deployment/flux-system/my-workload?logs=app-abc')
    const initial = [{
      name: 'app-abc',
      status: 'Running',
      podStatus: { containerStatuses: [{ name: 'main', restartCount: 0 }] }
    }]
    const { rerender } = render(<WorkloadLogsAction {...defaultProps} pods={initial} />)
    expect(await screen.findByTestId('logs-viewer-mock')).toHaveAttribute('data-restarts', '0')

    // The container restarts and the parent polls fresh pod data; the open viewer
    // picks up the new restart count without being reopened.
    const updated = [{
      name: 'app-abc',
      status: 'Running',
      podStatus: { containerStatuses: [{ name: 'main', restartCount: 1 }] }
    }]
    rerender(<WorkloadLogsAction {...defaultProps} pods={updated} />)
    expect(screen.getByTestId('logs-viewer-mock')).toHaveAttribute('data-restarts', '1')
  })

  it('keeps the open viewer mounted and disables the button when the workload scales to zero', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-button'))
    expect(screen.getByTestId('logs-viewer-mock')).toBeInTheDocument()

    // Last pod goes away: viewer stays open (inline error), button disables.
    rerender(<WorkloadLogsAction {...defaultProps} pods={[]} />)
    expect(screen.getByTestId('logs-viewer-mock')).toBeInTheDocument()
    expect(screen.getByTestId('view-logs-button')).toBeDisabled()
  })

  it('deep-links to a pod when loaded with ?logs=<pod>, without clicking the button', async () => {
    window.history.replaceState(null, '', '/workload/Deployment/flux-system/my-workload?logs=app-abc')
    render(<WorkloadLogsAction {...defaultProps} />)

    const viewer = await screen.findByTestId('logs-viewer-mock')
    expect(viewer).toHaveAttribute('data-initialpod', 'app-abc')
  })

  it('deep-links to the All pods view when loaded with ?logs=*', async () => {
    window.history.replaceState(null, '', '/workload/Deployment/flux-system/my-workload?logs=*')
    render(<WorkloadLogsAction {...defaultProps} />)

    const viewer = await screen.findByTestId('logs-viewer-mock')
    expect(viewer).toHaveAttribute('data-initialpod', '')
  })

  it('treats a deep-linked pod name as a pod, never the All pods sentinel', async () => {
    // The All pods sentinel is `*`, which is not a valid pod name, so a real pod
    // name in ?logs never collides with the merged view.
    window.history.replaceState(null, '', '/workload/Deployment/flux-system/my-workload?logs=app-abc')
    render(<WorkloadLogsAction {...defaultProps} />)
    const viewer = await screen.findByTestId('logs-viewer-mock')
    expect(viewer).toHaveAttribute('data-initialpod', 'app-abc')
  })

  it('does not open a viewer on mount without a ?logs param', () => {
    render(<WorkloadLogsAction {...defaultProps} />)
    expect(screen.queryByTestId('logs-viewer-mock')).not.toBeInTheDocument()
  })

  it('keeps the ?logs param in sync when the pod is switched inside the viewer', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-button'))
    expect(new URLSearchParams(window.location.search).get('logs')).toBe('*')

    // Switching to a specific pod inside the viewer rewrites the share link.
    await user.click(screen.getByTestId('logs-viewer-switch-pod'))
    expect(new URLSearchParams(window.location.search).get('logs')).toBe('app-abc')

    // Switching back to All pods rewrites it again.
    await user.click(screen.getByTestId('logs-viewer-switch-all'))
    expect(new URLSearchParams(window.location.search).get('logs')).toBe('*')
  })

  it('clears the ?logs param when the viewer is closed', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-button'))
    expect(new URLSearchParams(window.location.search).get('logs')).toBe('*')

    await user.click(screen.getByTestId('logs-viewer-close'))
    expect(screen.queryByTestId('logs-viewer-mock')).not.toBeInTheDocument()
    expect(new URLSearchParams(window.location.search).get('logs')).toBeNull()
  })

  it('preserves other query params when writing the ?logs param', async () => {
    const user = userEvent.setup()
    window.history.replaceState(null, '', '/workload/Deployment/flux-system/my-workload?tab=pods')
    render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-button'))
    const params = new URLSearchParams(window.location.search)
    expect(params.get('tab')).toBe('pods')
    expect(params.get('logs')).toBe('*')
  })
})
