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

// Mock routing utilities (stub the URL-sync hooks, keep the real getDashboardUrl)
vi.mock('../../utils/routing', async (importActual) => ({
  ...(await importActual()),
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

    it('should display event rows when data loads', async () => {
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      render(<EventList />)

      // Each row renders the involved-object link twice (mobile + desktop layouts),
      // so the name appears in two links.
      await waitFor(() => {
        expect(screen.getAllByRole('link', { name: 'flux-system/flux-system' })).toHaveLength(2)
      })
      expect(screen.getAllByText('apps')).toHaveLength(2)
      // The message shows in the desktop one-line summary and again in the inline reveal.
      expect(screen.getAllByText(/Fetched revision/).length).toBeGreaterThan(0)
      expect(screen.getAllByText('Health check failed').length).toBeGreaterThan(0)
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

      // The count renders once, in the FilterBar toolbar. (The desktop section
      // count lives in the App tab nav, not this component.)
      await waitFor(() => {
        expect(screen.getAllByText('2 events')).toHaveLength(1)
      })
    })

    it('should show a loading indicator instead of the count while loading', async () => {
      eventsData.value = mockEvents
      eventsLoading.value = true

      render(<EventList />)

      // While loading the FilterBar shows "Loading…" rather than a (stale) count.
      expect(screen.getByText('Loading…')).toBeInTheDocument()
      expect(screen.queryByText('2 events')).not.toBeInTheDocument()
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

  describe('EventRow rendering', () => {
    it('should render a severity-colored kind chip for Normal events', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[0]] })

      render(<EventList />)

      // GitRepository shortens to "gitrepo"; the chip is rendered for both the
      // desktop top row and the mobile second line.
      await waitFor(() => {
        expect(screen.getAllByText('gitrepo').length).toBeGreaterThan(0)
      })
      screen.getAllByText('gitrepo').forEach((chip) => {
        expect(chip.className).toContain('bg-green-100')
      })
    })

    it('should render a severity-colored kind chip for Warning events', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[1]] })

      render(<EventList />)

      // Kustomization shortens to "ks"; Warning events get the red chip palette.
      await waitFor(() => {
        expect(screen.getAllByText('ks').length).toBeGreaterThan(0)
      })
      screen.getAllByText('ks').forEach((chip) => {
        expect(chip.className).toContain('bg-red-100')
      })
    })

    it('should display the severity word "Info" for Normal events', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[0]] })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getByText('Info')).toBeInTheDocument()
      })
    })

    it('should display the severity word "Warning" for Warning events', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[1]] })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getByText('Warning')).toBeInTheDocument()
      })
    })
  })

  describe('Inline message reveal', () => {
    it('should render a details toggle for each event', async () => {
      fetchWithMock.mockResolvedValue({ events: mockEvents })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getAllByLabelText('Toggle event details')).toHaveLength(2)
      })
    })

    it('should reveal the full message inline when expanded, with no spinner or fetch', async () => {
      const fullMessage = 'Line one of the event\nLine two with more detail'
      fetchWithMock.mockResolvedValue({ events: [{ ...mockEvents[0], message: fullMessage }] })

      const { container } = render(<EventList />)

      const toggle = await screen.findByLabelText('Toggle event details')

      // Collapsed before expanding, and only the mount fetch has run.
      const reveal = container.querySelector('[class*="grid-rows-"]')
      expect(reveal.className).toContain('grid-rows-[0fr]')
      expect(fetchWithMock).toHaveBeenCalledTimes(1)

      fireEvent.click(toggle)

      // Expanded: the animated reveal opens and shows the full message verbatim.
      await waitFor(() => {
        expect(container.querySelector('[class*="grid-rows-"]').className).toContain('grid-rows-[1fr]')
      })
      const pre = container.querySelector('pre')
      expect(pre).toBeInTheDocument()
      expect(pre.textContent).toBe(fullMessage)

      // Events carry their message in the list, so expanding triggers no extra
      // fetch and shows no loading spinner.
      expect(fetchWithMock).toHaveBeenCalledTimes(1)
      expect(container.querySelector('.animate-spin')).not.toBeInTheDocument()
    })

    it('should collapse the reveal again when the toggle is clicked twice', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[0]] })

      const { container } = render(<EventList />)

      const toggle = await screen.findByLabelText('Toggle event details')

      fireEvent.click(toggle)
      await waitFor(() => {
        expect(container.querySelector('[class*="grid-rows-"]').className).toContain('grid-rows-[1fr]')
      })

      fireEvent.click(toggle)
      await waitFor(() => {
        expect(container.querySelector('[class*="grid-rows-"]').className).toContain('grid-rows-[0fr]')
      })
    })
  })

  describe('Navigation to resource dashboard', () => {
    it('should link the involved object to its dashboard', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[0]] })

      render(<EventList />)

      // The name link is rendered twice (mobile + desktop); both point at the
      // involved object's dashboard.
      await waitFor(() => {
        expect(screen.getAllByRole('link', { name: 'flux-system/flux-system' })).toHaveLength(2)
      })
      screen.getAllByRole('link', { name: 'flux-system/flux-system' }).forEach((link) => {
        expect(link).toHaveAttribute('href', '/resource/GitRepository/flux-system/flux-system')
      })
    })

    it('should have correct href for different resource', async () => {
      fetchWithMock.mockResolvedValue({ events: [mockEvents[1]] })

      render(<EventList />)

      await waitFor(() => {
        expect(screen.getAllByRole('link', { name: 'flux-system/apps' })).toHaveLength(2)
      })
      screen.getAllByRole('link', { name: 'flux-system/apps' }).forEach((link) => {
        expect(link).toHaveAttribute('href', '/resource/Kustomization/flux-system/apps')
      })
    })
  })
})
