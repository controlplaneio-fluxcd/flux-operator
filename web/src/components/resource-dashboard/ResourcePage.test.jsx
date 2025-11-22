// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import { ResourcePage } from './ResourcePage'
import { fetchWithMock } from '../../utils/fetch'

// Mock fetchWithMock
vi.mock('../../utils/fetch', () => ({
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

  const mockOverviewData = {
    status: 'Ready',
    message: 'Reconciliation succeeded',
    lastReconciled: '2023-01-01T12:00:00Z'
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render loading state initially', () => {
    // Return a promise that never resolves immediately to check loading state
    fetchWithMock.mockReturnValue(new Promise(() => {}))

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    expect(screen.getByText('Loading resource...')).toBeInTheDocument()
  })

  it('should render error state when fetch fails', async () => {
    fetchWithMock.mockRejectedValue(new Error('API Error'))

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      expect(screen.getByText('Failed to load resource: API Error')).toBeInTheDocument()
    })
  })

  it('should render not found state when no data returned', async () => {
    // Mock resource fetch returning null (not found), and overview fetch returning empty
    fetchWithMock
      .mockResolvedValueOnce(null)
      .mockResolvedValueOnce({ resources: [] })

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      expect(screen.getByText('Resource not found: FluxInstance/flux-system/flux')).toBeInTheDocument()
    })
  })

  it('should render resource header and panels on success', async () => {
    // Mock both fetch calls: resource and overview
    fetchWithMock
      .mockResolvedValueOnce(mockResourceData) // resource detail
      .mockResolvedValueOnce({ resources: [mockOverviewData] }) // overview list

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    // Check Header
    await waitFor(() => {
      expect(screen.getByText('flux')).toBeInTheDocument()
    })
    // Note: The text in DOM is 'FluxInstance', CSS makes it uppercase
    expect(screen.getByText('FluxInstance')).toBeInTheDocument()
    expect(screen.getByText('flux-system namespace')).toBeInTheDocument()
    
    // Check Status Icon presence (Ready status)
    const iconContainer = screen.getByText('flux').closest('.card').querySelector('.bg-green-50')
    expect(iconContainer).toBeInTheDocument()

    // Check Child Panels using mocked components
    expect(screen.getByTestId('reconciler-panel')).toHaveTextContent('ReconcilerPanel: FluxInstance/flux-system/flux')
    expect(screen.getByTestId('inventory-panel')).toHaveTextContent('InventoryPanel: flux')
    expect(screen.getByTestId('source-panel')).toHaveTextContent('SourcePanel: flux-system')
  })

  it('should render correct status style for Failed status', async () => {
    const failedOverview = { ...mockOverviewData, status: 'Failed' }
    
    fetchWithMock
      .mockResolvedValueOnce(mockResourceData)
      .mockResolvedValueOnce({ resources: [failedOverview] })

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      // Check for red background class associated with Failed status
      const headerCard = screen.getByText('flux').closest('.card')
      expect(headerCard).toHaveClass('bg-red-50')
      expect(headerCard).toHaveClass('border-danger')
    })
  })

  it('should render correct status style for Progressing status', async () => {
    const progressingOverview = { ...mockOverviewData, status: 'Progressing' }
    
    fetchWithMock
      .mockResolvedValueOnce(mockResourceData)
      .mockResolvedValueOnce({ resources: [progressingOverview] })

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      // Check for blue background class associated with Progressing status
      const headerCard = screen.getByText('flux').closest('.card')
      expect(headerCard).toHaveClass('bg-blue-50')
      expect(headerCard).toHaveClass('border-blue-500')
    })
  })

  it('should render correct status style for Suspended status', async () => {
    const suspendedOverview = { ...mockOverviewData, status: 'Suspended' }
    
    fetchWithMock
      .mockResolvedValueOnce(mockResourceData)
      .mockResolvedValueOnce({ resources: [suspendedOverview] })

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

    fetchWithMock
      .mockResolvedValueOnce(dataNoSource)
      .mockResolvedValueOnce({ resources: [mockOverviewData] })

    render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

    await waitFor(() => {
      expect(screen.getByTestId('reconciler-panel')).toBeInTheDocument()
    })
    
    expect(screen.queryByTestId('source-panel')).not.toBeInTheDocument()
  })

  it('should handle missing overview data gracefully', async () => {
    // Mock resource data but empty overview response
    fetchWithMock
      .mockResolvedValueOnce(mockResourceData)
      .mockResolvedValueOnce({ resources: [] }) // No overview found

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
      fetchWithMock.mockResolvedValue({ resources: [mockOverviewData] })

      render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      // Initial fetch should happen
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2) // resource + overview
      })

      // Clear mock call history
      fetchWithMock.mockClear()

      // Mock stays the same for auto-refresh
      fetchWithMock.mockResolvedValue(mockResourceData)
      fetchWithMock.mockResolvedValue({ resources: [mockOverviewData] })

      // Fast-forward 30 seconds
      vi.advanceTimersByTime(30000)

      // Auto-refresh should trigger
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2) // resource + overview
      })
    })

    it('should set lastUpdatedAt timestamp on successful fetch', async () => {
      const now = new Date('2023-01-01T12:30:00Z')
      vi.setSystemTime(now)

      fetchWithMock
        .mockResolvedValueOnce(mockResourceData)
        .mockResolvedValueOnce({ resources: [mockOverviewData] })

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
      fetchWithMock.mockResolvedValue({ resources: [mockOverviewData] })

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

    it('should only show loading spinner on initial load, not on auto-refresh', async () => {
      fetchWithMock.mockResolvedValue(mockResourceData)
      fetchWithMock.mockResolvedValue({ resources: [mockOverviewData] })

      render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      // Initial load should show spinner
      expect(screen.getByText('Loading resource...')).toBeInTheDocument()

      // Wait for initial load to complete
      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument()
      })

      // Fast-forward to trigger auto-refresh
      vi.advanceTimersByTime(30000)

      // Content should remain visible during refresh (no spinner)
      expect(screen.queryByText('Loading resource...')).not.toBeInTheDocument()
      expect(screen.getByText('flux')).toBeInTheDocument()
    })

    it('should clear interval on unmount', async () => {
      fetchWithMock.mockResolvedValue(mockResourceData)
      fetchWithMock.mockResolvedValue({ resources: [mockOverviewData] })

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
      fetchWithMock.mockResolvedValue({ resources: [mockOverviewData] })

      const { rerender } = render(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux" />)

      await waitFor(() => {
        expect(screen.getByText('flux')).toBeInTheDocument()
      })

      // Clear mock history
      fetchWithMock.mockClear()

      // Change route parameter (different resource)
      const newResourceData = { ...mockResourceData, metadata: { ...mockResourceData.metadata, name: 'flux-2' } }
      fetchWithMock.mockResolvedValue(newResourceData)
      fetchWithMock.mockResolvedValue({ resources: [mockOverviewData] })

      // Rerender with different name
      rerender(<ResourcePage kind="FluxInstance" namespace="flux-system" name="flux-2" />)

      // Should fetch immediately for new resource
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2) // resource + overview
      })
    })
  })
})
