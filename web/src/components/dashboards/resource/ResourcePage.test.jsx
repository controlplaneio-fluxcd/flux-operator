// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import { ResourcePage } from './ResourcePage'
import { fetchWithMock } from '../../../utils/fetch'

// Mock fetchWithMock
vi.mock('../../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock preact-iso
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    route: vi.fn()
  })
}))

// Mock child components to simplify testing and avoid cascading failures
vi.mock('./ReconcilerPanel', () => ({
  ReconcilerPanel: ({ kind, name, namespace }) => (
    <div data-testid="reconciler-panel">
      ReconcilerPanel: {kind}/{namespace}/{name}
    </div>
  )
}))

vi.mock('./InventoryPanel', () => ({
  InventoryPanel: ({ resourceData }) => (
    <div data-testid="inventory-panel">
      InventoryPanel: {resourceData?.metadata?.name}
    </div>
  )
}))

vi.mock('./SourcePanel', () => ({
  SourcePanel: ({ sourceRef }) => (
    <div data-testid="source-panel">
      SourcePanel: {sourceRef?.name}
    </div>
  )
}))

describe('ResourcePage component', () => {
  const mockResourceData = {
    apiVersion: 'fluxcd.controlplane.io/v1',
    kind: 'FluxInstance',
    metadata: {
      name: 'flux',
      namespace: 'flux-system',
      creationTimestamp: '2023-01-01T00:00:00Z'
    },
    spec: {
      interval: '1m'
    },
    status: {
      reconcilerRef: {
        status: 'Ready',
        message: 'Reconciliation succeeded',
        lastReconciled: '2023-01-01T12:00:00Z',
        managedBy: ''
      },
      conditions: [
        {
          type: 'Ready',
          status: 'True',
          message: 'Reconciliation succeeded',
          lastTransitionTime: '2023-01-01T12:00:00Z'
        }
      ],
      sourceRef: {
        kind: 'GitRepository',
        name: 'flux-system',
        namespace: 'flux-system'
      }
    }
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render loading state with hero section', () => {
    // Return a promise that never resolves immediately to check loading state
    fetchWithMock.mockReturnValue(new Promise(() => {}))

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    // Hero section should be visible with kind, name, namespace
    expect(screen.getByText('FluxInstance')).toBeInTheDocument()
    expect(screen.getByText('flux')).toBeInTheDocument()
    expect(screen.getByText('Namespace: flux-system')).toBeInTheDocument()

    // Loading message should appear below hero
    expect(screen.getByTestId('loading-message')).toBeInTheDocument()
    expect(screen.getByText('Loading resource data...')).toBeInTheDocument()

    // Hero should have blue border for loading state (same as Progressing)
    const headerCard = screen.getByText('flux').closest('.card')
    expect(headerCard).toHaveClass('border-blue-500')
  })

  it('should render error state with hero section when fetch fails', async () => {
    fetchWithMock.mockRejectedValue(new Error('API Error'))

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      // Hero section should be visible with kind, name, namespace
      expect(screen.getByText('FluxInstance')).toBeInTheDocument()
      expect(screen.getByText('flux')).toBeInTheDocument()
      expect(screen.getByText('Namespace: flux-system')).toBeInTheDocument()

      // Error message should appear below hero
      expect(screen.getByTestId('error-message')).toBeInTheDocument()
      expect(screen.getByText('Failed to load resource: API Error')).toBeInTheDocument()

      // Hero should have danger border for error state
      const headerCard = screen.getByText('flux').closest('.card')
      expect(headerCard).toHaveClass('border-danger')
    })
  })

  it('should render not found state with hero section when no data returned', async () => {
    // Mock resource fetch returning null (not found)
    fetchWithMock.mockResolvedValueOnce(null)

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      // Hero section should be visible with kind, name, namespace
      expect(screen.getByText('FluxInstance')).toBeInTheDocument()
      expect(screen.getByText('flux')).toBeInTheDocument()
      expect(screen.getByText('Namespace: flux-system')).toBeInTheDocument()

      // Not found message should appear below hero
      expect(screen.getByTestId('not-found-message')).toBeInTheDocument()
      expect(screen.getByText('Resource not found in the cluster.')).toBeInTheDocument()

      // Hero should have gray border for not found state (different from error)
      const headerCard = screen.getByText('flux').closest('.card')
      expect(headerCard).toHaveClass('border-gray-400')
    })
  })

  it('should render not found state with hero section when empty object returned', async () => {
    // Mock resource fetch returning empty object (server returns {} for not found)
    fetchWithMock.mockResolvedValueOnce({})

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      // Hero section should be visible with kind, name, namespace
      expect(screen.getByText('FluxInstance')).toBeInTheDocument()
      expect(screen.getByText('flux')).toBeInTheDocument()
      expect(screen.getByText('Namespace: flux-system')).toBeInTheDocument()

      // Not found message should appear below hero
      expect(screen.getByTestId('not-found-message')).toBeInTheDocument()
      expect(screen.getByText('Resource not found in the cluster.')).toBeInTheDocument()

      // Hero should have gray border for not found state (different from error)
      const headerCard = screen.getByText('flux').closest('.card')
      expect(headerCard).toHaveClass('border-gray-400')
    })
  })

  it('should render resource header and panels on success', async () => {
    // Mock resource fetch
    fetchWithMock.mockResolvedValueOnce(mockResourceData)

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    // Check Header
    await waitFor(() => {
      expect(screen.getByText('flux')).toBeInTheDocument()
    })
    // Note: The text in DOM is 'FluxInstance', CSS makes it uppercase
    expect(screen.getByText('FluxInstance')).toBeInTheDocument()
    expect(screen.getByText('Namespace: flux-system')).toBeInTheDocument()

    // Check Status Icon presence (Ready status)
    const iconContainer = screen.getByText('flux').closest('.card').querySelector('.bg-green-50')
    expect(iconContainer).toBeInTheDocument()

    // Check Child Panels using mocked components
    expect(screen.getByTestId('reconciler-panel')).toHaveTextContent('ReconcilerPanel: FluxInstance/flux-system/flux')
    expect(screen.getByTestId('inventory-panel')).toHaveTextContent('InventoryPanel: flux')
    expect(screen.getByTestId('source-panel')).toHaveTextContent('SourcePanel: flux-system')
  })

  it('should render correct status style for Failed status', async () => {
    const failedData = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        reconcilerRef: {
          ...mockResourceData.status.reconcilerRef,
          status: 'Failed'
        }
      }
    }

    fetchWithMock.mockResolvedValueOnce(failedData)

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      // Check for red background class associated with Failed status
      const headerCard = screen.getByText('flux').closest('.card')
      expect(headerCard).toHaveClass('bg-red-50')
      expect(headerCard).toHaveClass('border-danger')
    })
  })

  it('should render correct status style for Progressing status', async () => {
    const progressingData = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        reconcilerRef: {
          ...mockResourceData.status.reconcilerRef,
          status: 'Progressing'
        }
      }
    }

    fetchWithMock.mockResolvedValueOnce(progressingData)

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      // Check for blue background class associated with Progressing status
      const headerCard = screen.getByText('flux').closest('.card')
      expect(headerCard).toHaveClass('bg-blue-50')
      expect(headerCard).toHaveClass('border-blue-500')
    })
  })

  it('should render correct status style for Suspended status', async () => {
    const suspendedData = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        reconcilerRef: {
          ...mockResourceData.status.reconcilerRef,
          status: 'Suspended'
        }
      }
    }

    fetchWithMock.mockResolvedValueOnce(suspendedData)

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      // Check for yellow background class associated with Suspended status
      const headerCard = screen.getByText('flux').closest('.card')
      expect(headerCard).toHaveClass('bg-yellow-50')
      expect(headerCard).toHaveClass('border-yellow-500')
    })
  })

  it('should not render SourcePanel if sourceRef is missing', async () => {
    const dataNoSource = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        sourceRef: null
      }
    }

    fetchWithMock.mockResolvedValueOnce(dataNoSource)

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      expect(screen.getByTestId('reconciler-panel')).toBeInTheDocument()
    })

    expect(screen.queryByTestId('source-panel')).not.toBeInTheDocument()
  })

  it('should handle missing reconcilerRef data gracefully', async () => {
    // Mock resource data but without reconcilerRef
    const dataNoReconcilerRef = {
      ...mockResourceData,
      status: {
        ...mockResourceData.status,
        reconcilerRef: null
      }
    }

    fetchWithMock.mockResolvedValueOnce(dataNoReconcilerRef)

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      expect(screen.getByText('flux')).toBeInTheDocument()
    })

    // Should default to 'Unknown' status styling (gray)
    const headerCard = screen.getByText('flux').closest('.card')
    expect(headerCard).toHaveClass('bg-gray-50')
  })

  describe('Auto-refresh functionality', () => {
    beforeEach(() => {
      vi.useFakeTimers()
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    it('should fetch data on mount and setup auto-refresh interval', async () => {
      fetchWithMock.mockResolvedValue(mockResourceData)

      render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      // Initial fetch should happen
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1) // resource only
      })

      // Clear mock call history
      fetchWithMock.mockClear()

      // Mock stays the same for auto-refresh
      fetchWithMock.mockResolvedValue(mockResourceData)

      // Fast-forward 30 seconds
      vi.advanceTimersByTime(30000)

      // Auto-refresh should trigger
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1) // resource only
      })
    })

    it('should set lastUpdatedAt timestamp on successful fetch', async () => {
      const now = new Date('2023-01-01T12:30:00Z')
      vi.setSystemTime(now)

      fetchWithMock.mockResolvedValueOnce(mockResourceData)

      render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument()
      })

      // Check that "Last Updated" header is displayed
      expect(screen.getByText('Last Updated')).toBeInTheDocument()
    })

    it('should preserve existing data when auto-refresh fails', async () => {
      // Initial successful fetch
      fetchWithMock.mockResolvedValue(mockResourceData)

      render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument()
      })

      // Clear mock and setup failure for auto-refresh
      fetchWithMock.mockClear()
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      // Fast-forward to trigger auto-refresh
      vi.advanceTimersByTime(30000)

      // Wait for fetch to be called
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalled()
      })

      // Content should still be visible (not replaced with error screen)
      expect(screen.getByText('flux')).toBeInTheDocument()
      expect(screen.queryByText('Failed to load resource: Network error')).not.toBeInTheDocument()
    })

    it('should only show loading message on initial load, not on auto-refresh', async () => {
      fetchWithMock.mockResolvedValue(mockResourceData)

      render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      // Initial load should show loading message below hero
      expect(screen.getByTestId('loading-message')).toBeInTheDocument()
      expect(screen.getByText('Loading resource data...')).toBeInTheDocument()

      // Wait for initial load to complete
      await waitFor(() => {
        expect(screen.queryByTestId('loading-message')).not.toBeInTheDocument()
      })

      // Fast-forward to trigger auto-refresh
      vi.advanceTimersByTime(30000)

      // Content should remain visible during refresh (no loading message)
      expect(screen.queryByTestId('loading-message')).not.toBeInTheDocument()
      expect(screen.getByText('flux')).toBeInTheDocument()
    })

    it('should clear interval on unmount', async () => {
      fetchWithMock.mockResolvedValue(mockResourceData)

      const { unmount } = render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument()
      })

      // Clear mock history
      fetchWithMock.mockClear()

      // Unmount component
      unmount()

      // Fast-forward time - should NOT trigger fetch
      vi.advanceTimersByTime(30000)

      // Fetch should NOT be called after unmount
      expect(fetchWithMock).not.toHaveBeenCalled()
    })

    it('should restart interval when route parameters change', async () => {
      fetchWithMock.mockResolvedValue(mockResourceData)

      const { rerender } = render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument()
      })

      // Clear mock history
      fetchWithMock.mockClear()

      // Change route parameter (different resource)
      const newResourceData = { ...mockResourceData, metadata: { ...mockResourceData.metadata, name: 'flux-2' } }
      fetchWithMock.mockResolvedValue(newResourceData)

      // Rerender with different name
      rerender(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux-2" />)

      // Should fetch immediately for new resource
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1) // resource only
      })
    })
  })
})
