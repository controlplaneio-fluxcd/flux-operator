// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/preact'
import {
  ResourceList,
  resourcesData,
  resourcesLoading,
  resourcesError,
  selectedResourceKind,
  selectedResourceName,
  selectedResourceNamespace,
  selectedResourceStatus,
  fetchResourcesStatus
} from './ResourceList'
import { reportData } from '../../app'
import { fetchWithMock } from '../../utils/fetch'
import { favorites } from '../../utils/favorites'

// Mock the fetch utility
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock routing utilities (stub the URL-sync hooks, keep the real getDashboardUrl)
vi.mock('../../utils/routing', async (importActual) => ({
  ...(await importActual()),
  useRestoreFiltersFromUrl: vi.fn(),
  useSyncFiltersToUrl: vi.fn()
}))

// Mock preact-iso
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    path: '/resources',
    query: {},
    route: vi.fn()
  })
}))

// Mock FilterForm component to simplify testing
vi.mock('./FilterForm', () => ({
  FilterForm: ({ onClear, kindSignal, nameSignal, namespaceSignal, statusSignal }) => (
    <div data-testid="filter-form">
      <button onClick={onClear} data-testid="clear-filters">Clear</button>
      <span data-testid="kind-signal">{kindSignal.value}</span>
      <span data-testid="name-signal">{nameSignal.value}</span>
      <span data-testid="namespace-signal">{namespaceSignal.value}</span>
      <span data-testid="status-signal">{statusSignal.value}</span>
    </div>
  )
}))

// Mock ResourceDetailsView component to simplify testing.
// Exposes a "details-ready" button so tests can drive the disclosure's `onReady`
// callback (which the real panel fires once its data has loaded), letting us
// assert the spinner -> reveal transition deterministically.
vi.mock('./ResourceDetailsView', () => ({
  ResourceDetailsView: ({ kind, name, namespace, isExpanded, onReady, onData }) => (
    isExpanded ? (
      <div data-testid="resource-details-view">
        <span data-testid="resource-details-view-kind">{kind}</span>
        <span data-testid="resource-details-view-name">{name}</span>
        <span data-testid="resource-details-view-namespace">{namespace}</span>
        <button data-testid="details-ready" onClick={onReady}>ready</button>
        {/* Simulates the detail fetch landing with a fresher server-computed
            reconcilerRef summary, used to test the row write-back. */}
        <button
          data-testid="details-emit-data"
          onClick={() => onData && onData({ status: { reconcilerRef: { status: 'Ready', message: 'now reconciled', lastReconciled: '2025-11-18T11:10:59Z' } } })}
        >emit</button>
      </div>
    ) : null
  )
}))

describe('ResourceList', () => {
  const mockResources = [
    {
      kind: 'GitRepository',
      name: 'flux-system',
      namespace: 'flux-system',
      status: 'Ready',
      message: 'Stored artifact for revision: main@sha1:abc123',
      lastReconciled: new Date('2025-01-15T10:00:00Z'),
      inventory: [
        { apiVersion: 'v1', kind: 'Namespace', name: 'flux-system' },
        { apiVersion: 'apps/v1', kind: 'Deployment', namespace: 'flux-system', name: 'source-controller' }
      ]
    },
    {
      kind: 'Kustomization',
      name: 'apps',
      namespace: 'flux-system',
      status: 'Failed',
      message: 'Health check failed',
      lastReconciled: new Date('2025-01-15T09:00:00Z'),
      inventory: []
    }
  ]

  beforeEach(() => {
    // Reset all signals
    resourcesData.value = []
    resourcesLoading.value = false
    resourcesError.value = null
    selectedResourceKind.value = ''
    selectedResourceName.value = ''
    selectedResourceNamespace.value = ''
    selectedResourceStatus.value = ''

    // Reset reportData
    reportData.value = {
      spec: {
        namespaces: ['flux-system', 'default']
      }
    }

    // Reset favorites (persisted in localStorage across tests in this file)
    favorites.value = []

    // Reset mocks
    vi.clearAllMocks()
    fetchWithMock.mockResolvedValue({ resources: [] })
  })

  describe('fetchResourcesStatus function', () => {
    it('should fetch resources with no filters', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      await fetchResourcesStatus()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resources?',
        mockPath: '../mock/resources',
        mockExport: 'getMockResources'
      })
      expect(resourcesData.value).toEqual(mockResources)
      expect(resourcesLoading.value).toBe(false)
    })

    it('should pass correct query params for kind filter', async () => {
      selectedResourceKind.value = 'GitRepository'
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      await fetchResourcesStatus()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resources?kind=GitRepository',
        mockPath: '../mock/resources',
        mockExport: 'getMockResources'
      })
    })

    it('should pass correct query params for name filter', async () => {
      selectedResourceName.value = 'flux-system'
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      await fetchResourcesStatus()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resources?name=flux-system',
        mockPath: '../mock/resources',
        mockExport: 'getMockResources'
      })
    })

    it('should pass correct query params for namespace filter', async () => {
      selectedResourceNamespace.value = 'flux-system'
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      await fetchResourcesStatus()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resources?namespace=flux-system',
        mockPath: '../mock/resources',
        mockExport: 'getMockResources'
      })
    })

    it('should pass correct query params for status filter', async () => {
      selectedResourceStatus.value = 'Ready'
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      await fetchResourcesStatus()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resources?status=Ready',
        mockPath: '../mock/resources',
        mockExport: 'getMockResources'
      })
    })

    it('should pass multiple query params when multiple filters set', async () => {
      selectedResourceKind.value = 'GitRepository'
      selectedResourceName.value = 'flux-system'
      selectedResourceNamespace.value = 'flux-system'
      selectedResourceStatus.value = 'Ready'
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      await fetchResourcesStatus()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/resources?kind=GitRepository&name=flux-system&namespace=flux-system&status=Ready',
        mockPath: '../mock/resources',
        mockExport: 'getMockResources'
      })
    })

    it('should set loading state during fetch', async () => {
      let resolvePromise
      const promise = new Promise((resolve) => { resolvePromise = resolve })
      fetchWithMock.mockReturnValue(promise)

      const fetchPromise = fetchResourcesStatus()

      expect(resourcesLoading.value).toBe(true)

      resolvePromise({ resources: mockResources })
      await fetchPromise

      expect(resourcesLoading.value).toBe(false)
    })

    it('should handle fetch errors', async () => {
      const error = new Error('Network error')
      fetchWithMock.mockRejectedValue(error)

      await fetchResourcesStatus()

      expect(resourcesError.value).toBe('Network error')
      expect(resourcesData.value).toEqual([])
      expect(resourcesLoading.value).toBe(false)
    })
  })

  describe('Component rendering', () => {
    it('should render loading shimmer on mount', async () => {
      resourcesLoading.value = true

      const { container } = render(<ResourceList />)

      // Should show loading shimmer in timeline chart
      const loadingShimmer = container.querySelector('.loading-shimmer')
      expect(loadingShimmer).toBeInTheDocument()
    })

    it('should fetch resources on component mount', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalled()
      })
    })

    it('should display resource rows when data loads', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      // Each row renders the namespace/name link twice (mobile + desktop variants).
      await waitFor(() => {
        expect(screen.getAllByText(/flux-system/).length).toBeGreaterThan(0)
      })
      // The status message is desktop-only, so it appears exactly once per row.
      expect(screen.getByText(/Stored artifact/)).toBeInTheDocument()
      expect(screen.getByText('Health check failed')).toBeInTheDocument()
      // The Kustomization name is rendered in both the mobile and desktop links.
      expect(screen.getAllByText('apps').length).toBeGreaterThan(0)
    })

    it('should show empty state when no resources match filters', async () => {
      fetchWithMock.mockResolvedValue({ resources: [] })

      render(<ResourceList />)

      await waitFor(() => {
        expect(screen.getByText('No resources found for the selected filters')).toBeInTheDocument()
      })
    })

    it('should show error state on fetch failure', async () => {
      fetchWithMock.mockRejectedValue(new Error('Failed to connect to server'))

      render(<ResourceList />)

      await waitFor(() => {
        expect(screen.getByText(/Failed to load resources: Failed to connect to server/)).toBeInTheDocument()
      })
    })

    it('should display resource count when resources are loaded', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      // The count renders once, in the FilterBar toolbar. (The desktop section
      // count lives in the App tab nav, not this component.)
      await waitFor(() => {
        expect(screen.getAllByText('2 resources')).toHaveLength(1)
      })
    })

    it('should show a loading indicator instead of the count while loading', async () => {
      resourcesData.value = mockResources
      resourcesLoading.value = true

      render(<ResourceList />)

      // While loading the FilterBar shows "Loading…" rather than a (stale) count.
      expect(screen.getByText('Loading…')).toBeInTheDocument()
      expect(screen.queryByText('2 resources')).not.toBeInTheDocument()
    })

    it('should sort resources by lastReconciled (newest first)', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      // Rows render their dashboard links in list order; the GitRepository
      // (10:00:00) must come before the Kustomization (09:00:00).
      await waitFor(() => {
        const hrefs = screen.getAllByRole('link').map((a) => a.getAttribute('href'))
        const gitIdx = hrefs.findIndex((h) => h.includes('GitRepository'))
        const ksIdx = hrefs.findIndex((h) => h.includes('Kustomization'))
        expect(gitIdx).toBeGreaterThanOrEqual(0)
        expect(gitIdx).toBeLessThan(ksIdx)
      })
    })
  })

  describe('Filter interactions', () => {
    it('should re-fetch when kind filter changes', async () => {
      fetchWithMock.mockResolvedValue({ resources: [] })

      render(<ResourceList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      selectedResourceKind.value = 'GitRepository'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should re-fetch when name filter changes', async () => {
      fetchWithMock.mockResolvedValue({ resources: [] })

      render(<ResourceList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      selectedResourceName.value = 'test-name'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should re-fetch when namespace filter changes', async () => {
      fetchWithMock.mockResolvedValue({ resources: [] })

      render(<ResourceList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      selectedResourceNamespace.value = 'default'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should re-fetch when status filter changes', async () => {
      fetchWithMock.mockResolvedValue({ resources: [] })

      render(<ResourceList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      selectedResourceStatus.value = 'Ready'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should clear all filters when clear button clicked', async () => {
      selectedResourceKind.value = 'GitRepository'
      selectedResourceName.value = 'test'
      selectedResourceNamespace.value = 'default'
      selectedResourceStatus.value = 'Ready'

      render(<ResourceList />)

      const clearButton = screen.getByTestId('clear-filters')
      fireEvent.click(clearButton)

      expect(selectedResourceKind.value).toBe('')
      expect(selectedResourceName.value).toBe('')
      expect(selectedResourceNamespace.value).toBe('')
      expect(selectedResourceStatus.value).toBe('')
    })
  })

  describe('Compact row rendering', () => {
    it('should display status badges correctly', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      await waitFor(() => {
        expect(screen.getByText('Ready')).toBeInTheDocument()
      })
      expect(screen.getByText('Failed')).toBeInTheDocument()
    })

    it('should display all status badge types with correct styling', async () => {
      const allStatusResources = [
        { kind: 'Kustomization', name: 'ready', namespace: 'flux-system', status: 'Ready', message: 'Ready', lastReconciled: new Date() },
        { kind: 'Kustomization', name: 'failed', namespace: 'flux-system', status: 'Failed', message: 'Failed', lastReconciled: new Date() },
        { kind: 'Kustomization', name: 'progressing', namespace: 'flux-system', status: 'Progressing', message: 'Progressing', lastReconciled: new Date() },
        { kind: 'Kustomization', name: 'suspended', namespace: 'flux-system', status: 'Suspended', message: 'Suspended', lastReconciled: new Date() },
        { kind: 'Kustomization', name: 'unknown', namespace: 'flux-system', status: 'Unknown', message: 'Unknown', lastReconciled: new Date() }
      ]

      fetchWithMock.mockResolvedValue({ resources: allStatusResources })

      render(<ResourceList />)

      await waitFor(() => {
        // Each status appears multiple times (as badge and in message)
        expect(screen.getAllByText('Ready').length).toBeGreaterThan(0)
        expect(screen.getAllByText('Failed').length).toBeGreaterThan(0)
        expect(screen.getAllByText('Progressing').length).toBeGreaterThan(0)
        expect(screen.getAllByText('Suspended').length).toBeGreaterThan(0)
        expect(screen.getAllByText('Unknown').length).toBeGreaterThan(0)
      })
    })

    it('should display a details toggle for every resource', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      // The expand affordance is now an icon button labelled "Toggle details".
      await waitFor(() => {
        expect(screen.getAllByLabelText('Toggle details')).toHaveLength(2)
      })
    })

    it('should expose the namespace/name dashboard link and favorite star', async () => {
      fetchWithMock.mockResolvedValue({ resources: [mockResources[0]] })

      render(<ResourceList />)

      // The row links the namespace/name to the resource dashboard (mobile +
      // desktop variants share the same href).
      await waitFor(() => {
        const hrefs = screen.getAllByRole('link').map((a) => a.getAttribute('href'))
        expect(hrefs).toContain('/resource/GitRepository/flux-system/flux-system')
      })

      // The favorite star toggles the resource in/out of favorites.
      const star = screen.getByTitle('Add to favorites')
      fireEvent.click(star)
      expect(screen.getByTitle('Remove from favorites')).toBeInTheDocument()
    })
  })

  describe('Row disclosure', () => {
    it('should spin, mount ResourceDetailsView, then reveal it on expand', async () => {
      fetchWithMock.mockResolvedValue({ resources: [mockResources[0]] })

      render(<ResourceList />)

      const toggle = await screen.findByLabelText('Toggle details')

      // Collapsed: the panel is not mounted and the button shows no spinner.
      expect(screen.queryByTestId('resource-details-view')).not.toBeInTheDocument()
      expect(toggle.querySelector('.animate-spin')).not.toBeInTheDocument()

      // Expand: the panel mounts (still collapsed) and the button spins while it
      // "loads".
      fireEvent.click(toggle)
      expect(screen.getByTestId('resource-details-view')).toBeInTheDocument()
      expect(toggle.querySelector('.animate-spin')).toBeInTheDocument()

      // The panel signals readiness via onReady -> the spinner is replaced by the
      // open chevron and the row is revealed.
      fireEvent.click(screen.getByTestId('details-ready'))
      await waitFor(() => {
        expect(toggle.querySelector('.animate-spin')).not.toBeInTheDocument()
      })
      expect(toggle.querySelector('.rotate-90')).toBeInTheDocument()
    })

    it('should mount ResourceDetailsView with the resource identity', async () => {
      fetchWithMock.mockResolvedValue({ resources: [mockResources[0]] })

      render(<ResourceList />)

      const toggle = await screen.findByLabelText('Toggle details')
      fireEvent.click(toggle)

      await waitFor(() => {
        expect(screen.getByTestId('resource-details-view')).toBeInTheDocument()
      })
      expect(screen.getByTestId('resource-details-view-kind')).toHaveTextContent('GitRepository')
      expect(screen.getByTestId('resource-details-view-name')).toHaveTextContent('flux-system')
      expect(screen.getByTestId('resource-details-view-namespace')).toHaveTextContent('flux-system')
    })

    it('refreshes the row summary from the detail data (stale Failed -> Ready)', async () => {
      const stale = {
        kind: 'Kustomization', namespace: 'flux-system', name: 'apps',
        status: 'Failed', message: 'reconciliation failed', lastReconciled: '2025-11-01T00:00:00Z'
      }
      fetchWithMock.mockResolvedValue({ resources: [stale] })

      render(<ResourceList />)

      const toggle = await screen.findByLabelText('Toggle details')
      // The row first shows the stale list summary.
      expect(screen.getByText('reconciliation failed')).toBeInTheDocument()

      fireEvent.click(toggle)
      // The panel lands with a fresher reconcilerRef summary -> the row updates.
      fireEvent.click(screen.getByTestId('details-emit-data'))

      await waitFor(() => {
        expect(screen.getByText('now reconciled')).toBeInTheDocument()
      })
      expect(screen.queryByText('reconciliation failed')).not.toBeInTheDocument()
    })
  })

  describe('FilterBar', () => {
    it('should show the result count header', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      // The FilterBar always renders the count, even on mobile where the desktop
      // page header is hidden.
      await waitFor(() => {
        expect(screen.getAllByText('2 resources').length).toBeGreaterThan(0)
      })
    })

    it('should toggle the mobile filters form', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      const filtersToggle = await screen.findByLabelText('Toggle filters')
      expect(filtersToggle).toBeInTheDocument()

      // The FilterForm is always present in the DOM (CSS controls mobile
      // visibility); pressing the toggle is a no-throw interaction.
      fireEvent.click(filtersToggle)
      expect(screen.getByTestId('filter-form')).toBeInTheDocument()
    })
  })
})
