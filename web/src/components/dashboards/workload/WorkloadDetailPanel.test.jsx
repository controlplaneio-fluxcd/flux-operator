// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadDetailPanel } from './WorkloadDetailPanel'
import { fetchWithMock } from '../../../utils/fetch'

// Mock fetchWithMock
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock useHashTab to use simple useState instead
vi.mock('../../../utils/hash', async () => {
  const { useState } = await import('preact/hooks')
  return {
    useHashTab: (panel, defaultTab) => useState(defaultTab)
  }
})

vi.mock('../resource/WorkloadDeleteAction', () => ({
  WorkloadDeleteAction: (props) => (
    <div data-testid="workload-delete-action">Delete: {props.name}</div>
  )
}))

describe('WorkloadDetailPanel component', () => {
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
    }
  }

  const mockWorkloadInfo = {
    status: 'Current',
    statusMessage: 'Replicas: 3',
    createdAt: '2023-01-01T00:00:00Z',
    containerImages: ['nginx:1.25.0'],
    userActions: ['restart', 'deletePods'],
    pods: [
      { name: 'nginx-abc-123', status: 'Running', statusMessage: 'Started at 2023-01-01 12:00:00 UTC', createdAt: '2023-01-01T12:00:00Z' },
      { name: 'nginx-abc-456', status: 'Running', statusMessage: 'Started at 2023-01-01 12:00:00 UTC', createdAt: '2023-01-01T11:00:00Z' }
    ]
  }

  const defaultProps = {
    kind: 'Deployment',
    namespace: 'default',
    name: 'nginx',
    workloadData: mockWorkloadData,
    workloadInfo: mockWorkloadInfo,
    workloadStatus: 'Current',
    pendingDeletions: new Set(),
    onPodDeleteStart: vi.fn(),
    onPodDeleteFailed: vi.fn(),
    onActionStart: vi.fn(),
    onActionComplete: vi.fn()
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the panel title from kind', () => {
    render(<WorkloadDetailPanel {...defaultProps} />)

    expect(screen.getByText('Deployment')).toBeInTheDocument()
  })

  it('should render all tab buttons', () => {
    render(<WorkloadDetailPanel {...defaultProps} />)

    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByText('Pods')).toBeInTheDocument()
    expect(screen.getByText('Events')).toBeInTheDocument()
    expect(screen.getByText('Specification')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Status' })).toBeInTheDocument()
  })

  it('should toggle collapse/expand state', async () => {
    const user = userEvent.setup()

    render(<WorkloadDetailPanel {...defaultProps} />)

    expect(screen.getByText('Overview')).toBeInTheDocument()

    const headerButton = screen.getByText('Deployment').closest('button')
    await user.click(headerButton)

    expect(screen.queryByText('Overview')).not.toBeInTheDocument()

    await user.click(headerButton)

    expect(screen.getByText('Overview')).toBeInTheDocument()
  })

  describe('Overview tab', () => {
    it('should display status badge', () => {
      render(<WorkloadDetailPanel {...defaultProps} />)

      // formatWorkloadStatus('Current') returns 'Ready'
      expect(screen.getByText('Ready')).toBeInTheDocument()
    })

    it('should display status message in right column', () => {
      render(<WorkloadDetailPanel {...defaultProps} />)

      expect(screen.getByText('Replicas: 3')).toBeInTheDocument()
    })

    it('should display last action timestamp from conditions', () => {
      const dataWithTransitionTime = {
        ...mockWorkloadData,
        status: {
          ...mockWorkloadData.status,
          conditions: [
            { type: 'Available', status: 'True', message: 'Deployment has minimum availability', lastUpdateTime: '2023-06-15T10:30:00Z' }
          ]
        }
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadData={dataWithTransitionTime}
        />
      )

      expect(screen.getByText('Last action')).toBeInTheDocument()
    })

    it('should fall back to creation time for last action when conditions lack lastTransitionTime', () => {
      render(<WorkloadDetailPanel {...defaultProps} />)

      // No lastTransitionTime in conditions, falls back to workloadInfo.createdAt
      expect(screen.getByText('Last action')).toBeInTheDocument()
    })

    it('should display last action from lastScheduleTime for CronJob', () => {
      const cronData = {
        ...mockWorkloadData,
        kind: 'CronJob',
        spec: { ...mockWorkloadData.spec, schedule: '*/5 * * * *' },
        status: {
          lastScheduleTime: '2023-06-15T11:00:00Z'
        }
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          kind="CronJob"
          workloadData={cronData}
        />
      )

      expect(screen.getByText('Last action')).toBeInTheDocument()
    })

    it('should display created timestamp', () => {
      render(<WorkloadDetailPanel {...defaultProps} />)

      expect(screen.getByText('Created')).toBeInTheDocument()
    })

    it('should display rollout strategy', () => {
      render(<WorkloadDetailPanel {...defaultProps} />)

      expect(screen.getByText('Strategy')).toBeInTheDocument()
      expect(screen.getByText(/RollingUpdate/)).toBeInTheDocument()
      expect(screen.getByText(/maxUnavailable: 1/)).toBeInTheDocument()
    })

    it('should display service account', () => {
      const dataWithSA = {
        ...mockWorkloadData,
        spec: {
          ...mockWorkloadData.spec,
          template: {
            spec: {
              serviceAccountName: 'nginx-sa',
              containers: [{ name: 'nginx', image: 'nginx:1.25.0' }]
            }
          }
        }
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadData={dataWithSA}
        />
      )

      expect(screen.getByText('Service account')).toBeInTheDocument()
      expect(screen.getByText('nginx-sa')).toBeInTheDocument()
    })

    it('should display sorted container ports', () => {
      const dataWithPorts = {
        ...mockWorkloadData,
        spec: {
          ...mockWorkloadData.spec,
          template: {
            spec: {
              containers: [
                { name: 'nginx', image: 'nginx:1.25.0', ports: [{ containerPort: 8080 }, { containerPort: 80 }] },
                { name: 'sidecar', image: 'sidecar:1.0', ports: [{ containerPort: 9090 }] }
              ]
            }
          }
        }
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadData={dataWithPorts}
        />
      )

      expect(screen.getByText('Ports')).toBeInTheDocument()
      expect(screen.getByText('80, 8080, 9090')).toBeInTheDocument()
    })

    it('should not display ports when containers have no ports', () => {
      render(<WorkloadDetailPanel {...defaultProps} />)

      expect(screen.queryByText('Ports')).not.toBeInTheDocument()
    })

    it('should display concurrency policy for CronJob', () => {
      const cronData = {
        ...mockWorkloadData,
        kind: 'CronJob',
        spec: { ...mockWorkloadData.spec, schedule: '*/5 * * * *', concurrencyPolicy: 'Forbid' }
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          kind="CronJob"
          workloadData={cronData}
        />
      )

      expect(screen.getByText('Concurrency policy')).toBeInTheDocument()
      expect(screen.getByText('Forbid')).toBeInTheDocument()
    })

    it('should display DaemonSet update strategy with rollingUpdate details', () => {
      const daemonData = {
        ...mockWorkloadData,
        kind: 'DaemonSet',
        spec: {
          ...mockWorkloadData.spec,
          updateStrategy: {
            type: 'RollingUpdate',
            rollingUpdate: { maxUnavailable: 1, maxSurge: 2 }
          }
        }
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          kind="DaemonSet"
          workloadData={daemonData}
        />
      )

      expect(screen.getByText('Update strategy')).toBeInTheDocument()
      expect(screen.getByText(/maxUnavailable: 1/)).toBeInTheDocument()
      expect(screen.getByText(/maxSurge: 2/)).toBeInTheDocument()
    })

    it('should display StatefulSet update strategy with rollingUpdate details', () => {
      const stsData = {
        ...mockWorkloadData,
        kind: 'StatefulSet',
        spec: {
          ...mockWorkloadData.spec,
          strategy: undefined,
          updateStrategy: {
            type: 'RollingUpdate',
            rollingUpdate: { maxUnavailable: 1, partition: 0 }
          }
        }
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          kind="StatefulSet"
          workloadData={stsData}
        />
      )

      expect(screen.getByText('Update strategy')).toBeInTheDocument()
      expect(screen.getByText(/maxUnavailable: 1/)).toBeInTheDocument()
      expect(screen.getByText(/partition: 0/)).toBeInTheDocument()
    })

    it('should not display spec.strategy for non-Deployment kinds', () => {
      const stsData = {
        ...mockWorkloadData,
        kind: 'StatefulSet',
        spec: { ...mockWorkloadData.spec }
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          kind="StatefulSet"
          workloadData={stsData}
        />
      )

      // spec.strategy.type exists on mockWorkloadData but should not render for StatefulSet
      expect(screen.queryByText('Strategy')).not.toBeInTheDocument()
    })
  })

  describe('Pods tab', () => {
    it('should display pods list', async () => {
      const user = userEvent.setup()

      render(<WorkloadDetailPanel {...defaultProps} />)

      await user.click(screen.getByText('Pods'))

      expect(screen.getByText('nginx-abc-123')).toBeInTheDocument()
      expect(screen.getByText('nginx-abc-456')).toBeInTheDocument()
    })

    it('should sort pods by creation time (most recent first)', async () => {
      const user = userEvent.setup()

      render(<WorkloadDetailPanel {...defaultProps} />)

      await user.click(screen.getByText('Pods'))

      // Query pod name spans directly (exclude delete action text)
      const podNameSpans = screen.getAllByText(/^nginx-abc-\d+$/)
      // nginx-abc-123 has createdAt 12:00, nginx-abc-456 has 11:00
      // Most recent first
      expect(podNameSpans[0]).toHaveTextContent('nginx-abc-123')
      expect(podNameSpans[1]).toHaveTextContent('nginx-abc-456')
    })

    it('should show "No pods found" when pods list is empty', async () => {
      const user = userEvent.setup()

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadInfo={{ ...mockWorkloadInfo, pods: [] }}
        />
      )

      await user.click(screen.getByText('Pods'))

      expect(screen.getByText('No pods found')).toBeInTheDocument()
    })

    it('should show "No recent jobs" for CronJob with no pods', async () => {
      const user = userEvent.setup()

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          kind="CronJob"
          workloadInfo={{ ...mockWorkloadInfo, pods: [] }}
        />
      )

      await user.click(screen.getByText('Pods'))

      expect(screen.getByText('No recent jobs')).toBeInTheDocument()
    })

    it('should render delete buttons when deletePods action is available', async () => {
      const user = userEvent.setup()

      render(<WorkloadDetailPanel {...defaultProps} />)

      await user.click(screen.getByText('Pods'))

      const deleteActions = screen.getAllByTestId('workload-delete-action')
      expect(deleteActions).toHaveLength(2)
    })

    it('should show Terminating status for pending deletions', async () => {
      const user = userEvent.setup()

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          pendingDeletions={new Set(['nginx-abc-123'])}
        />
      )

      await user.click(screen.getByText('Pods'))

      expect(screen.getByText('Terminating')).toBeInTheDocument()
    })

    it('should display triggered by info for pods with createdBy', async () => {
      const user = userEvent.setup()
      const infoWithTriggeredPod = {
        ...mockWorkloadInfo,
        pods: [
          { name: 'pod-1', status: 'Running', createdAt: '2023-01-01T12:00:00Z', createdBy: 'admin@example.com' }
        ]
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadInfo={infoWithTriggeredPod}
        />
      )

      await user.click(screen.getByText('Pods'))

      expect(screen.getByTestId('pod-created-by')).toHaveTextContent('Triggered by admin@example.com')
    })

    it('should show container summary when podStatus has non-ready containers', async () => {
      const user = userEvent.setup()
      const infoWithPodStatus = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'pod-1',
          status: 'Pending',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Pending',
            containerStatuses: [
              { name: 'app', ready: true, restartCount: 0, state: { running: {} } },
              { name: 'sidecar', ready: false, restartCount: 0, state: { waiting: { reason: 'CrashLoopBackOff' } } }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoWithPodStatus} />)
      await user.click(screen.getByText('Pods'))

      const summary = screen.getByTestId('pod-container-summary')
      expect(summary).toHaveTextContent('1/2 ready')
    })

    it('should show summary line for healthy pods with all containers ready', async () => {
      const user = userEvent.setup()
      const infoHealthy = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'pod-1',
          status: 'Running',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Running',
            containerStatuses: [
              { name: 'app', ready: true, restartCount: 0, state: { running: {} } }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoHealthy} />)
      await user.click(screen.getByText('Pods'))

      const summary = screen.getByTestId('pod-container-summary')
      expect(summary).toHaveTextContent('Containers: 1/1 ready')
    })

    it('should show completed instead of ready for succeeded pods', async () => {
      const user = userEvent.setup()
      const infoSucceeded = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'job-pod-1',
          status: 'Succeeded',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Succeeded',
            containerStatuses: [
              { name: 'worker', ready: false, restartCount: 0, state: { terminated: { exitCode: 0, reason: 'Completed' } } }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoSucceeded} />)
      await user.click(screen.getByText('Pods'))

      const summary = screen.getByTestId('pod-container-summary')
      expect(summary).toHaveTextContent('Containers: 1/1 completed')
    })

    it('should expand pod to show container statuses on click', async () => {
      const user = userEvent.setup()
      const infoWithPodStatus = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'pod-1',
          status: 'Pending',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Pending',
            containerStatuses: [
              { name: 'manager', ready: false, restartCount: 3, state: { waiting: { reason: 'ImagePullBackOff' } } }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoWithPodStatus} />)
      await user.click(screen.getByText('Pods'))

      // Should not show expanded details initially
      expect(screen.queryByTestId('pod-expanded-details')).not.toBeInTheDocument()

      // Click on the pod row to expand
      await user.click(screen.getByText('pod-1'))

      // Should show expanded container details
      expect(screen.getByTestId('pod-expanded-details')).toBeInTheDocument()
      expect(screen.getByText('manager')).toBeInTheDocument()
      expect(screen.getByText('Waiting')).toBeInTheDocument()
      expect(screen.getByText('ImagePullBackOff')).toBeInTheDocument()
    })

    it('should collapse pod on second click', async () => {
      const user = userEvent.setup()
      const infoWithPodStatus = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'pod-1',
          status: 'Running',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Running',
            containerStatuses: [
              { name: 'app', ready: true, restartCount: 0, state: { running: {} } }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoWithPodStatus} />)
      await user.click(screen.getByText('Pods'))

      // Click to expand
      await user.click(screen.getByText('pod-1'))
      expect(screen.getByTestId('pod-expanded-details')).toBeInTheDocument()

      // Click again to collapse
      await user.click(screen.getByText('pod-1'))
      expect(screen.queryByTestId('pod-expanded-details')).not.toBeInTheDocument()
    })

    it('should show waiting reason and message for waiting containers', async () => {
      const user = userEvent.setup()
      const infoWithWaiting = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'pod-1',
          status: 'Pending',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Pending',
            containerStatuses: [
              {
                name: 'app',
                ready: false,
                restartCount: 0,
                state: {
                  waiting: {
                    reason: 'ImagePullBackOff',
                    message: 'Back-off pulling image "nginx:invalid"'
                  }
                }
              }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoWithWaiting} />)
      await user.click(screen.getByText('Pods'))
      await user.click(screen.getByText('pod-1'))

      expect(screen.getByText('ImagePullBackOff')).toBeInTheDocument()
      expect(screen.getByText('Back-off pulling image "nginx:invalid"')).toBeInTheDocument()
    })

    it('should show terminated reason and exit code', async () => {
      const user = userEvent.setup()
      const infoWithTerminated = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'pod-1',
          status: 'Failed',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Failed',
            containerStatuses: [
              {
                name: 'app',
                ready: false,
                restartCount: 0,
                state: {
                  terminated: {
                    reason: 'OOMKilled',
                    exitCode: 137
                  }
                }
              }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoWithTerminated} />)
      await user.click(screen.getByText('Pods'))
      await user.click(screen.getByText('pod-1'))

      expect(screen.getByText('Terminated')).toBeInTheDocument()
      expect(screen.getByText('OOMKilled (exit 137)')).toBeInTheDocument()
    })

    it('should display container image from image field when available', async () => {
      const user = userEvent.setup()
      const infoWithImage = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'pod-1',
          status: 'Running',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Running',
            containerStatuses: [
              { name: 'app', ready: true, restartCount: 0, image: 'nginx:1.25.0', imageID: 'docker-pullable://nginx:1.25.0@sha256:abc123', state: { running: {} } }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoWithImage} />)
      await user.click(screen.getByText('Pods'))
      await user.click(screen.getByText('pod-1'))

      expect(screen.getByText('nginx:1.25.0')).toBeInTheDocument()
    })

    it('should fall back to imageID when image is a bare digest', async () => {
      const user = userEvent.setup()
      const infoWithDigest = {
        ...mockWorkloadInfo,
        pods: [{
          name: 'pod-1',
          status: 'Running',
          createdAt: '2023-01-01T12:00:00Z',
          podStatus: {
            phase: 'Running',
            containerStatuses: [
              { name: 'app', ready: true, restartCount: 0, image: 'sha256:abc123def456', imageID: 'docker-pullable://nginx:1.25.0@sha256:abc123def456', state: { running: {} } }
            ]
          }
        }]
      }

      render(<WorkloadDetailPanel {...defaultProps} workloadInfo={infoWithDigest} />)
      await user.click(screen.getByText('Pods'))
      await user.click(screen.getByText('pod-1'))

      expect(screen.getByText('nginx:1.25.0@sha256:abc123def456')).toBeInTheDocument()
    })

    it('should not render chevron or expand for pods without podStatus', async () => {
      const user = userEvent.setup()
      // Default mockWorkloadInfo has pods without podStatus
      render(<WorkloadDetailPanel {...defaultProps} />)
      await user.click(screen.getByText('Pods'))

      expect(screen.queryByTestId('pod-chevron')).not.toBeInTheDocument()
    })
  })

  describe('Events tab', () => {
    it('should fetch and display events when Events tab is clicked', async () => {
      const user = userEvent.setup()
      const mockEvents = {
        events: [
          {
            type: 'Normal',
            reason: 'Scaled',
            message: 'Scaled up replica set nginx-abc to 3',
            lastTimestamp: '2023-01-01T12:05:00Z'
          },
          {
            type: 'Warning',
            reason: 'BackOff',
            message: 'Back-off restarting failed container',
            lastTimestamp: '2023-01-01T12:10:00Z'
          }
        ]
      }

      fetchWithMock.mockResolvedValueOnce(mockEvents)

      render(<WorkloadDetailPanel {...defaultProps} />)

      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledWith(expect.objectContaining({
          endpoint: expect.stringContaining('/api/v1/events')
        }))
      })

      await waitFor(() => {
        expect(screen.getByText('Scaled up replica set nginx-abc to 3')).toBeInTheDocument()
        expect(screen.getByText('Back-off restarting failed container')).toBeInTheDocument()
      })

      const infoBadges = screen.getAllByText('Info')
      expect(infoBadges.length).toBeGreaterThan(0)
      expect(screen.getByText('Warning')).toBeInTheDocument()
    })

    it('should display "No events found" when events list is empty', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockResolvedValueOnce({ events: [] })

      render(<WorkloadDetailPanel {...defaultProps} />)

      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(screen.getByText('No events found')).toBeInTheDocument()
      })
    })

    it('should handle fetch error gracefully', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockRejectedValueOnce(new Error('Network error'))
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      render(<WorkloadDetailPanel {...defaultProps} />)

      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalled()
      })

      await waitFor(() => {
        expect(screen.getByText('No events found')).toBeInTheDocument()
      })

      consoleSpy.mockRestore()
    })
  })

  describe('Events auto-refresh', () => {
    const mockEvents = {
      events: [
        {
          type: 'Normal',
          reason: 'Scaled',
          message: 'Scaled up replica set',
          lastTimestamp: '2023-01-01T12:05:00Z'
        }
      ]
    }

    it('should refetch events when workloadData changes if Events tab is open', async () => {
      const user = userEvent.setup()

      const { rerender } = render(<WorkloadDetailPanel {...defaultProps} />)

      fetchWithMock.mockResolvedValueOnce(mockEvents)
      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(screen.getByText('Scaled up replica set')).toBeInTheDocument()
      })

      fetchWithMock.mockResolvedValueOnce({
        events: [
          {
            type: 'Normal',
            reason: 'Scaled',
            message: 'New event after refresh',
            lastTimestamp: '2023-01-01T13:05:00Z'
          }
        ]
      })

      const updatedWorkloadData = {
        ...mockWorkloadData,
        status: { ...mockWorkloadData.status, readyReplicas: 4 }
      }

      rerender(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadData={updatedWorkloadData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('New event after refresh')).toBeInTheDocument()
      })
      expect(fetchWithMock.mock.calls.length).toBeGreaterThanOrEqual(2)
    })

    it('should NOT refetch events when workloadData changes if Events tab is not open', async () => {
      const { rerender } = render(<WorkloadDetailPanel {...defaultProps} />)

      expect(fetchWithMock).not.toHaveBeenCalled()

      const updatedWorkloadData = {
        ...mockWorkloadData,
        status: { ...mockWorkloadData.status, readyReplicas: 4 }
      }

      rerender(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadData={updatedWorkloadData}
        />
      )

      await waitFor(() => {
        expect(fetchWithMock).not.toHaveBeenCalled()
      })
    })

    it('should NOT double-fetch events on initial mount', async () => {
      const user = userEvent.setup()

      render(<WorkloadDetailPanel {...defaultProps} />)

      fetchWithMock.mockResolvedValueOnce(mockEvents)
      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })
    })

    it('should preserve event data when refetch fails during auto-refresh', async () => {
      const user = userEvent.setup()
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      const { rerender } = render(<WorkloadDetailPanel {...defaultProps} />)

      fetchWithMock.mockResolvedValueOnce(mockEvents)
      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(screen.getByText('Scaled up replica set')).toBeInTheDocument()
      })

      fetchWithMock.mockRejectedValueOnce(new Error('Network error'))

      const updatedWorkloadData = {
        ...mockWorkloadData,
        status: { ...mockWorkloadData.status, readyReplicas: 4 }
      }

      rerender(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadData={updatedWorkloadData}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Scaled up replica set')).toBeInTheDocument()
      })

      consoleSpy.mockRestore()
    })
  })

  describe('Specification tab', () => {
    it('should switch to Specification tab and display yaml', async () => {
      const user = userEvent.setup()

      render(<WorkloadDetailPanel {...defaultProps} />)

      await user.click(screen.getByText('Specification'))

      // YamlBlock uses Prism which splits text — check for key parts
      expect(screen.getAllByText(/replicas/).length).toBeGreaterThan(0)
    })
  })

  describe('Status tab', () => {
    it('should switch to Status tab and display yaml', async () => {
      const user = userEvent.setup()

      render(<WorkloadDetailPanel {...defaultProps} />)

      await user.click(screen.getByRole('button', { name: 'Status' }))

      expect(screen.getAllByText(/readyReplicas/).length).toBeGreaterThan(0)
    })
  })

  describe('Edge cases', () => {
    it('should handle null workloadInfo gracefully', () => {
      render(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadInfo={null}
          workloadStatus="Unknown"
        />
      )

      expect(screen.getByText('Unknown')).toBeInTheDocument()
    })

    it('should handle pods without createdAt in sorting', async () => {
      const user = userEvent.setup()
      const infoWithMissingDates = {
        ...mockWorkloadInfo,
        pods: [
          { name: 'pod-no-date', status: 'Running' },
          { name: 'pod-with-date', status: 'Running', createdAt: '2023-01-01T12:00:00Z' }
        ]
      }

      render(
        <WorkloadDetailPanel
          {...defaultProps}
          workloadInfo={infoWithMissingDates}
        />
      )

      await user.click(screen.getByText('Pods'))

      // Pod with date should come first (query exact pod name spans)
      const pods = screen.getAllByText(/^pod-/)
      expect(pods[0]).toHaveTextContent('pod-with-date')
      expect(pods[1]).toHaveTextContent('pod-no-date')
    })
  })
})
