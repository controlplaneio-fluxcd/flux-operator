// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { WorkloadReconcilerPanel } from './WorkloadReconcilerPanel'
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

describe('WorkloadReconcilerPanel component', () => {
  const mockReconciler = {
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
      }
    }
  }

  const mockWorkloadData = { metadata: { name: 'nginx' } }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the Reconciler title', () => {
    render(
      <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
    )

    expect(screen.getByText('Reconciler')).toBeInTheDocument()
  })

  it('should be expanded by default with tabs visible', () => {
    render(
      <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
    )

    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByText('Source')).toBeInTheDocument()
    expect(screen.getByText('Events')).toBeInTheDocument()
  })

  it('should toggle collapse/expand state', async () => {
    const user = userEvent.setup()

    render(
      <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
    )

    expect(screen.getByText('Overview')).toBeInTheDocument()

    const headerButton = screen.getByText('Reconciler').closest('button')
    await user.click(headerButton)

    expect(screen.queryByText('Overview')).not.toBeInTheDocument()

    await user.click(headerButton)

    expect(screen.getByText('Overview')).toBeInTheDocument()
  })

  it('should display overview tab content', () => {
    render(
      <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
    )

    // Status
    expect(screen.getAllByText('Status').length).toBeGreaterThan(0)
    expect(screen.getByText('Ready')).toBeInTheDocument()

    // Reconciled by
    expect(screen.getByText('Reconciled by')).toBeInTheDocument()

    // Reconcile every
    expect(screen.getByText('Reconcile every')).toBeInTheDocument()
    expect(screen.getByText('10m')).toBeInTheDocument()

    // Message
    expect(screen.getByText(/Applied revision: main@sha1:abc123/)).toBeInTheDocument()
  })

  it('should render reconciler link with Name label and correct href', () => {
    render(
      <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
    )

    const nameLabels = screen.getAllByText('Name')
    expect(nameLabels.length).toBeGreaterThan(0)

    const link = screen.getByTestId('reconciler-link')
    expect(link).toHaveAttribute('href', '/resource/Kustomization/flux-system/apps')
  })

  describe('Reconcile interval', () => {
    it('should calculate interval from spec.interval', () => {
      render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('10m')).toBeInTheDocument()
    })

    it('should calculate interval from annotation', () => {
      const reconciler = {
        ...mockReconciler,
        spec: {},
        metadata: {
          ...mockReconciler.metadata,
          annotations: { 'fluxcd.controlplane.io/reconcileEvery': '3m' }
        }
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('3m')).toBeInTheDocument()
    })

    it('should use default 60m for FluxInstance', () => {
      const reconciler = {
        ...mockReconciler,
        kind: 'FluxInstance',
        spec: {},
        metadata: { name: 'flux', namespace: 'flux-system' }
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('60m')).toBeInTheDocument()
    })

    it('should use default 60m for ResourceSet', () => {
      const reconciler = {
        ...mockReconciler,
        kind: 'ResourceSet',
        spec: {},
        metadata: { name: 'rs', namespace: 'flux-system' }
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('60m')).toBeInTheDocument()
    })

    it('should use default 10m for ResourceSetInputProvider', () => {
      const reconciler = {
        ...mockReconciler,
        kind: 'ResourceSetInputProvider',
        spec: {},
        metadata: { name: 'ip', namespace: 'flux-system' }
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('10m')).toBeInTheDocument()
    })

    it('should not show interval for unknown kind without spec.interval', () => {
      const reconciler = {
        ...mockReconciler,
        kind: 'UnknownKind',
        spec: {},
        metadata: { name: 'x', namespace: 'default' }
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.queryByText('Reconcile every')).not.toBeInTheDocument()
    })
  })

  describe('Reconcile timeout', () => {
    it('should display timeout from spec.timeout', () => {
      const reconciler = {
        ...mockReconciler,
        spec: { interval: '10m', timeout: '2m' }
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('(timeout 2m)')).toBeInTheDocument()
    })

    it('should display default timeout 1m for Source types', () => {
      const reconciler = {
        apiVersion: 'source.toolkit.fluxcd.io/v1beta2',
        kind: 'GitRepository',
        metadata: { name: 'git', namespace: 'flux-system' },
        spec: { interval: '10m' },
        status: {}
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('(timeout 1m)')).toBeInTheDocument()
    })

    it('should display timeout from spec.interval for Kustomization', () => {
      render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('(timeout 10m)')).toBeInTheDocument()
    })

    it('should display default timeout 5m for HelmRelease', () => {
      const reconciler = {
        apiVersion: 'helm.toolkit.fluxcd.io/v2beta1',
        kind: 'HelmRelease',
        metadata: { name: 'app', namespace: 'flux-system' },
        spec: { interval: '10m' },
        status: {}
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
    })

    it('should display default timeout 5m for FluxInstance', () => {
      const reconciler = {
        ...mockReconciler,
        kind: 'FluxInstance',
        metadata: { name: 'flux', namespace: 'flux-system' },
        spec: {}
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
    })

    it('should display timeout from annotation for FluxInstance', () => {
      const reconciler = {
        ...mockReconciler,
        kind: 'FluxInstance',
        metadata: {
          name: 'flux',
          namespace: 'flux-system',
          annotations: {
            'fluxcd.controlplane.io/reconcileEvery': '1m',
            'fluxcd.controlplane.io/reconcileTimeout': '10m'
          }
        },
        spec: {}
      }
      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )
      expect(screen.getByText('(timeout 10m)')).toBeInTheDocument()
    })
  })

  describe('Source tab', () => {
    it('should switch to Source tab and display source info', async () => {
      const user = userEvent.setup()

      render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )

      await user.click(screen.getByText('Source'))

      // Name label with source link
      expect(screen.getByText('Name')).toBeInTheDocument()
      const sourceLinks = screen.getAllByText(/GitRepository/)
      expect(sourceLinks.length).toBeGreaterThan(0)

      // Status badge
      const readyBadges = screen.getAllByText('Ready')
      expect(readyBadges.length).toBeGreaterThan(0)

      // URL
      expect(screen.getByText('https://github.com/example/repo')).toBeInTheDocument()

      // Fetch result
      expect(screen.getByText('stored artifact for revision main@sha1:abc123')).toBeInTheDocument()
    })

    it('should not show Source tab when sourceRef is missing', () => {
      const reconciler = {
        ...mockReconciler,
        status: {
          reconcilerRef: mockReconciler.status.reconcilerRef
        }
      }

      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )

      expect(screen.queryByText('Source')).not.toBeInTheDocument()
    })

    it('should display origin URL when present', async () => {
      const user = userEvent.setup()
      const reconciler = {
        ...mockReconciler,
        status: {
          ...mockReconciler.status,
          sourceRef: {
            ...mockReconciler.status.sourceRef,
            originURL: 'https://github.com/fork/repo',
            originRevision: 'feature-branch/def456'
          }
        }
      }

      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )

      await user.click(screen.getByText('Source'))

      expect(screen.getByText('https://github.com/fork/repo')).toBeInTheDocument()
      expect(screen.getByText('feature-branch/def456')).toBeInTheDocument()
    })
  })

  describe('Events tab', () => {
    it('should fetch and display events when Events tab is clicked', async () => {
      const user = userEvent.setup()
      const mockEvents = {
        events: [
          {
            type: 'Normal',
            reason: 'Reconciled',
            message: 'Reconciliation finished',
            lastTimestamp: '2023-01-01T12:05:00Z'
          },
          {
            type: 'Warning',
            reason: 'Failed',
            message: 'Something went wrong',
            lastTimestamp: '2023-01-01T12:10:00Z'
          }
        ]
      }

      fetchWithMock.mockResolvedValueOnce(mockEvents)

      render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )

      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledWith(expect.objectContaining({
          endpoint: expect.stringContaining('/api/v1/events')
        }))
      })

      await waitFor(() => {
        expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
        expect(screen.getByText('Something went wrong')).toBeInTheDocument()
      })

      const infoBadges = screen.getAllByText('Info')
      expect(infoBadges.length).toBeGreaterThan(0)
      expect(screen.getByText('Warning')).toBeInTheDocument()
    })

    it('should display "No events found" when events list is empty', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockResolvedValueOnce({ events: [] })

      render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )

      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(screen.getByText('No events found')).toBeInTheDocument()
      })
    })

    it('should handle fetch error gracefully', async () => {
      const user = userEvent.setup()
      fetchWithMock.mockRejectedValueOnce(new Error('Network error'))
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )

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
          reason: 'Reconciled',
          message: 'Reconciliation finished',
          lastTimestamp: '2023-01-01T12:05:00Z'
        }
      ]
    }

    it('should refetch events when workloadData changes if Events tab is open', async () => {
      const user = userEvent.setup()

      const { rerender } = render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )

      fetchWithMock.mockResolvedValueOnce(mockEvents)
      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
      })

      fetchWithMock.mockResolvedValueOnce({
        events: [
          {
            type: 'Normal',
            reason: 'Reconciled',
            message: 'New reconciliation after refresh',
            lastTimestamp: '2023-01-01T13:05:00Z'
          }
        ]
      })

      rerender(
        <WorkloadReconcilerPanel
          reconciler={mockReconciler}
          workloadData={{ metadata: { name: 'nginx-updated' } }}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('New reconciliation after refresh')).toBeInTheDocument()
      })
      expect(fetchWithMock.mock.calls.length).toBeGreaterThanOrEqual(2)
    })

    it('should NOT refetch events when workloadData changes if Events tab is not open', async () => {
      const { rerender } = render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )

      expect(fetchWithMock).not.toHaveBeenCalled()

      rerender(
        <WorkloadReconcilerPanel
          reconciler={mockReconciler}
          workloadData={{ metadata: { name: 'nginx-updated' } }}
        />
      )

      await waitFor(() => {
        expect(fetchWithMock).not.toHaveBeenCalled()
      })
    })

    it('should NOT double-fetch events on initial mount', async () => {
      const user = userEvent.setup()

      render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )

      fetchWithMock.mockResolvedValueOnce(mockEvents)
      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })
    })

    it('should preserve event data when refetch fails during auto-refresh', async () => {
      const user = userEvent.setup()
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      const { rerender } = render(
        <WorkloadReconcilerPanel reconciler={mockReconciler} workloadData={mockWorkloadData} />
      )

      fetchWithMock.mockResolvedValueOnce(mockEvents)
      await user.click(screen.getByText('Events'))

      await waitFor(() => {
        expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
      })

      fetchWithMock.mockRejectedValueOnce(new Error('Network error'))

      rerender(
        <WorkloadReconcilerPanel
          reconciler={mockReconciler}
          workloadData={{ metadata: { name: 'nginx-updated' } }}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
      })

      consoleSpy.mockRestore()
    })
  })

  describe('Suspended by', () => {
    it('should display "Suspended by" when status is Suspended and annotation exists', () => {
      const reconciler = {
        ...mockReconciler,
        metadata: {
          ...mockReconciler.metadata,
          annotations: {
            'fluxcd.controlplane.io/suspendedBy': 'test-user@example.com'
          }
        },
        status: {
          ...mockReconciler.status,
          reconcilerRef: {
            ...mockReconciler.status.reconcilerRef,
            status: 'Suspended'
          }
        }
      }

      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )

      expect(screen.getByText('Suspended by')).toBeInTheDocument()
      expect(screen.getByText('test-user@example.com')).toBeInTheDocument()
    })

    it('should NOT display "Suspended by" when status is not Suspended', () => {
      const reconciler = {
        ...mockReconciler,
        metadata: {
          ...mockReconciler.metadata,
          annotations: {
            'fluxcd.controlplane.io/suspendedBy': 'test-user@example.com'
          }
        }
      }

      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )

      expect(screen.queryByText('Suspended by')).not.toBeInTheDocument()
    })

    it('should NOT display "Suspended by" when annotation does not exist', () => {
      const reconciler = {
        ...mockReconciler,
        status: {
          ...mockReconciler.status,
          reconcilerRef: {
            ...mockReconciler.status.reconcilerRef,
            status: 'Suspended'
          }
        }
      }

      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )

      expect(screen.getByText('Suspended')).toBeInTheDocument()
      expect(screen.queryByText('Suspended by')).not.toBeInTheDocument()
    })
  })

  describe('Edge cases', () => {
    it('should use condition message when reconcilerRef message is missing', () => {
      const reconciler = {
        ...mockReconciler,
        status: {
          ...mockReconciler.status,
          reconcilerRef: undefined,
          conditions: [
            {
              type: 'Ready',
              status: 'True',
              message: 'Message from condition',
              lastTransitionTime: '2023-01-01T12:00:00Z'
            }
          ]
        }
      }

      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )

      expect(screen.getByText('Message from condition')).toBeInTheDocument()
    })

    it('should show Unknown status when reconcilerRef is missing', () => {
      const reconciler = {
        ...mockReconciler,
        status: {
          conditions: []
        }
      }

      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )

      expect(screen.getByText('Unknown')).toBeInTheDocument()
    })

    it('should not show message section when message is empty', () => {
      const reconciler = {
        ...mockReconciler,
        status: {
          ...mockReconciler.status,
          reconcilerRef: {
            status: 'Ready',
            message: '',
            lastReconciled: '2023-01-01T12:00:00Z'
          },
          conditions: []
        }
      }

      render(
        <WorkloadReconcilerPanel reconciler={reconciler} workloadData={mockWorkloadData} />
      )

      expect(screen.queryByText('Last action')).not.toBeInTheDocument()
    })
  })
})
