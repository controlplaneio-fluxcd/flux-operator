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
import { reportData } from '../app'
import { fetchWithMock } from '../utils/fetch'

// Mock the fetch utility
vi.mock('../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock routing utilities
vi.mock('../utils/routing', () => ({
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

// Mock ResourceView component to simplify testing
vi.mock('./ResourceView', () => ({
  ResourceView: ({ kind, name, namespace, isExpanded }) => (
    isExpanded ? (
      <div data-testid="resource-view">
        <span data-testid="resource-view-kind">{kind}</span>
        <span data-testid="resource-view-name">{name}</span>
        <span data-testid="resource-view-namespace">{namespace}</span>
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

    it('should display resource cards when data loads', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      await waitFor(() => {
        expect(screen.getAllByText(/flux-system/)).toHaveLength(3) // namespace + name in both cards
      })
      expect(screen.getByText(/Stored artifact/)).toBeInTheDocument()
      expect(screen.getByText('apps')).toBeInTheDocument()
      expect(screen.getByText('Health check failed')).toBeInTheDocument()
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

      await waitFor(() => {
        expect(screen.getByText('2 resources')).toBeInTheDocument()
      })
    })

    it('should not display resource count when loading', async () => {
      resourcesData.value = []
      resourcesLoading.value = true

      render(<ResourceList />)

      expect(screen.queryByText(/resources$/)).not.toBeInTheDocument()
    })

    it('should sort resources by lastReconciled (newest first)', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      await waitFor(() => {
        const cards = screen.getAllByText(/GitRepository|Kustomization/i)
        // First card should be GitRepository (10:00:00), second should be Kustomization (09:00:00)
        expect(cards[0].textContent).toBe('GitRepository')
        expect(cards[1].textContent).toBe('Kustomization')
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

  describe('ResourceCard rendering', () => {
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

    it('should show expand button for long messages', async () => {
      const longMessage = 'a'.repeat(200)
      fetchWithMock.mockResolvedValue({ resources: [{
        ...mockResources[0],
        message: longMessage
      }] })

      render(<ResourceList />)

      await waitFor(() => {
        expect(screen.getByText('Show more')).toBeInTheDocument()
      })
    })

    it('should expand message when show more is clicked', async () => {
      const longMessage = 'a'.repeat(200)
      fetchWithMock.mockResolvedValue({ resources: [{
        ...mockResources[0],
        message: longMessage
      }] })

      render(<ResourceList />)

      const showMoreButton = await screen.findByText('Show more')
      fireEvent.click(showMoreButton)

      expect(screen.getByText('Show less')).toBeInTheDocument()
    })

    it('should display details toggle for all resources', async () => {
      fetchWithMock.mockResolvedValue({ resources: mockResources })

      render(<ResourceList />)

      await waitFor(() => {
        const detailsButtons = screen.getAllByText('Details')
        expect(detailsButtons).toHaveLength(2) // One for each resource
      })
    })

    it('should expand ResourceView when details toggle is clicked', async () => {
      fetchWithMock.mockResolvedValue({ resources: [mockResources[0]] })

      render(<ResourceList />)

      const detailsToggle = await screen.findByText('Details')
      fireEvent.click(detailsToggle)

      // Check that ResourceView is rendered with correct props
      await waitFor(() => {
        expect(screen.getByTestId('resource-view')).toBeInTheDocument()
      })
      expect(screen.getByTestId('resource-view-kind')).toHaveTextContent('GitRepository')
      expect(screen.getByTestId('resource-view-name')).toHaveTextContent('flux-system')
      expect(screen.getByTestId('resource-view-namespace')).toHaveTextContent('flux-system')
    })
  })

  describe('Message truncation', () => {
    it('should not truncate short single-line messages', async () => {
      const resourceWithShortMessage = {
        kind: 'GitRepository',
        name: 'repo',
        namespace: 'default',
        status: 'Ready',
        message: 'Short message under 150 characters',
        lastReconciled: new Date('2025-01-15T10:00:00Z'),
        inventory: []
      }

      fetchWithMock.mockResolvedValue({ resources: [resourceWithShortMessage] })

      render(<ResourceList />)

      await waitFor(() => {
        expect(screen.getByText('Short message under 150 characters')).toBeInTheDocument()
      })

      // Should not show "Show more" button for short messages
      expect(screen.queryByText(/Show more/)).not.toBeInTheDocument()
    })

    it('should show first line without truncation when it is short but message has multiple lines', async () => {
      const resourceWithMultilineMessage = {
        kind: 'GitRepository',
        name: 'repo',
        namespace: 'default',
        status: 'Failed',
        message: 'First line is short\nSecond line contains more details\nThird line with even more information',
        lastReconciled: new Date('2025-01-15T10:00:00Z'),
        inventory: []
      }

      fetchWithMock.mockResolvedValue({ resources: [resourceWithMultilineMessage] })

      render(<ResourceList />)

      // Should show truncated to first line
      await waitFor(() => {
        expect(screen.getByText(/First line is short/)).toBeInTheDocument()
      })

      // Should show "Show more" button for multiline messages
      expect(screen.getByText(/Show more/)).toBeInTheDocument()
    })

    it('should truncate first line when it exceeds 150 characters', async () => {
      const longFirstLine = 'This is a very long message that exceeds one hundred and fifty characters and should be truncated at exactly that length with ellipsis added to indicate continuation'

      const resourceWithLongFirstLine = {
        kind: 'Kustomization',
        name: 'apps',
        namespace: 'default',
        status: 'Failed',
        message: longFirstLine,
        lastReconciled: new Date('2025-01-15T10:00:00Z'),
        inventory: []
      }

      fetchWithMock.mockResolvedValue({ resources: [resourceWithLongFirstLine] })

      render(<ResourceList />)

      // Should show truncated message with ellipsis
      await waitFor(() => {
        const messageElement = screen.getByText(/This is a very long message/)
        expect(messageElement.textContent).toContain('...')
        expect(messageElement.textContent.length).toBeLessThan(longFirstLine.length)
      })
    })
  })
})
