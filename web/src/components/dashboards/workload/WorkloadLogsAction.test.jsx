// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadLogsAction } from './WorkloadLogsAction'

// Mock the logs viewer so these tests stay focused on the dropdown.
vi.mock('./WorkloadLogsViewer', () => ({
  WorkloadLogsViewer: ({ namespace, name, containers }) => (
    <div data-testid="logs-viewer-mock" data-containers={(containers || []).map(c => c.name).join(',')}>
      {namespace}/{name}
    </div>
  )
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
    namespace: 'flux-system',
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

  it('opens the viewer for the selected pod with its containers (init first)', async () => {
    const user = userEvent.setup()
    render(<WorkloadLogsAction {...defaultProps} />)

    await user.click(screen.getByTestId('view-logs-dropdown-button'))
    await user.click(screen.getByTestId('view-logs-pod-app-abc'))

    const viewer = screen.getByTestId('logs-viewer-mock')
    expect(viewer).toHaveTextContent('flux-system/app-abc')
    expect(viewer).toHaveAttribute('data-containers', 'init,main')
  })

  it('closes the dropdown on Escape', async () => {
    render(<WorkloadLogsAction {...defaultProps} />)

    fireEvent.click(screen.getByTestId('view-logs-dropdown-button'))
    expect(screen.getByTestId('view-logs-dropdown-menu')).toBeInTheDocument()

    fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByTestId('view-logs-dropdown-menu')).not.toBeInTheDocument())
  })
})
