// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadDetailsView } from './WorkloadDetailsView'
import { fetchWithMock } from '../../utils/fetch'

// Mock the fetch utility
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

describe('WorkloadDetailsView component', () => {
  const mockWorkloadData = {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {
      name: 'podinfo',
      namespace: 'apps'
    },
    spec: {
      strategy: {
        type: 'RollingUpdate',
        rollingUpdate: { maxUnavailable: 1, maxSurge: 1 }
      },
      template: {
        spec: {
          serviceAccountName: 'podinfo',
          containers: [
            { name: 'podinfod', ports: [{ containerPort: 9898 }, { containerPort: 9797 }] }
          ]
        }
      }
    },
    status: {
      replicas: 2,
      conditions: [
        { type: 'Available', status: 'True', lastUpdateTime: '2025-01-15T10:00:00Z' }
      ]
    },
    workloadInfo: {
      status: 'Current',
      statusMessage: 'Deployment is available. Replicas: 2',
      createdAt: '2025-01-10T08:00:00Z',
      pods: [
        { name: 'podinfo-1', status: 'Running', podStatus: { conditions: [{ type: 'Ready', status: 'True' }] } },
        { name: 'podinfo-2', status: 'Running', podStatus: { conditions: [{ type: 'Ready', status: 'True' }] } }
      ]
    }
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render nothing when isExpanded is false', () => {
    const { container } = render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={false} />
    )

    expect(container.firstChild).toBeNull()
  })

  it('should not fetch until expanded', () => {
    render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={false} />
    )

    expect(fetchWithMock).not.toHaveBeenCalled()
  })

  it('should fetch workload data when expanded for the first time', async () => {
    fetchWithMock.mockResolvedValue(mockWorkloadData)

    render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/workload?kind=Deployment&name=podinfo&namespace=apps',
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkload'
      })
    })
  })

  it('should show loading state while fetching', async () => {
    let resolvePromise
    const promise = new Promise((resolve) => { resolvePromise = resolve })
    fetchWithMock.mockReturnValue(promise)

    render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />
    )

    expect(screen.getByText('Loading details...')).toBeInTheDocument()
    expect(document.querySelector('.animate-spin')).toBeInTheDocument()

    resolvePromise(mockWorkloadData)

    await waitFor(() => {
      expect(screen.queryByText('Loading details...')).not.toBeInTheDocument()
    })
  })

  it('should render the overview tab with status and metadata by default', async () => {
    fetchWithMock.mockResolvedValue(mockWorkloadData)

    render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />
    )

    await waitFor(() => {
      expect(screen.getByText('Service account')).toBeInTheDocument()
    })

    expect(screen.getByText('Service account')).toBeInTheDocument()
    expect(screen.getByText('podinfo')).toBeInTheDocument()
    expect(screen.getByText('Ports')).toBeInTheDocument()
    expect(screen.getByText('9797, 9898')).toBeInTheDocument()
    expect(screen.getByText('Pods')).toBeInTheDocument()
    expect(screen.getByText('2/2 ready')).toBeInTheDocument()
    expect(screen.queryByText('Strategy')).not.toBeInTheDocument()
  })

  it('should display the specification tab as highlighted YAML', async () => {
    fetchWithMock.mockResolvedValue(mockWorkloadData)
    const user = userEvent.setup()

    render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />
    )

    await waitFor(() => {
      expect(screen.getByText('Specification')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Specification'))

    await waitFor(() => {
      const codeElement = document.querySelector('.language-yaml')
      expect(codeElement).not.toBeNull()
      expect(codeElement.innerHTML).toContain('apiVersion')
      expect(codeElement.innerHTML).toContain('apps/v1')
      expect(codeElement.innerHTML).toContain('Deployment')
      expect(codeElement.innerHTML).toContain('serviceAccountName')
    })
  })

  it('should display the status tab as highlighted YAML', async () => {
    fetchWithMock.mockResolvedValue(mockWorkloadData)
    const user = userEvent.setup()

    render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />
    )

    await waitFor(() => {
      expect(screen.getByText('Specification')).toBeInTheDocument()
    })

    // The Status tab button is disambiguated from the Overview "Status" label by role.
    await user.click(screen.getByRole('button', { name: 'Status' }))

    await waitFor(() => {
      const codeElement = document.querySelector('.language-yaml')
      expect(codeElement).not.toBeNull()
      expect(codeElement.innerHTML).toContain('status')
      expect(codeElement.innerHTML).toContain('replicas')
    })
  })

  it('should show a not-found state when the workload has no metadata', async () => {
    fetchWithMock.mockResolvedValue({ kind: 'Deployment' })

    render(
      <WorkloadDetailsView kind="Deployment" name="missing" namespace="apps" isExpanded={true} />
    )

    await waitFor(() => {
      expect(screen.getByText('Workload not found in the cluster.')).toBeInTheDocument()
    })
  })

  it('should show an error state when the fetch fails', async () => {
    fetchWithMock.mockRejectedValue(new Error('User does not have access to the workload'))

    render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />
    )

    await waitFor(() => {
      expect(screen.getByText(/Failed to load details: User does not have access to the workload/)).toBeInTheDocument()
    })
  })

  it('should not render a pods tab, events tab, or action controls', async () => {
    fetchWithMock.mockResolvedValue(mockWorkloadData)

    render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />
    )

    await waitFor(() => {
      expect(screen.getByText('Service account')).toBeInTheDocument()
    })

    expect(screen.queryByRole('button', { name: 'Pods' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Events' })).not.toBeInTheDocument()
    expect(screen.queryByTestId('logs-pod-button')).not.toBeInTheDocument()
    expect(screen.queryByTestId('workload-delete-action')).not.toBeInTheDocument()
  })

  it('should fetch only once across collapse and re-expand', async () => {
    fetchWithMock.mockResolvedValue(mockWorkloadData)

    const { rerender } = render(
      <WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />
    )

    await waitFor(() => {
      expect(fetchWithMock).toHaveBeenCalledTimes(1)
    })

    // Collapse then re-expand: cached data must not trigger a second fetch
    rerender(<WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={false} />)
    rerender(<WorkloadDetailsView kind="Deployment" name="podinfo" namespace="apps" isExpanded={true} />)

    await waitFor(() => {
      expect(screen.getByText('Service account')).toBeInTheDocument()
    })
    expect(fetchWithMock).toHaveBeenCalledTimes(1)
  })
})
