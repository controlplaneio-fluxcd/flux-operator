// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { ReconcilerPanel } from './ReconcilerPanel'
import { fetchWithMock } from '../../utils/fetch'

// Mock fetchWithMock
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
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

  const mockOverviewData = {
    status: 'Ready',
    message: 'Applied revision: main/1234567',
    lastReconciled: '2023-01-01T12:00:00Z'
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
        overviewData={mockOverviewData}
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
        overviewData={mockOverviewData}
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
        overviewData={mockOverviewData}
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
        overviewData={mockOverviewData}
      />
    )

    // Status
    expect(screen.getByText('Status:')).toBeInTheDocument()
    expect(screen.getByText('Ready')).toBeInTheDocument()

    // Managed by
    expect(screen.getByText('Managed by:')).toBeInTheDocument()
    
    // ID
    expect(screen.getByText('ID:')).toBeInTheDocument()
    expect(screen.getByText('FluxInstance/flux-system/flux')).toBeInTheDocument()

    // Reconcile every
    expect(screen.getByText('Reconcile every:')).toBeInTheDocument()
    expect(screen.getByText('1m')).toBeInTheDocument()

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
        overviewData={mockOverviewData}
      />
    )
    expect(screen.getByText('Reconcile every:')).toBeInTheDocument()
    expect(screen.getByText('1m')).toBeInTheDocument()
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
        overviewData={mockOverviewData}
      />
    )
    expect(screen.getByText('5m')).toBeInTheDocument()
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
        overviewData={mockOverviewData}
      />
    )
    expect(screen.getByText('60m')).toBeInTheDocument()
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
        overviewData={mockOverviewData}
      />
    )
    expect(screen.getByText('10m')).toBeInTheDocument()
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
        overviewData={mockOverviewData}
      />
    )
    expect(screen.queryByText('Reconcile every:')).not.toBeInTheDocument()
  })

  it('should switch to Specification tab', async () => {
    const user = userEvent.setup()

    render(
      <ReconcilerPanel
        kind="FluxInstance"
        name="flux"
        namespace="flux-system"
        resourceData={mockResourceData}
        overviewData={mockOverviewData}
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
        overviewData={mockOverviewData}
      />
    )

    const statusTab = screen.getByText('Status')
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
        overviewData={mockOverviewData}
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

    // Check badges (Info/Warning)
    expect(screen.getByText('Info')).toBeInTheDocument()
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
        overviewData={mockOverviewData}
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
        overviewData={mockOverviewData}
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
          overviewData={mockOverviewData}
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
          overviewData={mockOverviewData}
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
          overviewData={mockOverviewData}
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
          overviewData={mockOverviewData}
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
          overviewData={mockOverviewData}
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
          overviewData={mockOverviewData}
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
          overviewData={mockOverviewData}
        />
      )

      // Should preserve existing events
      await waitFor(() => {
        expect(screen.getByText('Reconciliation finished')).toBeInTheDocument()
      })

      consoleSpy.mockRestore()
    })
  })
})
