// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import {
  App,
  fetchFluxReport,
  reportData,
  reportUpdatedAt,
  reportLoading,
  reportError,
  connectionStatus
} from './app'

// Mock location state that can be modified in tests
let mockLocationPath = '/'
const mockRoute = vi.fn()

// Mock preact-iso
vi.mock('preact-iso', () => ({
  LocationProvider: ({ children }) => <div data-testid="location-provider">{children}</div>,
  Router: ({ children }) => <div data-testid="router">{children}</div>,
  Route: ({ component: Component, ...props }) => Component ? <Component {...props} /> : null,
  useLocation: () => ({
    path: mockLocationPath,
    query: {},
    route: mockRoute
  })
}))

// Mock child components
vi.mock('./components/layout/ConnectionStatus', () => ({
  ConnectionStatus: () => <div data-testid="connection-status">ConnectionStatus</div>
}))

vi.mock('./components/layout/Header', () => ({
  Header: () => <div data-testid="header">Header</div>
}))

vi.mock('./components/dashboards/cluster/ClusterPage', () => ({
  ClusterPage: ({ spec }) => <div data-testid="dashboard-view">ClusterPage: {JSON.stringify(spec)}</div>
}))

vi.mock('./components/search/EventList', () => ({
  EventList: () => <div data-testid="event-list">EventList</div>
}))

vi.mock('./components/search/ResourceList', () => ({
  ResourceList: () => <div data-testid="resource-list">ResourceList</div>
}))

vi.mock('./components/dashboards/resource/ResourcePage', () => ({
  ResourcePage: () => <div data-testid="resource-page">ResourcePage</div>
}))

// Mock fetchWithMock utility and authRequired signal
vi.mock('./utils/fetch', async () => {
  const { signal } = await import('@preact/signals')
  return {
    fetchWithMock: vi.fn(),
    authRequired: signal(false)
  }
})

// Mock cookies utility
vi.mock('./utils/cookies', () => ({
  parseAuthProviderCookie: vi.fn(() => null)
}))

// Mock LoginPage component
vi.mock('./components/auth/LoginPage', () => ({
  LoginPage: () => <div data-testid="login-page">LoginPage</div>
}))

// Mock theme utilities
vi.mock('./utils/theme', () => ({
  themeMode: { value: 'light' },
  appliedTheme: { value: 'light' },
  themes: { light: 'light', dark: 'dark', auto: 'auto' }
}))

import { fetchWithMock, authRequired } from './utils/fetch'
import { parseAuthProviderCookie } from './utils/cookies'

describe('app.jsx', () => {
  beforeEach(() => {
    // Reset all signals to initial state
    reportData.value = null
    reportUpdatedAt.value = null
    reportLoading.value = true
    reportError.value = null
    connectionStatus.value = 'loading'
    authRequired.value = false

    // Reset mock location path
    mockLocationPath = '/'

    // Reset mock returns
    parseAuthProviderCookie.mockReturnValue(null)

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
      expect(reportData.value).toEqual(mockData)
      expect(connectionStatus.value).toBe('connected')
      expect(reportLoading.value).toBe(false)
      expect(reportError.value).toBe(null)
      expect(reportUpdatedAt.value).toBeInstanceOf(Date)
    })

    it('should handle fetch errors', async () => {
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      await fetchFluxReport()

      expect(connectionStatus.value).toBe('disconnected')
      expect(reportLoading.value).toBe(false)
      expect(reportUpdatedAt.value).toBeInstanceOf(Date)
      expect(reportData.value).toBeNull()
      expect(reportError.value).toBe('Network error')
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

      expect(reportUpdatedAt.value).toBeInstanceOf(Date)
      expect(reportUpdatedAt.value.getTime()).toBeGreaterThanOrEqual(beforeFetch.getTime())
    })

    it('should update lastUpdated timestamp on failure', async () => {
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const beforeFetch = new Date()
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      await fetchFluxReport()

      expect(reportUpdatedAt.value).toBeInstanceOf(Date)
      expect(reportUpdatedAt.value.getTime()).toBeGreaterThanOrEqual(beforeFetch.getTime())

      consoleErrorSpy.mockRestore()
    })
  })

  describe('App Component - Loading State', () => {
    it('should show loading spinner when reportLoading=true and no reportData', async () => {
      reportLoading.value = true
      reportData.value = null
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      expect(screen.getByText('Loading Flux status...')).toBeInTheDocument()
      const spinner = document.querySelector('.animate-spin')
      expect(spinner).toBeInTheDocument()

      // Wait for effect to complete
      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })

    it('should show ConnectionStatus in loading state', async () => {
      reportLoading.value = true
      reportData.value = null
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      expect(screen.getByTestId('connection-status')).toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })

    it('should have loading spinner with proper styling', async () => {
      reportLoading.value = true
      reportData.value = null
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

    it('should not show loading state if reportData exists', async () => {
      reportLoading.value = true
      reportData.value = { spec: { distribution: { version: 'v2.4.0' } } }
      fetchWithMock.mockResolvedValue({ spec: {} })

      render(<App />)

      expect(screen.queryByText('Loading Flux status...')).not.toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })
  })

  describe('App Component - Error State', () => {
    it('should not show error if reportData exists even when disconnected', async () => {
      reportLoading.value = false
      reportData.value = { spec: { distribution: { version: 'v2.4.0' } } }
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

    it('should render ClusterPage on root path', () => {
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      expect(screen.getByTestId('dashboard-view')).toBeInTheDocument()
    })

    it('should pass spec to ClusterPage', () => {
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      const dashboardView = screen.getByTestId('dashboard-view')
      expect(dashboardView.textContent).toContain('v2.4.0')
    })

    it('should render Header in normal state', () => {
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      expect(screen.getByTestId('header')).toBeInTheDocument()
    })

    it('should render ConnectionStatus in normal state', () => {
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      expect(screen.getByTestId('connection-status')).toBeInTheDocument()
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
    it('should export reportData signal', () => {
      expect(reportData.value).toBeNull()
      reportData.value = { spec: {} }
      expect(reportData.value).toEqual({ spec: {} })
    })

    it('should export reportUpdatedAt signal', () => {
      expect(reportUpdatedAt.value).toBeNull()
      const now = new Date()
      reportUpdatedAt.value = now
      expect(reportUpdatedAt.value).toBe(now)
    })

    it('should export reportLoading signal with default true', () => {
      expect(reportLoading.value).toBe(true)
      reportLoading.value = false
      expect(reportLoading.value).toBe(false)
    })

    it('should export reportError signal with default null', () => {
      expect(reportError.value).toBeNull()
      reportError.value = 'Test error'
      expect(reportError.value).toBe('Test error')
    })

    it('should export connectionStatus signal with default loading', () => {
      expect(connectionStatus.value).toBe('loading')
      connectionStatus.value = 'connected'
      expect(connectionStatus.value).toBe('connected')
    })
  })

  describe('Layout and Styling', () => {
    it('should have min-h-screen on all states', () => {
      reportLoading.value = true
      reportData.value = null

      const { container, rerender } = render(<App />)
      expect(container.querySelector('.min-h-screen')).toBeInTheDocument()

      reportLoading.value = false
      connectionStatus.value = 'disconnected'
      rerender(<App />)
      expect(container.querySelector('.min-h-screen')).toBeInTheDocument()

      reportData.value = { spec: {} }
      rerender(<App />)
      expect(container.querySelector('.min-h-screen')).toBeInTheDocument()
    })

    it('should have dark mode support classes', () => {
      reportLoading.value = false
      reportData.value = { spec: {} }

      const { container } = render(<App />)
      const root = container.querySelector('.bg-gray-50')
      expect(root).toHaveClass('dark:bg-gray-900')
    })

    it('should have transition-colors class', () => {
      reportLoading.value = false
      reportData.value = { spec: {} }

      const { container } = render(<App />)
      expect(container.querySelector('.transition-colors')).toBeInTheDocument()
    })

    it('should have flex-col layout', () => {
      reportLoading.value = false
      reportData.value = { spec: {} }

      const { container } = render(<App />)
      expect(container.querySelector('.flex-col')).toBeInTheDocument()
    })
  })

  describe('Edge Cases', () => {
    it('should require valid reportData with spec to render normal state', async () => {
      // In practice, the app always shows loading or error state if reportData is invalid
      // This test verifies that the normal render path requires a valid spec

      reportLoading.value = false
      reportData.value = { spec: { distribution: { version: 'v2.4.0' } } }
      fetchWithMock.mockResolvedValue({ spec: { distribution: { version: 'v2.4.0' } } })

      const { container } = render(<App />)

      // Should render successfully with valid spec
      expect(container).toBeInTheDocument()
      expect(screen.getByTestId('dashboard-view')).toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })

    it('should handle spec with null values in nested properties', async () => {
      reportLoading.value = false
      reportData.value = {
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

    it('should handle reportData with null metadata', async () => {
      reportLoading.value = false
      reportData.value = {
        spec: { distribution: { version: 'v2.4.0' } },
        metadata: null
      }
      fetchWithMock.mockResolvedValue({ spec: {} })

      const { container } = render(<App />)

      expect(container).toBeInTheDocument()
      expect(screen.getByTestId('dashboard-view')).toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })

    it('should handle reportData with undefined metadata', async () => {
      reportLoading.value = false
      reportData.value = {
        spec: { distribution: { version: 'v2.4.0' } }
        // metadata is undefined
      }
      fetchWithMock.mockResolvedValue({ spec: {} })

      const { container } = render(<App />)

      expect(container).toBeInTheDocument()
      expect(screen.getByTestId('dashboard-view')).toBeInTheDocument()

      await waitFor(() => expect(fetchWithMock).toHaveBeenCalled())
    })
  })

  describe('TabNavigation Component', () => {
    const mockReport = {
      spec: {
        distribution: { version: 'v2.4.0' },
        components: [],
        reconcilers: []
      },
      metadata: { namespace: 'flux-system' }
    }

    it('should show tab navigation on /resources path', () => {
      mockLocationPath = '/resources'
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      expect(screen.getByRole('button', { name: 'Resources' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Events' })).toBeInTheDocument()
    })

    it('should show tab navigation on /events path', () => {
      mockLocationPath = '/events'
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      expect(screen.getByRole('button', { name: 'Resources' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Events' })).toBeInTheDocument()
    })

    it('should not show tab navigation on root path', () => {
      mockLocationPath = '/'
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      expect(screen.queryByRole('button', { name: 'Resources' })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Events' })).not.toBeInTheDocument()
    })

    it('should not show tab navigation on resource detail path', () => {
      mockLocationPath = '/resource/HelmRelease/flux-system/podinfo'
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      expect(screen.queryByRole('button', { name: 'Resources' })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Events' })).not.toBeInTheDocument()
    })

    it('should highlight Resources tab when on /resources path', () => {
      mockLocationPath = '/resources'
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      const resourcesTab = screen.getByRole('button', { name: 'Resources' })
      const eventsTab = screen.getByRole('button', { name: 'Events' })

      expect(resourcesTab.className).toContain('border-flux-blue')
      expect(resourcesTab.className).toContain('text-flux-blue')
      expect(eventsTab.className).toContain('border-transparent')
    })

    it('should highlight Events tab when on /events path', () => {
      mockLocationPath = '/events'
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      const resourcesTab = screen.getByRole('button', { name: 'Resources' })
      const eventsTab = screen.getByRole('button', { name: 'Events' })

      expect(eventsTab.className).toContain('border-flux-blue')
      expect(eventsTab.className).toContain('text-flux-blue')
      expect(resourcesTab.className).toContain('border-transparent')
    })

    it('should navigate to /resources when Resources tab is clicked', async () => {
      mockLocationPath = '/events'
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      const resourcesTab = screen.getByRole('button', { name: 'Resources' })
      resourcesTab.click()

      expect(mockRoute).toHaveBeenCalledWith('/resources')
    })

    it('should navigate to /events when Events tab is clicked', async () => {
      mockLocationPath = '/resources'
      reportLoading.value = false
      reportData.value = mockReport

      render(<App />)

      const eventsTab = screen.getByRole('button', { name: 'Events' })
      eventsTab.click()

      expect(mockRoute).toHaveBeenCalledWith('/events')
    })
  })

  describe('App Component - Error State Display', () => {
    it('should show error state when disconnected and no data', async () => {
      // Mock fetchWithMock to reject immediately so the component goes to error state
      fetchWithMock.mockRejectedValue(new Error('Network error'))
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      render(<App />)

      // Wait for the fetch to complete and error state to be set
      await waitFor(() => {
        expect(screen.getByText('Failed to load Flux report')).toBeInTheDocument()
      })

      expect(screen.getByText('Unable to connect to the server. Retrying automatically...')).toBeInTheDocument()

      consoleErrorSpy.mockRestore()
    })

    it('should show ConnectionStatus in error state', async () => {
      fetchWithMock.mockRejectedValue(new Error('Network error'))
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      render(<App />)

      // Wait for the fetch to complete and error state to be set
      await waitFor(() => {
        expect(connectionStatus.value).toBe('disconnected')
      })

      expect(screen.getByTestId('connection-status')).toBeInTheDocument()

      consoleErrorSpy.mockRestore()
    })
  })
})
