// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadsTabContent } from './WorkloadsTabContent'
import { fetchWithMock } from '../../../utils/fetch'

// Mock the fetch utility
vi.mock('../../../utils/fetch', () => ({
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
        createdAt: '2025-01-15T10:30:00Z'
      },
      {
        name: 'podinfo-7d8b9c4f5d-def34',
        status: 'Current',
        statusMessage: 'Pod is running',
        createdAt: '2025-01-15T10:30:05Z'
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
        createdAt: '2025-01-15T10:25:00Z'
      },
      {
        name: 'redis-1',
        status: 'InProgress',
        statusMessage: 'Container creating. Reason: ContainerCreating',
        createdAt: '2025-01-15T10:35:00Z'
      }
    ]
  }

  beforeEach(() => {
    vi.clearAllMocks()
    // Default mock response for batch endpoint
    fetchWithMock.mockImplementation(({ body }) => {
      const workloads = body?.workloads || []
      const results = workloads.map(w => {
        if (w.kind === 'Deployment' && w.name === 'podinfo') return mockDeploymentWorkload
        if (w.kind === 'StatefulSet' && w.name === 'redis') return mockStatefulSetWorkload
        return { ...w, status: 'NotFound', statusMessage: 'Workload not found' }
      })
      return Promise.resolve({ workloads: results })
    })
  })

  it('should fetch workload data with single POST request', async () => {
    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(1)
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workloads',
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkloads',
        method: 'POST',
        body: {
          workloads: [
            { kind: 'Deployment', name: 'podinfo', namespace: 'default' },
            { kind: 'StatefulSet', name: 'redis', namespace: 'default' }
          ]
        }
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
    resolvePromise({ workloads: [mockDeploymentWorkload, mockStatefulSetWorkload] })

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByText('Loading workloads...')).not.toBeInTheDocument()
    })
  })

  it('should display workload list after loading', async () => {
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
      expect(textContent).toContain('Ready')
      expect(textContent).toContain('redis')
      expect(textContent).toContain('StatefulSet')
      expect(textContent).toContain('Progressing')
    })
  })

  it('should expand workload to show container images and pods', async () => {
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
    render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    await waitFor(() => {
      const readyBadge = screen.getAllByText('Ready')[0]
      expect(readyBadge).toBeInTheDocument()
      expect(readyBadge.className).toContain('bg-green-100')

      const progressingBadge = screen.getByText('Progressing')
      expect(progressingBadge).toBeInTheDocument()
      expect(progressingBadge.className).toContain('bg-blue-100')
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

    render(
      <WorkloadsTabContent
        workloadItems={workloadItemsWithoutNamespace}
        namespace="custom-namespace"
      />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workloads',
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkloads',
        method: 'POST',
        body: {
          workloads: [
            { kind: 'Deployment', name: 'podinfo', namespace: 'custom-namespace' }
          ]
        }
      })
    })
  })

  it('should display status message in workload header', async () => {
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
      expect(screen.getByText('Ready')).toBeInTheDocument()
      // Status message in header
      expect(textContent).toContain('Deployment has minimum availability.')
    })
  })

  it('should handle workload with no pods', async () => {
    const workloadWithNoPods = {
      ...mockDeploymentWorkload,
      pods: []
    }

    fetchWithMock.mockImplementation(() =>
      Promise.resolve({ workloads: [workloadWithNoPods] })
    )
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

  it('should show "No recent jobs" for CronJob with no pods', async () => {
    const cronJobWorkload = {
      kind: 'CronJob',
      name: 'backup',
      namespace: 'default',
      status: 'Idle',
      statusMessage: '0 */6 * * *',
      containerImages: ['busybox:1.36'],
      pods: []
    }

    fetchWithMock.mockImplementation(() =>
      Promise.resolve({ workloads: [cronJobWorkload] })
    )
    const user = userEvent.setup()

    const cronJobItem = [{
      kind: 'CronJob',
      name: 'backup',
      namespace: 'default'
    }]

    render(
      <WorkloadsTabContent
        workloadItems={cronJobItem}
        namespace="default"
      />
    )

    await waitFor(() => {
      const textContent = document.body.textContent
      expect(textContent).toContain('default/backup')
    })

    // Expand the workload
    const workloadButtons = screen.getAllByRole('button')
    const backupButton = workloadButtons.find(btn => btn.textContent.includes('backup'))
    await user.click(backupButton)

    // Should show "No recent jobs" for CronJob
    await waitFor(() => {
      expect(screen.getByText('No recent jobs')).toBeInTheDocument()
    })
  })

  it('should refetch workload data when workloadItems change', async () => {
    const { rerender } = render(
      <WorkloadsTabContent
        workloadItems={mockWorkloadItems}
        namespace="default"
      />
    )

    // Wait for initial load
    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(1)
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

    rerender(
      <WorkloadsTabContent
        workloadItems={updatedWorkloadItems}
        namespace="default"
      />
    )

    // Should refetch all workloads with single POST request
    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(2) // 1 initial + 1 after rerender
    })
  })

  describe('Recent pod highlighting', () => {
    it('should highlight pods with timestamps within the last 30 seconds', async () => {
      const user = userEvent.setup()
      const now = new Date()
      const recentTimestamp = new Date(now.getTime() - 10000).toISOString() // 10 seconds ago

      const workloadWithRecentPod = {
        ...mockDeploymentWorkload,
        pods: [
          {
            name: 'podinfo-recent-pod',
            status: 'Current',
            statusMessage: 'Pod is running',
            createdAt: recentTimestamp
          }
        ]
      }

      fetchWithMock.mockImplementation(() =>
        Promise.resolve({ workloads: [workloadWithRecentPod] })
      )

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

      // Should have the recent-pod data-testid
      await waitFor(() => {
        expect(screen.getByTestId('recent-pod')).toBeInTheDocument()
      })

      // Should have the ring highlight class
      const recentPod = screen.getByTestId('recent-pod')
      expect(recentPod.className).toContain('ring-2')
      expect(recentPod.className).toContain('ring-blue-400')
    })

    it('should not highlight pods with timestamps older than 30 seconds', async () => {
      const user = userEvent.setup()
      const oldTimestamp = new Date(Date.now() - 60000).toISOString() // 60 seconds ago

      const workloadWithOldPod = {
        ...mockDeploymentWorkload,
        pods: [
          {
            name: 'podinfo-old-pod',
            status: 'Current',
            statusMessage: 'Pod is running',
            createdAt: oldTimestamp
          }
        ]
      }

      fetchWithMock.mockImplementation(() =>
        Promise.resolve({ workloads: [workloadWithOldPod] })
      )

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

      // Should NOT have the recent-pod data-testid
      await waitFor(() => {
        expect(screen.getByText('podinfo-old-pod')).toBeInTheDocument()
      })
      expect(screen.queryByTestId('recent-pod')).not.toBeInTheDocument()
    })

    it('should not highlight pods without timestamps', async () => {
      const user = userEvent.setup()

      const workloadWithNoTimestampPod = {
        ...mockDeploymentWorkload,
        pods: [
          {
            name: 'podinfo-no-timestamp',
            status: 'Current',
            statusMessage: 'Pod is running'
            // No timestamp
          }
        ]
      }

      fetchWithMock.mockImplementation(() =>
        Promise.resolve({ workloads: [workloadWithNoTimestampPod] })
      )

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

      // Should NOT have the recent-pod data-testid
      await waitFor(() => {
        expect(screen.getByText('podinfo-no-timestamp')).toBeInTheDocument()
      })
      expect(screen.queryByTestId('recent-pod')).not.toBeInTheDocument()
    })
  })

  describe('Delete pod button', () => {
    it('should show delete button when canDeletePods is true', async () => {
      const user = userEvent.setup()

      const workloadWithDeletePods = {
        ...mockDeploymentWorkload,
        canDeletePods: true
      }

      fetchWithMock.mockImplementation(() =>
        Promise.resolve({ workloads: [workloadWithDeletePods] })
      )

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

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-pod-button').length).toBeGreaterThan(0)
      })
    })

    it('should not show delete button when canDeletePods is false', async () => {
      const user = userEvent.setup()

      const workloadWithoutDeletePods = {
        ...mockDeploymentWorkload,
        canDeletePods: false
      }

      fetchWithMock.mockImplementation(() =>
        Promise.resolve({ workloads: [workloadWithoutDeletePods] })
      )

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

      await waitFor(() => {
        expect(screen.getByText('podinfo-7d8b9c4f5d-abc12')).toBeInTheDocument()
      })
      expect(screen.queryByTestId('delete-pod-button')).not.toBeInTheDocument()
    })

    it('should show Terminating badge when pod delete is pending', async () => {
      const user = userEvent.setup()

      const workloadWithDeletePods = {
        ...mockDeploymentWorkload,
        canDeletePods: true
      }

      fetchWithMock.mockImplementation(({ endpoint, body }) => {
        // Handle the workloads fetch
        if (endpoint === '/api/v1/workloads' || body?.workloads) {
          return Promise.resolve({ workloads: [workloadWithDeletePods] })
        }
        // Handle the delete action
        return Promise.resolve({ success: true, message: 'Pod deleted' })
      })

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

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-pod-button').length).toBeGreaterThan(0)
      })

      // Click the first delete button and confirm
      vi.spyOn(window, 'confirm').mockReturnValue(true)
      const deleteButtons = screen.getAllByTestId('delete-pod-button')
      await user.click(deleteButtons[0])

      // The pod badge should now show Terminating with yellow styling
      await waitFor(() => {
        const terminatingBadges = screen.getAllByText('Terminating')
        expect(terminatingBadges.length).toBeGreaterThan(0)
        expect(terminatingBadges[0].className).toContain('bg-yellow-100')
      })

      window.confirm.mockRestore()
    })

    it('should not show delete button when canDeletePods is missing', async () => {
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

      await waitFor(() => {
        expect(screen.getByText('podinfo-7d8b9c4f5d-abc12')).toBeInTheDocument()
      })
      expect(screen.queryByTestId('delete-pod-button')).not.toBeInTheDocument()
    })
  })
})
