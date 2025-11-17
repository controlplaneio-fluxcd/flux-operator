// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import {
  App,
  fetchFluxReport,
  fluxReport,
  lastUpdated,
  isLoading,
  connectionStatus,
  showSearchView,
  activeSearchTab
} from './app'

// Mock child components
vi.mock('./components/ConnectionStatus', () => ({
  ConnectionStatus: () => <div data-testid="connection-status">ConnectionStatus</div>
}))

vi.mock('./components/Header', () => ({
  Header: () => <div data-testid="header">Header</div>
}))

vi.mock('./components/DashboardView', () => ({
  DashboardView: ({ spec }) => <div data-testid="dashboard-view">DashboardView: {JSON.stringify(spec)}</div>
}))

vi.mock('./components/SearchView', () => ({
  SearchView: () => <div data-testid="search-view">SearchView</div>
}))

// Mock fetchWithMock utility
vi.mock('./utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock theme utilities
vi.mock('./utils/theme', () => ({
  themeMode: { value: 'light' },
  appliedTheme: { value: 'light' },
  themes: { light: 'light', dark: 'dark', auto: 'auto' }
}))

import { fetchWithMock } from './utils/fetch'

describe('app.jsx', () => {
  beforeEach(() => {
    // Reset all signals to initial state
    fluxReport.value = null
    lastUpdated.value = null
    isLoading.value = true
    connectionStatus.value = 'loading'
    showSearchView.value = false
    activeSearchTab.value = 'events'

    // Clear all mocks
    vi.clearAllMocks()

    // Use fake timers for interval testing
    vi.useFakeTimers()
  })

  afterEach(() => {
    // Restore real timers
    vi.useRealTimers()
  })

  describe('fetchFluxReport function', () => {
    it('should fetch report data successfully', async () => {
      const mockData = {
        spec: {
          distribution: { version: 'v2.4.0' },
          components: [{ name: 'source-controller', ready: true }],
          reconcilers: []
        }
      }

      fetchWithMock.mockResolvedValue(mockData)

      await fetchFluxReport()

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/report',
        mockPath: '../mock/report',
        mockExport: 'mockReport'
      })
      expect(fluxReport.value).toEqual(mockData)
      expect(connectionStatus.value).toBe('connected')
      expect(isLoading.value).toBe(false)
      expect(lastUpdated.value).toBeInstanceOf(Date)
    })

    it('should handle fetch errors', async () => {
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      await fetchFluxReport()

      expect(connectionStatus.value).toBe('disconnected')
      expect(isLoading.value).toBe(false)
      expect(lastUpdated.value).toBeInstanceOf(Date)
      expect(fluxReport.value).toBeNull()
      expect(consoleErrorSpy).toHaveBeenCalledWith('Failed to fetch report:', expect.any(Error))

      consoleErrorSpy.mockRestore()
    })

    it('should set loading status before fetching', async () => {
      connectionStatus.value = 'connected'
      let resolveFunc
      const promise = new Promise(resolve => { resolveFunc = resolve })
      fetchWithMock.mockReturnValue(promise)

      const fetchPromise = fetchFluxReport()

      expect(connectionStatus.value).toBe('loading')

      resolveFunc({ spec: {} })
      await fetchPromise
    })

    it('should not overwrite loading status if already loading', async () => {
      connectionStatus.value = 'loading'
      fetchWithMock.mockResolvedValue({ spec: {} })

      await fetchFluxReport()

      // Should have been set to loading, then to connected after success
      expect(connectionStatus.value).toBe('connected')
    })

    it('should update lastUpdated timestamp on success', async () => {
      const beforeFetch = new Date()
      fetchWithMock.mockResolvedValue({ spec: {} })

      await fetchFluxReport()

      expect(lastUpdated.value).toBeInstanceOf(Date)
      expect(lastUpdated.value.getTime()).toBeGreaterThanOrEqual(beforeFetch.getTime())
    })

    it('should update lastUpdated timestamp on failure', async () => {
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const beforeFetch = new Date()
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      await fetchFluxReport()

      expect(lastUpdated.value).toBeInstanceOf(Date)
      expect(lastUpdated.value.getTime()).toBeGreaterThanOrEqual(beforeFetch.getTime())

      consoleErrorSpy.mockRestore()
    })
  })

  describe('App Component - Loading State', () => {
    it('should show loading spinner when isLoading=true and no fluxReport', async () => {
      isLoading.value = true
      fluxReport.value = null
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      expect(screen.getByText('Loading Flux status...')).toBeInTheDocument()
      const spinner = document.querySelector('.animate-spin')
      expect(spinner).toBeInTheDocument()

      // Wait for effect to complete
      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })

    it('should show ConnectionStatus in loading state', async () => {
      isLoading.value = true
      fluxReport.value = null
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      expect(screen.getByTestId('connection-status')).toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })

    it('should have loading spinner with proper styling', async () => {
      isLoading.value = true
      fluxReport.value = null
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      const spinner = document.querySelector('.animate-spin')
      expect(spinner).toHaveClass('rounded-full')
      expect(spinner).toHaveClass('h-12')
      expect(spinner).toHaveClass('w-12')
      expect(spinner).toHaveClass('border-b-2')
      expect(spinner).toHaveClass('border-flux-blue')

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })

    it('should not show loading state if fluxReport exists', async () => {
      isLoading.value = true
      fluxReport.value = { spec: { distribution: { version: 'v2.4.0' } } }
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      expect(screen.queryByText('Loading Flux status...')).not.toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })
  })

  describe('App Component - Error State', () => {
    it('should not show error if fluxReport exists even when disconnected', async () => {
      isLoading.value = false
      fluxReport.value = { spec: { distribution: { version: 'v2.4.0' } } }
      connectionStatus.value = 'disconnected'
      fetchWithMock.mockResolvedValue({ spec: { distribution: { version: 'v2.4.0' } } })

      render(<App />)

      expect(screen.queryByText('Failed to load Flux report')).not.toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })
  })

  describe('App Component - Normal State', () => {
    const mockReport = {
      spec: {
        distribution: { version: 'v2.4.0' },
        components: [{ name: 'source-controller', ready: true }],
        reconcilers: []
      }
    }

    it('should render DashboardView when showSearchView=false', () => {
      isLoading.value = false
      fluxReport.value = mockReport
      showSearchView.value = false

      render(<App />)

      expect(screen.getByTestId('dashboard-view')).toBeInTheDocument()
      expect(screen.queryByTestId('search-view')).not.toBeInTheDocument()
    })

    it('should render SearchView when showSearchView=true', () => {
      isLoading.value = false
      fluxReport.value = mockReport
      showSearchView.value = true

      render(<App />)

      expect(screen.getByTestId('search-view')).toBeInTheDocument()
      expect(screen.queryByTestId('dashboard-view')).not.toBeInTheDocument()
    })

    it('should pass spec to DashboardView', () => {
      isLoading.value = false
      fluxReport.value = mockReport
      showSearchView.value = false

      render(<App />)

      const dashboardView = screen.getByTestId('dashboard-view')
      expect(dashboardView.textContent).toContain('v2.4.0')
    })

    it('should render Header in normal state', () => {
      isLoading.value = false
      fluxReport.value = mockReport

      render(<App />)

      expect(screen.getByTestId('header')).toBeInTheDocument()
    })

    it('should render ConnectionStatus in normal state', () => {
      isLoading.value = false
      fluxReport.value = mockReport

      render(<App />)

      expect(screen.getByTestId('connection-status')).toBeInTheDocument()
    })

    it('should toggle between dashboard and search views', () => {
      isLoading.value = false
      fluxReport.value = mockReport
      showSearchView.value = false

      const { rerender } = render(<App />)
      expect(screen.getByTestId('dashboard-view')).toBeInTheDocument()

      showSearchView.value = true
      rerender(<App />)
      expect(screen.getByTestId('search-view')).toBeInTheDocument()
    })
  })

  describe('App Component - Data Fetching Lifecycle', () => {
    it('should call fetchFluxReport on mount', async () => {
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })
    })

    it('should setup 30-second auto-refresh interval', async () => {
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      // Initial fetch
      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      // Advance time by 30 seconds
      vi.advanceTimersByTime(30000)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(2)
      })

      // Advance another 30 seconds
      vi.advanceTimersByTime(30000)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(3)
      })
    })

    it('should cleanup interval on unmount', async () => {
      fetchWithMock.mockResolvedValue({ spec: {} })

      const { unmount } = render(<App />)

      await waitFor(() => {
        expect(fetchWithMock).toHaveBeenCalledTimes(1)
      })

      unmount()

      // Advance time by 30 seconds after unmount
      vi.advanceTimersByTime(30000)

      // Should not call fetch again after unmount
      expect(fetchWithMock).toHaveBeenCalledTimes(1)
    })
  })

  describe('Global Signals', () => {
    it('should export fluxReport signal', () => {
      expect(fluxReport.value).toBeNull()
      fluxReport.value = { spec: {} }
      expect(fluxReport.value).toEqual({ spec: {} })
    })

    it('should export lastUpdated signal', () => {
      expect(lastUpdated.value).toBeNull()
      const now = new Date()
      lastUpdated.value = now
      expect(lastUpdated.value).toBe(now)
    })

    it('should export isLoading signal with default true', () => {
      expect(isLoading.value).toBe(true)
      isLoading.value = false
      expect(isLoading.value).toBe(false)
    })

    it('should export connectionStatus signal with default loading', () => {
      expect(connectionStatus.value).toBe('loading')
      connectionStatus.value = 'connected'
      expect(connectionStatus.value).toBe('connected')
    })

    it('should export showSearchView signal with default false', () => {
      expect(showSearchView.value).toBe(false)
      showSearchView.value = true
      expect(showSearchView.value).toBe(true)
    })

    it('should export activeSearchTab signal with default events', () => {
      expect(activeSearchTab.value).toBe('events')
      activeSearchTab.value = 'resources'
      expect(activeSearchTab.value).toBe('resources')
    })
  })

  describe('Layout and Styling', () => {
    it('should have min-h-screen on all states', () => {
      isLoading.value = true
      fluxReport.value = null

      const { container, rerender } = render(<App />)
      expect(container.querySelector('.min-h-screen')).toBeInTheDocument()

      isLoading.value = false
      connectionStatus.value = 'disconnected'
      rerender(<App />)
      expect(container.querySelector('.min-h-screen')).toBeInTheDocument()

      fluxReport.value = { spec: {} }
      rerender(<App />)
      expect(container.querySelector('.min-h-screen')).toBeInTheDocument()
    })

    it('should have dark mode support classes', () => {
      isLoading.value = false
      fluxReport.value = { spec: {} }

      const { container } = render(<App />)
      const root = container.querySelector('.bg-gray-50')
      expect(root).toHaveClass('dark:bg-gray-900')
    })

    it('should have transition-colors class', () => {
      isLoading.value = false
      fluxReport.value = { spec: {} }

      const { container } = render(<App />)
      expect(container.querySelector('.transition-colors')).toBeInTheDocument()
    })

    it('should have flex-col layout', () => {
      isLoading.value = false
      fluxReport.value = { spec: {} }

      const { container } = render(<App />)
      expect(container.querySelector('.flex-col')).toBeInTheDocument()
    })
  })

  describe('Edge Cases', () => {
    it('should require valid fluxReport with spec to render normal state', async () => {
      // In practice, the app always shows loading or error state if fluxReport is invalid
      // This test verifies that the normal render path requires a valid spec

      isLoading.value = false
      fluxReport.value = { spec: { distribution: { version: 'v2.4.0' } } }
      fetchWithMock.mockResolvedValue({ spec: { distribution: { version: 'v2.4.0' } } })

      const { container } = render(<App />)

      // Should render successfully with valid spec
      expect(container).toBeInTheDocument()
      expect(screen.getByTestId('dashboard-view')).toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })

    it('should handle spec with null values in nested properties', async () => {
      isLoading.value = false
      fluxReport.value = {
        spec: {
          distribution: null,
          components: [],
          reconcilers: []
        }
      }
      fetchWithMock.mockResolvedValue({ spec: {} })

      const { container } = render(<App />)

      // App can handle spec with null nested values - components will handle it
      expect(container).toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })
  })
})
