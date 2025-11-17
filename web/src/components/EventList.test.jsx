// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/preact'
import {
  EventList,
  eventsData,
  eventsLoading,
  eventsError,
  selectedEventsKind,
  selectedEventsName,
  selectedEventsNamespace,
  selectedEventsSeverity,
  fetchEvents
} from './EventList'
import { fluxReport } from '../app'
import { fetchWithMock } from '../utils/fetch'

// Mock the fetch utility
vi.mock('../utils/fetch', () => ({
  fetchWithMock: vi.fn()
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
    selectedEventsKind.value = ''
    selectedEventsName.value = ''
    selectedEventsNamespace.value = ''
    selectedEventsSeverity.value = ''

    // Reset fluxReport
    fluxReport.value = {
      spec: {
        namespaces: ['flux-system', 'default']
      }
    }

    // Reset mocks
    vi.clearAllMocks()
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
      selectedEventsKind.value = 'GitRepository'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?kind=GitRepository',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should pass correct query params for name filter', async () => {
      selectedEventsName.value = 'flux-system'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?name=flux-system',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should pass correct query params for namespace filter', async () => {
      selectedEventsNamespace.value = 'flux-system'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?namespace=flux-system',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should pass severity as "type" query param', async () => {
      selectedEventsSeverity.value = 'Warning'
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      await fetchEvents()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/events?type=Warning',
        mockPath: '../mock/events',
        mockExport: 'getMockEvents'
      })
    })

    it('should pass multiple query params when multiple filters set', async () => {
      selectedEventsKind.value = 'GitRepository'
      selectedEventsName.value = 'flux-system'
      selectedEventsNamespace.value = 'flux-system'
      selectedEventsSeverity.value = 'Normal'
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
    it('should render loading spinner on mount', async () => {
      eventsLoading.value = true

      render(<EventList />)

      expect(screen.getByText('Loading events...')).toBeInTheDocument()
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
      selectedEventsKind.value = 'GitRepository'

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

      selectedEventsName.value = 'test-name'

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

      selectedEventsNamespace.value = 'default'

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

      selectedEventsSeverity.value = 'Warning'

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })
    })

    it('should clear all filters when clear button clicked', async () => {
      selectedEventsKind.value = 'GitRepository'
      selectedEventsName.value = 'test'
      selectedEventsNamespace.value = 'default'
      selectedEventsSeverity.value = 'Warning'

      render(<EventList />)

      const clearButton = screen.getByTestId('clear-filters')
      fireEvent.click(clearButton)

      expect(selectedEventsKind.value).toBe('')
      expect(selectedEventsName.value).toBe('')
      expect(selectedEventsNamespace.value).toBe('')
      expect(selectedEventsSeverity.value).toBe('')
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
})
