// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadsTabContent } from './WorkloadsTabContent'
import { fetchWithMock } from '../../utils/fetch'

// Mock the fetch utility
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

describe('WorkloadsTabContent component', () => {
  const mockWorkloadItems = [
    {
      kind: 'Deployment',
      name: 'podinfo',
      namespace: 'default'
    },
    {
      kind: 'StatefulSet',
      name: 'redis',
      namespace: 'default'
    }
  ]

  const mockDeploymentWorkload = {
    kind: 'Deployment',
    name: 'podinfo',
    namespace: 'default',
    status: 'Current',
    statusMessage: 'Deployment has minimum availability.',
    containerImages: ['ghcr.io/stefanprodan/podinfo:6.0.0'],
    pods: [
      {
        name: 'podinfo-7d8b9c4f5d-abc12',
        status: 'Current',
        statusMessage: 'Pod is running',
        timestamp: '2025-01-15T10:30:00Z'
      },
      {
        name: 'podinfo-7d8b9c4f5d-def34',
        status: 'Current',
        statusMessage: 'Pod is running',
        timestamp: '2025-01-15T10:30:05Z'
      }
    ]
  }

  const mockStatefulSetWorkload = {
    kind: 'StatefulSet',
    name: 'redis',
    namespace: 'default',
    status: 'InProgress',
    statusMessage: 'Waiting for replicas to be ready',
    containerImages: ['redis:7.0'],
    pods: [
      {
        name: 'redis-0',
        status: 'Current',
        statusMessage: 'Pod is running',
        timestamp: '2025-01-15T10:25:00Z'
      },
      {
        name: 'redis-1',
        status: 'InProgress',
        statusMessage: 'Container creating. Reason: ContainerCreating',
        timestamp: '2025-01-15T10:35:00Z'
      }
    ]
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should fetch workload data for each workload item', async () => {
    fetchWithMock.mockResolvedValue(mockDeploymentWorkload)

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(2)
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workload?kind=Deployment&name=podinfo&namespace=default',
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkload'
      })
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workload?kind=StatefulSet&name=redis&namespace=default',
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkload'
      })
    })
  })

  it('should show loading state while fetching workload data', async () => {
    let resolvePromise
    const promise = new Promise((resolve) => { resolvePromise = resolve })
    fetchWithMock.mockReturnValue(promise)

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    // Should show loading spinner
    expect(screen.getByText('Loading workloads...')).toBeInTheDocument()
    expect(document.querySelector('.animate-spin')).toBeInTheDocument()

    // Resolve the promise
    resolvePromise(mockDeploymentWorkload)

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByText('Loading workloads...')).not.toBeInTheDocument()
    })
  })

  it('should display workload list after loading', async () => {
    fetchWithMock
      .mockResolvedValueOnce(mockDeploymentWorkload)
      .mockResolvedValueOnce(mockStatefulSetWorkload)

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('default/podinfo')
      expect(textContent).toContain('default/redis')
    })
  })

  it('should display workload details with status', async () => {
    fetchWithMock
      .mockResolvedValueOnce(mockDeploymentWorkload)
      .mockResolvedValueOnce(mockStatefulSetWorkload)

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('podinfo')
      expect(textContent).toContain('Deployment')
      expect(textContent).toContain('Current')
      expect(textContent).toContain('redis')
      expect(textContent).toContain('StatefulSet')
      expect(textContent).toContain('InProgress')
    })
  })

  it('should expand workload to show container images and pods', async () => {
    fetchWithMock
      .mockResolvedValueOnce(mockDeploymentWorkload)
      .mockResolvedValueOnce(mockStatefulSetWorkload)
    const user = userEvent.setup()

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('default/podinfo')
    })

    // Find and click the podinfo workload to expand it
    const workloadButtons = screen.getAllByRole('button')
    const podinfoButton = workloadButtons.find(btn => btn.textContent.includes('podinfo') && btn.textContent.includes('Deployment'))
    await user.click(podinfoButton)

    // Should show container images and pods
    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('Images')
      expect(textContent).toContain('ghcr.io/stefanprodan/podinfo:6.0.0')
      expect(textContent).toContain('Pods')
      expect(textContent).toContain('podinfo-7d8b9c4f5d-abc12')
      expect(textContent).toContain('podinfo-7d8b9c4f5d-def34')
    })
  })

  it('should display pod status and message', async () => {
    fetchWithMock
      .mockResolvedValueOnce(mockDeploymentWorkload)
      .mockResolvedValueOnce(mockStatefulSetWorkload)
    const user = userEvent.setup()

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('default/podinfo')
    })

    // Expand the podinfo workload
    const workloadButtons = screen.getAllByRole('button')
    const podinfoButton = workloadButtons.find(btn => btn.textContent.includes('podinfo') && btn.textContent.includes('Deployment'))
    await user.click(podinfoButton)

    // Should show pod status messages
    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('Pod is running')
    })
  })

  it('should display status badges with correct colors', async () => {
    fetchWithMock
      .mockResolvedValueOnce(mockDeploymentWorkload)
      .mockResolvedValueOnce(mockStatefulSetWorkload)

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    await waitFor(() => {
      const currentBadge = screen.getAllByText('Current')[0]
      expect(currentBadge).toBeInTheDocument()
      expect(currentBadge.className).toContain('bg-green-100')

      const inProgressBadge = screen.getByText('InProgress')
      expect(inProgressBadge).toBeInTheDocument()
      expect(inProgressBadge.className).toContain('bg-blue-100')
    })
  })

  it('should handle fetch error gracefully', async () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    fetchWithMock.mockRejectedValue(new Error('Network error'))

    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    // Wait for fetch to complete
    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalled()
    })

    // Component should still render, just without complete data
    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('default/podinfo')
    })

    consoleErrorSpy.mockRestore()
  })

  it('should use fallback namespace when workload namespace is missing', async () => {
    const workloadItemsWithoutNamespace = [
      {
        kind: 'Deployment',
        name: 'podinfo'
        // namespace missing
      }
    ]

    fetchWithMock.mockResolvedValue(mockDeploymentWorkload)

    render(
      <WorkloadsTabContent
        workloadItems={workloadItemsWithoutNamespace}
        namespace="custom-namespace"
      />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workload?kind=Deployment&name=podinfo&namespace=custom-namespace',
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkload'
      })
    })
  })

  it('should display status message in workload header', async () => {
    fetchWithMock.mockResolvedValue(mockDeploymentWorkload)

    const singleWorkloadItem = [{
      kind: 'Deployment',
      name: 'podinfo',
      namespace: 'default'
    }]

    render(
      <WorkloadsTabContent
        workloadItems={singleWorkloadItem}
        namespace="default"
      />
    )

    // Should show status message in header (no need to expand)
    await waitFor(() => {
      expect(screen.getByText('Deployment has minimum availability.')).toBeInTheDocument()
    })

    // Should NOT show "Status Message" label (it's not a section anymore)
    expect(screen.queryByText('Status Message')).not.toBeInTheDocument()
  })

  it('should display workload header with uppercase kind, namespace/name, and status badge', async () => {
    fetchWithMock.mockResolvedValue(mockDeploymentWorkload)

    const singleWorkloadItem = [{
      kind: 'Deployment',
      name: 'podinfo',
      namespace: 'default'
    }]

    render(
      <WorkloadsTabContent
        workloadItems={singleWorkloadItem}
        namespace="default"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      // Kind should be present
      expect(textContent).toContain('Deployment')
      // Namespace/Name format
      expect(textContent).toContain('default/podinfo')
      // Status badge
      expect(screen.getByText('Current')).toBeInTheDocument()
      // Status message in header
      expect(textContent).toContain('Deployment has minimum availability.')
    })
  })

  it('should handle workload with no pods', async () => {
    const workloadWithNoPods = {
      ...mockDeploymentWorkload,
      pods: []
    }

    fetchWithMock.mockResolvedValue(workloadWithNoPods)
    const user = userEvent.setup()

    const singleWorkloadItem = [{
      kind: 'Deployment',
      name: 'podinfo',
      namespace: 'default'
    }]

    render(
      <WorkloadsTabContent
        workloadItems={singleWorkloadItem}
        namespace="default"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('default/podinfo')
    })

    // Expand the workload
    const workloadButtons = screen.getAllByRole('button')
    const podinfoButton = workloadButtons.find(btn => btn.textContent.includes('podinfo'))
    await user.click(podinfoButton)

    // Should show "No pods found"
    await waitFor(() => {
      expect(screen.getByText('No pods found')).toBeInTheDocument()
    })
  })

  it('should refetch workload data when workloadItems change', async () => {
    fetchWithMock
      .mockResolvedValueOnce(mockDeploymentWorkload)
      .mockResolvedValueOnce(mockStatefulSetWorkload)

    const { rerender } = render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    // Wait for initial load
    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(2)
    })

    // Update with new workload
    const updatedWorkloadItems = [
      ...mockWorkloadItems,
      {
        kind: 'DaemonSet',
        name: 'logger',
        namespace: 'default'
      }
    ]

    fetchWithMock
      .mockResolvedValueOnce(mockDeploymentWorkload)
      .mockResolvedValueOnce(mockStatefulSetWorkload)
      .mockResolvedValueOnce({ kind: 'DaemonSet', name: 'logger', status: 'Current', pods: [] })

    rerender(
      <WorkloadsTabContent
        workloadItems={updatedWorkloadItems}
        namespace="default"
      />
    )

    // Should refetch all workloads
    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(5) // 2 initial + 3 after rerender
    })
  })
})
