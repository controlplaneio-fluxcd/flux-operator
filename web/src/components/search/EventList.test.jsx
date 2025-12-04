// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/preact'
import {
  EventList,
  eventsData,
  eventsLoading,
  eventsError,
  selectedEventKind,
  selectedEventName,
  selectedEventNamespace,
  selectedEventSeverity,
  fetchEvents
} from './EventList'
import { reportData } from '../../app'
import { fetchWithMock } from '../../utils/fetch'

// Mock the fetch utility
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock routing utilities
vi.mock('../../utils/routing', () => ({
  useRestoreFiltersFromUrl: vi.fn(),
  useSyncFiltersToUrl: vi.fn()
}))

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    path: '/events',
    query: {},
    route: mockRoute
  })
}))

// Mock FilterForm component to simplify testing
vi.mock('./FilterForm', () => ({
  FilterForm: ({ onClear, kindSignal, nameSignal, namespaceSignal, severitySignal }) => (
    <div data-testid="filter-form">
      <button onClick={onClear} data-testid="clear-filters">Clear</button>
      <span data-testid="kind-signal">{kindSignal.value}</span>
      <span data-testid="name-signal">{nameSignal.value}</span>
      <span data-testid="namespace-signal">{namespaceSignal.value}</span>
      <span data-testid="severity-signal">{severitySignal.value}</span>
    </div>
  )
}))

describe('EventList', () => {
  const mockEvents = [
    {
      involvedObject: 'GitRepository/flux-system',
      type: 'Normal',
      message: 'Fetched revision: main@sha1:abc123',
      namespace: 'flux-system',
      lastTimestamp: new Date('2025-01-15T10:00:00Z')
    },
    {
      involvedObject: 'Kustomization/apps',
      type: 'Warning',
      message: 'Health check failed',
      namespace: 'flux-system',
      lastTimestamp: new Date('2025-01-15T09:00:00Z')
    }
  ]

  beforeEach(() => {
    // Reset all signals
    eventsData.value = []
    eventsLoading.value = false
    eventsError.value = null
    selectedEventKind.value = ''
    selectedEventName.value = ''
    selectedEventNamespace.value = ''
    selectedEventSeverity.value = ''

    // Reset reportData
    reportData.value = {
      spec: {
        namespaces: ['flux-system', 'default']
      }
    }

    // Reset mocks
    vi.clearAllMocks()
    mockRoute.mockClear()
    fetchWithMock.mockResolvedValue({ events: [] })
  })

  describe('fetchEvents function', () => {
    it('should fetch events with no filters', async () => {
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
      expect(eventsData.value).toEqual(mockEvents)
      expect(eventsLoading.value).toBe(false)
    })

    it('should pass correct query params for kind filter', async () => {
      selectedEventKind.value = 'GitRepository'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?kind=GitRepository',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should pass correct query params for name filter', async () => {
      selectedEventName.value = 'flux-system'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?name=flux-system',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should pass correct query params for namespace filter', async () => {
      selectedEventNamespace.value = 'flux-system'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?namespace=flux-system',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should pass severity as "type" query param', async () => {
      selectedEventSeverity.value = 'Warning'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?type=Warning',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should pass multiple query params when multiple filters set', async () => {
      selectedEventKind.value = 'GitRepository'
      selectedEventName.value = 'flux-system'
      selectedEventNamespace.value = 'flux-system'
      selectedEventSeverity.value = 'Normal'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?kind=GitRepository&name=flux-system&namespace=flux-system&type=Normal',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should set loading state during fetch', async () => {
      let resolvePromise
      const promise = new Promise((resolve) => { resolvePromise = resolve })
      fetchWithMock.mockReturnValue(promise)

      const fetchPromise = fetchEvents()

      expect(eventsLoading.value).toBe(true)

      resolvePromise({ events: mockEvents })
      await fetchPromise

      expect(eventsLoading.value).toBe(false)
    })

    it('should handle fetch errors', async () => {
      const error = new Error('Network error')
      fetchWithMock.mockRejectedValue(error)

      await fetchEvents()

      expect(eventsError.value).toBe('Network error')
      expect(eventsData.value).toEqual([])
      expect(eventsLoading.value).toBe(false)
    })
  })

  describe('Component rendering', () => {
    it('should render loading shimmer on mount', async () => {
      eventsLoading.value = true

      const { container } = render(<EventList />)

      // Should show loading shimmer in timeline chart
      const loadingShimmer = container.querySelector('.loading-shimmer')
      expect(loadingShimmer).toBeInTheDocument()
    })

    it('should fetch events on component mount', async () => {
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      render(<EventList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalled()
      })
    })

    it('should display event cards when data loads', async () => {
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getAllByText(/flux-system/)).toHaveLength(3) // namespace + name in both cards
      })
      expect(screen.getByText(/Fetched revision/)).toBeInTheDocument()
      expect(screen.getByText('apps')).toBeInTheDocument()
      expect(screen.getByText('Health check failed')).toBeInTheDocument()
    })

    it('should show empty state when no events match filters', async () => {
      fetchWithMock.mockResolvedValue({ events: [] })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getByText('No events found for the selected filters')).toBeInTheDocument()
      })
    })

    it('should show error state on fetch failure', async () => {
      fetchWithMock.mockRejectedValue(new Error('Failed to connect to server'))

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getByText(/Failed to load events: Failed to connect to server/)).toBeInTheDocument()
      })
    })

    it('should display event count when events are loaded', async () => {
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getByText('2 events')).toBeInTheDocument()
      })
    })

    it('should not display event count when loading', async () => {
      eventsData.value = []
      eventsLoading.value = true

      render(<EventList />)

      expect(screen.queryByText(/events$/)).not.toBeInTheDocument()
    })
  })

  describe('Filter interactions', () => {
    it('should re-fetch when kind filter changes', async () => {
      fetchWithMock.mockResolvedValue({ events: [] })

      render(<EventList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      // Change filter
      selectedEventKind.value = 'GitRepository'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should re-fetch when name filter changes', async () => {
      fetchWithMock.mockResolvedValue({ events: [] })

      render(<EventList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      selectedEventName.value = 'test-name'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should re-fetch when namespace filter changes', async () => {
      fetchWithMock.mockResolvedValue({ events: [] })

      render(<EventList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      selectedEventNamespace.value = 'default'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should re-fetch when severity filter changes', async () => {
      fetchWithMock.mockResolvedValue({ events: [] })

      render(<EventList />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      selectedEventSeverity.value = 'Warning'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should clear all filters when clear button clicked', async () => {
      selectedEventKind.value = 'GitRepository'
      selectedEventName.value = 'test'
      selectedEventNamespace.value = 'default'
      selectedEventSeverity.value = 'Warning'

      render(<EventList />)

      const clearButton = screen.getByTestId('clear-filters')
      fireEvent.click(clearButton)

      expect(selectedEventKind.value).toBe('')
      expect(selectedEventName.value).toBe('')
      expect(selectedEventNamespace.value).toBe('')
      expect(selectedEventSeverity.value).toBe('')
    })
  })

  describe('EventCard rendering', () => {
    it('should display event type badge as "Info" for Normal events', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[0]] })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getByText('Info')).toBeInTheDocument()
      })
    })

    it('should display event type badge as "Warning" for Warning events', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[1]] })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getByText('Warning')).toBeInTheDocument()
      })
    })

    it('should show expand button for long messages', async () => {
      const longMessage = 'a'.repeat(200)
      fetchWithMock.mockResolvedValue({ events: [{
        ...mockEvents[0],
        message: longMessage
      }] })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getByText('Show more')).toBeInTheDocument()
      })
    })

    it('should expand message when show more is clicked', async () => {
      const longMessage = 'a'.repeat(200)
      fetchWithMock.mockResolvedValue({ events: [{
        ...mockEvents[0],
        message: longMessage
      }] })

      render(<EventList />)

      const showMoreButton = await screen.findByText('Show more')
      fireEvent.click(showMoreButton)

      expect(screen.getByText('Show less')).toBeInTheDocument()
    })
  })

  describe('Navigation to resource dashboard', () => {
    it('should navigate to resource dashboard when resource name is clicked', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[0]] })

      render(<EventList />)

      // Wait for events to load and find the resource name button
      const resourceButton = await screen.findByRole('button', { name: /flux-system\/flux-system/ })
      fireEvent.click(resourceButton)

      expect(mockRoute).toHaveBeenCalledWith('/resource/GitRepository/flux-system/flux-system')
    })

    it('should navigate with correct params for different resource', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[1]] })

      render(<EventList />)

      const resourceButton = await screen.findByRole('button', { name: /flux-system\/apps/ })
      fireEvent.click(resourceButton)

      expect(mockRoute).toHaveBeenCalledWith('/resource/Kustomization/flux-system/apps')
    })

    it('should display navigation icon in resource button', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[0]] })

      render(<EventList />)

      const resourceButton = await screen.findByRole('button', { name: /flux-system\/flux-system/ })
      const svg = resourceButton.querySelector('svg')

      expect(svg).toBeInTheDocument()
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24')
    })
  })
})
