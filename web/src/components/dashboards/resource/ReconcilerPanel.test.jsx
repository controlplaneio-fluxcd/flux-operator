// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ReconcilerPanel } from './ReconcilerPanel'
import { fetchWithMock } from '../../../utils/fetch'

// Mock fetchWithMock
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    route: mockRoute
  })
}))

describe('ReconcilerPanel component', () => {
  const mockResourceData = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'FluxInstance',
    metadata: {
      name: 'flux',
      namespace: 'flux-system',
      annotations: {
        'fluxcd.controlplane.io/reconcileEvery': '1m'
      }
    },
    spec: {
      distribution: {
        version: '2.0.0'
      },
      components: [
        'source-controller',
        'kustomize-controller'
      ]
    },
    status: {
      reconcilerRef: {
        status: 'Ready',
        message: 'Applied revision: main/1234567',
        lastReconciled: '2023-01-01T12:00:00Z',
        managedBy: ''
      },
      conditions: [
        {
          type: 'Ready',
          status: 'True',
          reason: 'ReconciliationSucceeded',
          message: 'Applied revision: main/1234567',
          lastTransitionTime: '2023-01-01T12:00:00Z'
        }
      ],
      inventory: [], // Should be filtered out in status tab
      sourceRef: {
        kind: 'GitRepository',
        name: 'flux-system'
      }
    }
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the Reconciler title', () => {
    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    expect(screen.getByText('Reconciler')).toBeInTheDocument()
  })

  it('should be expanded by default', () => {
    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByText('Events')).toBeInTheDocument()
  })

  it('should toggle collapse/expand state', async () => {
    const user = userEvent.setup()

    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    // Initially expanded
    expect(screen.getByText('Overview')).toBeInTheDocument()

    // Click header to collapse
    const headerButton = screen.getByText('Reconciler').closest('button')
    await user.click(headerButton)

    // Content should be hidden
    expect(screen.queryByText('Overview')).not.toBeInTheDocument()

    // Click to expand again
    await user.click(headerButton)

    // Content should be visible
    expect(screen.getByText('Overview')).toBeInTheDocument()
  })

  it('should display overview tab content', () => {
    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    // Status
    expect(screen.getAllByText('Status').length).toBeGreaterThan(0)
    expect(screen.getByText('Ready')).toBeInTheDocument()

    // Reconciled by
    expect(screen.getByText('Reconciled by')).toBeInTheDocument()

    // Reconcile every
    expect(screen.getByText('Reconcile every')).toBeInTheDocument()
    expect(screen.getByText('1m')).toBeInTheDocument()
    expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()

    // Message
    expect(screen.getByText(/Applied revision: main\/1234567/)).toBeInTheDocument()
  })

  it('should calculate reconcile interval from spec', () => {
    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )
    expect(screen.getByText('Reconcile every')).toBeInTheDocument()
    expect(screen.getByText('1m')).toBeInTheDocument()
    expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
  })

  it('should calculate reconcile interval from spec.interval', () => {
    const dataWithSpecInterval = {
      ...mockResourceData,
      metadata: { ...mockResourceData.metadata, annotations: {} },
      spec: { interval: '5m' }
    }
    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={dataWithSpecInterval}
      />
    )
    expect(screen.getByText('5m')).toBeInTheDocument()
    expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
  })

  it('should calculate reconcile interval from default for ResourceSet', () => {
    const dataResourceSet = {
      ...mockResourceData,
      kind: 'ResourceSet',
      metadata: { ...mockResourceData.metadata, annotations: {} },
      spec: {}
    }
    render(
      <ReconcilerPanel
        kind="ResourceSet"
        name="flux"
        namespace="flux-system"
        resourceData={dataResourceSet}
      />
    )
    expect(screen.getByText('60m')).toBeInTheDocument()
    expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
  })

  it('should calculate reconcile interval from default for ResourceSetInputProvider', () => {
    const dataInputProvider = {
      ...mockResourceData,
      kind: 'ResourceSetInputProvider',
      metadata: { ...mockResourceData.metadata, annotations: {} },
      spec: {}
    }
    render(
      <ReconcilerPanel
        kind="ResourceSetInputProvider"
        name="flux"
        namespace="flux-system"
        resourceData={dataInputProvider}
      />
    )
    expect(screen.getByText('10m')).toBeInTheDocument()
    expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
  })

  it('should not show reconcile interval if unknown', () => {
    const dataUnknown = {
      ...mockResourceData,
      kind: 'UnknownKind',
      metadata: { ...mockResourceData.metadata, annotations: {} },
      spec: {}
    }
    render(
      <ReconcilerPanel
        kind="UnknownKind"
        name="flux"
        namespace="flux-system"
        resourceData={dataUnknown}
      />
    )
    expect(screen.queryByText('Reconcile every')).not.toBeInTheDocument()
  })

  it('should display "Managed by" field when reconcilerRef.managedBy exists', () => {
    const dataWithReconcilerRef = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        reconcilerRef: {
          ...mockResourceData.status.reconcilerRef,
          managedBy: 'FluxInstance/flux-system/flux'
        }
      }
    }
    render(
      <ReconcilerPanel
        kind="ResourceSet"
        name="apps"
        namespace="flux-system"
        resourceData={dataWithReconcilerRef}
      />
    )

    expect(screen.getByText('Managed by')).toBeInTheDocument()
    expect(screen.getByText('FluxInstance/flux-system/flux')).toBeInTheDocument()
  })

  it('should not display "Managed by" field when reconcilerRef.managedBy does not exist', () => {
    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    expect(screen.queryByText('Managed by')).not.toBeInTheDocument()
  })

  it('should navigate to reconciler resource when clicking "Managed by" link', () => {
    const dataWithReconcilerRef = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        reconcilerRef: {
          ...mockResourceData.status.reconcilerRef,
          managedBy: 'FluxInstance/flux-system/flux'
        }
      }
    }

    render(
      <ReconcilerPanel
        kind="ResourceSet"
        name="apps"
        namespace="flux-system"
        resourceData={dataWithReconcilerRef}
      />
    )

    const managedByLink = screen.getByText('FluxInstance/flux-system/flux')
    expect(managedByLink.closest('a')).toHaveAttribute('href', '/resource/FluxInstance/flux-system/flux')
  })

  it('should switch to Specification tab', async () => {
    const user = userEvent.setup()

    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    const specTab = screen.getByText('Specification')
    await user.click(specTab)

    // YamlBlock uses Prism which splits text.
    // We check if key parts are present in the document using regex for partial matching.
    expect(screen.getAllByText(/version/).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/2\.0\.0/).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/source/).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/controller/).length).toBeGreaterThan(0)
  })

  it('should switch to Status tab', async () => {
    const user = userEvent.setup()

    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    const statusTab = screen.getByRole('button', { name: 'Status' })
    await user.click(statusTab)

    // Should see yaml content
    expect(screen.getByText(/lastTransitionTime/)).toBeInTheDocument()
    // Inventory should be filtered out in status tab
    expect(screen.queryByText(/inventory:/)).not.toBeInTheDocument()
  })

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
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    const eventsTab = screen.getByText('Events')
    await user.click(eventsTab)

    // Verify loading state
    expect(fetchWithMock).toHaveBeenCalledWith(expect.objectContaining({
      endpoint: expect.stringContaining('/api/v1/events')
    }))

    // Wait for events to display
    await waitFor(() => {
      expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
      expect(screen.getByText('Something went wrong')).toBeInTheDocument()
    })

    // Check badges (Info/Warning) - using getAllByText since "Info" also appears in tab label
    const infoBadges = screen.getAllByText('Info')
    expect(infoBadges.length).toBeGreaterThan(0)
    expect(screen.getByText('Warning')).toBeInTheDocument()
  })

  it('should display "No events found" when events list is empty', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockResolvedValueOnce({ events: [] })

    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    const eventsTab = screen.getByText('Events')
    await user.click(eventsTab)

    await waitFor(() => {
      expect(screen.getByText('No events found')).toBeInTheDocument()
    })
  })

  it('should handle fetch error gracefully', async () => {
    const user = userEvent.setup()
    fetchWithMock.mockRejectedValueOnce(new Error('Network error'))
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
      />
    )

    const eventsTab = screen.getByText('Events')
    await user.click(eventsTab)

    await waitFor(() => {
      expect(screen.getByText('No events found')).toBeInTheDocument()
    })

    expect(consoleSpy).toHaveBeenCalled()
    consoleSpy.mockRestore()
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

    it('should refetch events when resourceData changes if Events tab is open', async () => {
      const user = userEvent.setup()

      // Initial render
      const { rerender } = render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceData}
        />
      )

      // Click on Events tab
      fetchWithMock.mockResolvedValueOnce(mockEvents)
      const eventsTab = screen.getByText('Events')
      await user.click(eventsTab)

      // Wait for events to load
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
        expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
      })

      // Simulate parent auto-refresh by changing resourceData
      const updatedResourceData = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          conditions: [
            {
              type: 'Ready',
              status: 'True',
              reason: 'ReconciliationSucceeded',
              message: 'Applied revision: main/9999999',
              lastTransitionTime: '2023-01-01T13:00:00Z'
            }
          ]
        }
      }

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
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={updatedResourceData}
        />
      )

      // Should refetch events
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2) // events, events (refresh)
        expect(screen.getByText('New reconciliation after refresh')).toBeInTheDocument()
      })
    })

    it('should NOT refetch events when resourceData changes if Events tab is not open', async () => {
      // Initial render (on Overview tab)
      const { rerender } = render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceData}
        />
      )

      // No events should be fetched yet
      expect(fetchWithMock).not.toHaveBeenCalled()

      // Simulate parent auto-refresh by changing resourceData
      const updatedResourceData = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          conditions: [
            {
              type: 'Ready',
              status: 'True',
              reason: 'ReconciliationSucceeded',
              message: 'Applied revision: main/9999999',
              lastTransitionTime: '2023-01-01T13:00:00Z'
            }
          ]
        }
      }

      rerender(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={updatedResourceData}
        />
      )

      // Should NOT fetch events
      await waitFor(() => {
        expect(fetchWithMock).not.toHaveBeenCalled()
      })
    })

    it('should NOT refetch events on initial mount when Events tab is opened', async () => {
      const user = userEvent.setup()

      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceData}
        />
      )

      // Click on Events tab
      fetchWithMock.mockResolvedValueOnce(mockEvents)
      const eventsTab = screen.getByText('Events')
      await user.click(eventsTab)

      // Events should be fetched only once (not twice due to auto-refresh effect)
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1) // events (NOT events again)
      })
    })

    it('should preserve event data when refetch fails during auto-refresh', async () => {
      const user = userEvent.setup()
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      // Initial render
      const { rerender } = render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceData}
        />
      )

      // Click on Events tab
      fetchWithMock.mockResolvedValueOnce(mockEvents)
      const eventsTab = screen.getByText('Events')
      await user.click(eventsTab)

      // Wait for events to load
      await waitFor(() => {
        expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
      })

      // Simulate parent auto-refresh with events fetch error
      const updatedResourceData = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          conditions: [
            {
              type: 'Ready',
              status: 'True',
              reason: 'ReconciliationSucceeded',
              message: 'Applied revision: main/9999999',
              lastTransitionTime: '2023-01-01T13:00:00Z'
            }
          ]
        }
      }

      fetchWithMock.mockRejectedValueOnce(new Error('Network error'))

      rerender(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={updatedResourceData}
        />
      )

      // Should preserve existing events
      await waitFor(() => {
        expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
      })

      consoleSpy.mockRestore()
    })
  })

  describe('Reconcile timeout', () => {
    it('should display timeout from spec.timeout', () => {
      const data = {
        ...mockResourceData,
        spec: { ...mockResourceData.spec, interval: '1m', timeout: '2m' }
      }
      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={data}
        />
      )
      expect(screen.getByText('1m')).toBeInTheDocument()
      expect(screen.getByText('(timeout 2m)')).toBeInTheDocument()
    })

    it('should display default timeout 1m for Source types', () => {
      const data = {
        apiVersion: 'source.toolkit.fluxcd.io/v1beta2',
        kind: 'GitRepository',
        metadata: { name: 'git', namespace: 'flux-system' },
        spec: { interval: '10m' },
        status: {}
      }
      render(
        <ReconcilerPanel
          kind="GitRepository"
          name="git"
          namespace="flux-system"
          resourceData={data}
        />
      )
      expect(screen.getByText('10m')).toBeInTheDocument()
      expect(screen.getByText('(timeout 1m)')).toBeInTheDocument()
    })

    it('should display default timeout from spec.interval for Kustomization', () => {
      const data = {
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        kind: 'Kustomization',
        metadata: { name: 'app', namespace: 'flux-system' },
        spec: { interval: '5m' },
        status: {}
      }
      render(
        <ReconcilerPanel
          kind="Kustomization"
          name="app"
          namespace="flux-system"
          resourceData={data}
        />
      )
      expect(screen.getAllByText('5m').length).toBeGreaterThan(0)
      expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
    })

    it('should display default timeout 5m for HelmRelease', () => {
      const data = {
        apiVersion: 'helm.toolkit.fluxcd.io/v2beta1',
        kind: 'HelmRelease',
        metadata: { name: 'app', namespace: 'flux-system' },
        spec: { interval: '10m' },
        status: {}
      }
      render(
        <ReconcilerPanel
          kind="HelmRelease"
          name="app"
          namespace="flux-system"
          resourceData={data}
        />
      )
      expect(screen.getByText('10m')).toBeInTheDocument()
      expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
    })

    it('should display default timeout 5m for FluxInstance', () => {
      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceData}
        />
      )
      // mockResourceData has annotation reconcileEvery: 1m
      expect(screen.getByText('1m')).toBeInTheDocument()
      expect(screen.getByText('(timeout 5m)')).toBeInTheDocument()
    })

    it('should display timeout from annotation for FluxInstance', () => {
      const data = {
        ...mockResourceData,
        metadata: {
          ...mockResourceData.metadata,
          annotations: {
            'fluxcd.controlplane.io/reconcileEvery': '1m',
            'fluxcd.controlplane.io/reconcileTimeout': '10m'
          }
        }
      }
      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={data}
        />
      )
      expect(screen.getByText('1m')).toBeInTheDocument()
      expect(screen.getByText('(timeout 10m)')).toBeInTheDocument()
    })
  })

  describe('History Tab', () => {
    const mockResourceDataWithHistory = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        history: [
          {
            digest: 'sha256:abc123def456789',
            lastReconciledStatus: 'Succeeded',
            lastReconciledDuration: '1m30s',
            totalReconciliations: 5,
            firstReconciled: '2023-01-01T10:00:00Z',
            lastReconciled: '2023-01-01T12:00:00Z',
            metadata: { revision: 'main/abc123' }
          },
          {
            digest: 'sha256:def456789abc123',
            lastReconciledStatus: 'Failed',
            lastReconciledDuration: '30s',
            totalReconciliations: 2,
            firstReconciled: '2023-01-01T08:00:00Z',
            lastReconciled: '2023-01-01T09:00:00Z',
            metadata: { revision: 'main/def456' }
          }
        ]
      }
    }

    it('should show History tab when history exists', () => {
      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceDataWithHistory}
        />
      )

      expect(screen.getByText('History')).toBeInTheDocument()
    })

    it('should NOT show History tab when history is empty', () => {
      const dataWithEmptyHistory = {
        ...mockResourceData,
        status: {
          ...mockResourceData.status,
          history: []
        }
      }

      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={dataWithEmptyHistory}
        />
      )

      expect(screen.queryByText('History')).not.toBeInTheDocument()
    })

    it('should NOT show History tab when history is undefined', () => {
      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceData}
        />
      )

      expect(screen.queryByText('History')).not.toBeInTheDocument()
    })

    it('should switch to History tab and display timeline', async () => {
      const user = userEvent.setup()

      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceDataWithHistory}
        />
      )

      const historyTab = screen.getByText('History')
      await user.click(historyTab)

      // History tab should be active
      expect(historyTab).toHaveClass('border-flux-blue')

      // Should display history entries with status badges
      expect(screen.getByText('Succeeded')).toBeInTheDocument()
      expect(screen.getByText('Failed')).toBeInTheDocument()
    })

    it('should switch back to Overview tab from History', async () => {
      const user = userEvent.setup()

      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={mockResourceDataWithHistory}
        />
      )

      // Switch to History tab
      const historyTab = screen.getByText('History')
      await user.click(historyTab)

      // Switch back to Overview tab - use "Info" which is the mobile label for the same tab
      const overviewTab = screen.getByText('Info')
      await user.click(overviewTab)

      // Should show overview content
      expect(screen.getByText('Reconciled by')).toBeInTheDocument()
    })
  })

  describe('Edge cases', () => {
    it('should handle null resourceData', () => {
      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={null}
        />
      )

      expect(screen.getByText('Reconciler')).toBeInTheDocument()
      expect(screen.getByText('Unknown')).toBeInTheDocument()
    })

    it('should handle resourceData without status', () => {
      const dataWithoutStatus = {
        apiVersion: 'fluxcd.controlplane.io/v1',
        kind: 'FluxInstance',
        metadata: { name: 'flux', namespace: 'flux-system' },
        spec: { interval: '5m' }
      }

      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={dataWithoutStatus}
        />
      )

      expect(screen.getByText('Unknown')).toBeInTheDocument()
    })

    it('should use condition message when reconcilerRef message is missing', () => {
      const dataWithConditionMessage = {
        ...mockResourceData,
        status: {
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
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={dataWithConditionMessage}
        />
      )

      expect(screen.getByText('Message from condition')).toBeInTheDocument()
    })

    it('should use condition lastTransitionTime when reconcilerRef lastReconciled is missing', () => {
      const dataWithConditionTime = {
        ...mockResourceData,
        status: {
          conditions: [
            {
              type: 'Ready',
              status: 'True',
              message: 'Applied',
              lastTransitionTime: '2023-06-15T10:30:00Z'
            }
          ]
        }
      }

      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={dataWithConditionTime}
        />
      )

      // Should show the formatted date
      expect(screen.getByText(/Last action/)).toBeInTheDocument()
    })

    it('should not show message section when message is empty', () => {
      const dataWithoutMessage = {
        ...mockResourceData,
        status: {
          reconcilerRef: {
            status: 'Ready',
            message: '',
            lastReconciled: '2023-01-01T12:00:00Z'
          },
          conditions: []
        }
      }

      render(
        <ReconcilerPanel
          kind="FluxInstance"
          name="flux"
          namespace="flux-system"
          resourceData={dataWithoutMessage}
        />
      )

      expect(screen.queryByText('Last action')).not.toBeInTheDocument()
    })
  })
})
