// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/preact'
import { WorkloadPage } from './WorkloadPage'
import { fetchWithMock } from '../../../utils/fetch'

// Polling constants used by WorkloadPage
const POLL_INTERVAL_MS = 10000
const FAST_POLL_INTERVAL_MS = 5000
const FAST_POLL_TIMEOUT_MS = 60000

// Mock fetchWithMock
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Store ActionBar callbacks for testing dynamic polling
let capturedOnActionStart = null
let capturedActionBarProps = null

vi.mock('../resource/ActionBar', () => ({
  ActionBar: (props) => {
    capturedOnActionStart = props.onActionStart
    capturedActionBarProps = props
    return <div data-testid="action-bar">ActionBar: {props.kind}/{props.namespace}/{props.name}</div>
  }
}))

vi.mock('../resource/WorkloadActionBar', () => ({
  WorkloadActionBar: (props) => (
    <div data-testid="workload-action-bar">WorkloadActionBar: {props.kind}/{props.name}</div>
  )
}))

vi.mock('../resource/WorkloadDeleteAction', () => ({
  WorkloadDeleteAction: (props) => (
    <div data-testid="workload-delete-action">Delete: {props.name}</div>
  )
}))

describe('WorkloadPage component', () => {
  const mockWorkloadData = {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {
      name: 'nginx',
      namespace: 'default',
      creationTimestamp: '2023-01-01T00:00:00Z'
    },
    spec: {
      replicas: 3,
      strategy: {
        type: 'RollingUpdate',
        rollingUpdate: { maxUnavailable: 1, maxSurge: 1 }
      },
      selector: { matchLabels: { app: 'nginx' } },
      template: { spec: { containers: [{ name: 'nginx', image: 'nginx:1.25.0' }] } }
    },
    status: {
      readyReplicas: 3,
      conditions: [
        { type: 'Available', status: 'True', message: 'Deployment has minimum availability' }
      ]
    },
    workloadInfo: {
      status: 'Current',
      statusMessage: 'Replicas: 3',
      createdAt: '2023-01-01T00:00:00Z',
      containerImages: ['nginx:1.25.0'],
      userActions: ['restart', 'deletePods'],
      pods: [
        { name: 'nginx-abc-123', status: 'Running', statusMessage: 'Started at 2023-01-01 12:00:00 UTC', createdAt: '2023-01-01T12:00:00Z' },
        { name: 'nginx-abc-456', status: 'Running', statusMessage: 'Started at 2023-01-01 12:00:00 UTC', createdAt: '2023-01-01T12:00:00Z' }
      ],
      reconciler: {
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        kind: 'Kustomization',
        metadata: {
          name: 'apps',
          namespace: 'flux-system'
        },
        spec: {
          interval: '10m',
          prune: true,
          wait: true
        },
        status: {
          reconcilerRef: {
            status: 'Ready',
            message: 'Applied revision: main@sha1:abc123',
            lastReconciled: '2023-01-01T12:00:00Z'
          },
          sourceRef: {
            kind: 'GitRepository',
            name: 'flux-system',
            namespace: 'flux-system',
            status: 'Ready',
            url: 'https://github.com/example/repo',
            message: 'stored artifact for revision main@sha1:abc123'
          },
          userActions: ['reconcile', 'suspend', 'resume']
        }
      }
    }
  }

  beforeEach(() => {
    vi.clearAllMocks()
    capturedOnActionStart = null
    capturedActionBarProps = null
  })

  it('should render loading state with header card', () => {
    fetchWithMock.mockReturnValue(new Promise(() => {}))

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    expect(screen.getByText('Deployment')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'nginx' })).toBeInTheDocument()
    expect(screen.getByText('Namespace: default')).toBeInTheDocument()

    expect(screen.getByTestId('loading-message')).toBeInTheDocument()
    expect(screen.getByText('Loading workload data...')).toBeInTheDocument()

    const headerCard = screen.getByRole('heading', { name: 'nginx' }).closest('.card')
    expect(headerCard).toHaveClass('border-blue-500')
  })

  it('should render error state when fetch fails', async () => {
    fetchWithMock.mockRejectedValue(new Error('API Error'))

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument()
      expect(screen.getByText('Failed to load workload: API Error')).toBeInTheDocument()

      const headerCard = screen.getByRole('heading', { name: 'nginx' }).closest('.card')
      expect(headerCard).toHaveClass('border-danger')
    })
  })

  it('should render not found state when empty object returned', async () => {
    fetchWithMock.mockResolvedValueOnce({})

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      expect(screen.getByTestId('not-found-message')).toBeInTheDocument()
      expect(screen.getByText('Workload not found in the cluster.')).toBeInTheDocument()

      const headerCard = screen.getByRole('heading', { name: 'nginx' }).closest('.card')
      expect(headerCard).toHaveClass('border-gray-400')
    })
  })

  it('should render not found state when null returned', async () => {
    fetchWithMock.mockResolvedValueOnce(null)

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      expect(screen.getByTestId('not-found-message')).toBeInTheDocument()
    })
  })

  it('should render header and panels on success', async () => {
    fetchWithMock.mockResolvedValueOnce(mockWorkloadData)

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'nginx' })).toBeInTheDocument()
    })

    // 'Deployment' appears in both the header kind label and the WorkloadPanel title
    const deploymentTexts = screen.getAllByText('Deployment')
    expect(deploymentTexts.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Namespace: default')).toBeInTheDocument()

    // Current status → green
    const iconContainer = screen.getByRole('heading', { name: 'nginx' }).closest('.card').querySelector('.bg-green-50')
    expect(iconContainer).toBeInTheDocument()

    // ActionBar should receive reconciler props (not workload props)
    expect(screen.getByTestId('action-bar')).toHaveTextContent('ActionBar: Kustomization/flux-system/apps')
  })

  it('should pass reconciler data to ActionBar', async () => {
    fetchWithMock.mockResolvedValueOnce(mockWorkloadData)

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      expect(capturedActionBarProps).toBeTruthy()
    })

    expect(capturedActionBarProps.kind).toBe('Kustomization')
    expect(capturedActionBarProps.namespace).toBe('flux-system')
    expect(capturedActionBarProps.name).toBe('apps')
    expect(capturedActionBarProps.resourceData).toBe(mockWorkloadData.workloadInfo.reconciler)
  })

  it('should render WorkloadActionBar alongside ActionBar with separator', async () => {
    fetchWithMock.mockResolvedValueOnce(mockWorkloadData)

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      expect(screen.getByTestId('combined-action-bar')).toBeInTheDocument()
    })

    expect(screen.getByTestId('action-bar')).toBeInTheDocument()
    expect(screen.getByTestId('workload-action-bar')).toBeInTheDocument()
    expect(screen.getByTestId('action-bar-separator')).toBeInTheDocument()
  })

  it('should render correct status colors for Failed workload', async () => {
    const failedData = {
      ...mockWorkloadData,
      workloadInfo: { ...mockWorkloadData.workloadInfo, status: 'Failed' }
    }
    fetchWithMock.mockResolvedValueOnce(failedData)

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      const headerCard = screen.getByRole('heading', { name: 'nginx' }).closest('.card')
      expect(headerCard).toHaveClass('bg-red-50')
      expect(headerCard).toHaveClass('border-danger')
    })
  })

  it('should render correct status colors for InProgress workload', async () => {
    const progressingData = {
      ...mockWorkloadData,
      workloadInfo: { ...mockWorkloadData.workloadInfo, status: 'InProgress' }
    }
    fetchWithMock.mockResolvedValueOnce(progressingData)

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      const headerCard = screen.getByRole('heading', { name: 'nginx' }).closest('.card')
      expect(headerCard).toHaveClass('bg-blue-50')
      expect(headerCard).toHaveClass('border-blue-500')
    })
  })

  it('should render correct status colors for Terminating workload', async () => {
    const terminatingData = {
      ...mockWorkloadData,
      workloadInfo: { ...mockWorkloadData.workloadInfo, status: 'Terminating' }
    }
    fetchWithMock.mockResolvedValueOnce(terminatingData)

    render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

    await waitFor(() => {
      const headerCard = screen.getByRole('heading', { name: 'nginx' }).closest('.card')
      expect(headerCard).toHaveClass('bg-yellow-50')
      expect(headerCard).toHaveClass('border-yellow-500')
    })
  })

  describe('Auto-refresh functionality', () => {
    beforeEach(() => {
      vi.useFakeTimers()
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    it('should fetch data on mount and setup auto-refresh interval', async () => {
      fetchWithMock.mockResolvedValue(mockWorkloadData)

      render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      fetchWithMock.mockClear()
      fetchWithMock.mockResolvedValue(mockWorkloadData)

      vi.advanceTimersByTime(POLL_INTERVAL_MS)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })
    })

    it('should set lastUpdatedAt timestamp on successful fetch', async () => {
      const now = new Date('2023-01-01T12:30:00Z')
      vi.setSystemTime(now)

      fetchWithMock.mockResolvedValueOnce(mockWorkloadData)

      render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

      await waitFor(() => {
        expect(screen.getByText('Last Updated')).toBeInTheDocument()
      })
    })

    it('should preserve existing data when auto-refresh fails', async () => {
      fetchWithMock.mockResolvedValue(mockWorkloadData)

      render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: 'nginx' })).toBeInTheDocument()
      })

      fetchWithMock.mockClear()
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      vi.advanceTimersByTime(POLL_INTERVAL_MS)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalled()
      })

      expect(screen.getByRole('heading', { name: 'nginx' })).toBeInTheDocument()
      expect(screen.queryByText('Failed to load workload: Network error')).not.toBeInTheDocument()
    })

    it('should clear interval on unmount', async () => {
      fetchWithMock.mockResolvedValue(mockWorkloadData)

      const { unmount } = render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: 'nginx' })).toBeInTheDocument()
      })

      fetchWithMock.mockClear()
      unmount()

      vi.advanceTimersByTime(POLL_INTERVAL_MS)

      expect(fetchWithMock).not.toHaveBeenCalled()
    })

    it('should switch to fast polling when action is triggered', async () => {
      fetchWithMock.mockResolvedValue(mockWorkloadData)

      render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

      await waitFor(() => {
        expect(capturedOnActionStart).toBeTruthy()
      })

      fetchWithMock.mockClear()

      await act(async () => {
        capturedOnActionStart()
      })

      expect(fetchWithMock).not.toHaveBeenCalled()

      await act(async () => {
        vi.advanceTimersByTime(FAST_POLL_INTERVAL_MS)
      })

      expect(fetchWithMock).toHaveBeenCalledTimes(1)
    })

    it('should revert to normal polling after timeout', async () => {
      fetchWithMock.mockResolvedValue(mockWorkloadData)

      render(<WorkloadPage kind="Deployment" namespace="default" name="nginx" />)

      await waitFor(() => {
        expect(capturedOnActionStart).toBeTruthy()
      })

      fetchWithMock.mockClear()

      await act(async () => {
        capturedOnActionStart()
      })

      await act(async () => {
        vi.advanceTimersByTime(FAST_POLL_TIMEOUT_MS)
      })

      fetchWithMock.mockClear()

      await act(async () => {
        vi.advanceTimersByTime(FAST_POLL_INTERVAL_MS)
      })

      // Only 5s into a 10s interval, so no fetch
      expect(fetchWithMock).not.toHaveBeenCalled()

      await act(async () => {
        vi.advanceTimersByTime(POLL_INTERVAL_MS - FAST_POLL_INTERVAL_MS)
      })

      expect(fetchWithMock).toHaveBeenCalledTimes(1)
    })
  })
})
