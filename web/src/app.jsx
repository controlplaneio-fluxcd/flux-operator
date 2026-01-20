// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect } from 'preact/hooks'
import { signal } from '@preact/signals'
import { LocationProvider, Router, Route, useLocation } from 'preact-iso'
import { fetchWithMock, authRequired, shouldUseMockData } from './utils/fetch'
import { POLL_INTERVAL_MS } from './utils/constants'
import { parseAuthProviderCookie } from './utils/cookies'
import { checkVersionChange } from './utils/version'
import './utils/theme'
import { ConnectionStatus } from './components/layout/ConnectionStatus'
import { Header } from './components/layout/Header'
import { LoginPage } from './components/auth/LoginPage'
import { ClusterPage } from './components/dashboards/cluster/ClusterPage'
import { EventList } from './components/search/EventList'
import { ResourceList } from './components/search/ResourceList'
import { ResourcePage } from './components/dashboards/resource/ResourcePage'
import { FavoritesPage } from './components/favorites/FavoritesPage'
import { ProfilePage } from './components/user/ProfilePage'
import { NotFoundPage } from './components/layout/NotFoundPage'
import { FluxOperatorIcon } from './components/layout/Icons'

// Global signals for FluxReport data and application state
// These signals are exported and used by child components throughout the app

// FluxReport data from API (null until first successful fetch)
export const reportData = signal(null)

// Timestamp of last successful data fetch (used by ClusterStatus component)
export const reportUpdatedAt = signal(null)

// Initial loading state (prevents flash of error state on first load)
export const reportLoading = signal(true)

// Report fetch error message (null if no error)
export const reportError = signal(null)

// Connection status: 'loading' | 'connected' | 'disconnected'
// - 'loading': Currently fetching data
// - 'connected': Successfully connected and fetched data
// - 'disconnected': Failed to fetch data (shows reconnection banner)
export const connectionStatus = signal('loading')


/**
 * Fetches FluxReport data from the API or mock data
 *
 * This function is called:
 * - On initial app mount
 * - Every 30 seconds via auto-refresh interval
 * - Manually when user clicks refresh button in Header
 *
 * Uses fetchWithMock utility which automatically switches between:
 * - Real API: /api/v1/report (production)
 * - Mock data: ../mock/report (development with VITE_USE_MOCK_DATA=true)
 */
export async function fetchFluxReport() {
  // Set loading status (avoid overwriting if already loading)
  if (connectionStatus.value !== 'loading') {
    connectionStatus.value = 'loading'
  }
  // Set reportLoading if we don't have data yet (handles retry after error)
  if (!reportData.value) {
    reportLoading.value = true
  }

  try {
    // Fetch FluxReport from API or mock
    reportData.value = await fetchWithMock({
      endpoint: '/api/v1/report',
      mockPath: '../mock/report',
      mockExport: 'mockReport'
    })

    // Check for operator version change and reload if needed
    const operatorVersion = reportData.value?.spec?.operator?.version
    const fluxVersion = reportData.value?.spec?.distribution?.version
    checkVersionChange(operatorVersion, fluxVersion)

    // Update connection status and timestamps on success
    connectionStatus.value = 'connected'
    reportUpdatedAt.value = new Date()
    reportLoading.value = false
    reportError.value = null // Clear error on success
  } catch (error) {
    // Update connection status on failure
    // Auto-refresh will retry after 30 seconds
    connectionStatus.value = 'disconnected'
    reportUpdatedAt.value = new Date()
    reportLoading.value = false
    reportError.value = error.message // Store error message
  }
}

/**
 * TabNavigation - Tab navigation for switching between Favorites, Resources, and Events views
 */
function TabNavigation() {
  const location = useLocation()
  const currentPath = location.path

  return (
    <div class="border-b border-gray-200 dark:border-gray-700 transition-colors">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <nav class="-mb-px flex space-x-8">
          <a
            href="/favorites"
            class={`${
              currentPath === '/favorites'
                ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
            } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm focus:outline-none transition-colors`}
          >
            Favorites
          </a>
          <a
            href="/resources"
            class={`${
              currentPath === '/resources'
                ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
            } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm focus:outline-none transition-colors`}
          >
            Resources
          </a>
          <a
            href="/events"
            class={`${
              currentPath === '/events'
                ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
            } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm focus:outline-none transition-colors`}
          >
            Events
          </a>
        </nav>
      </div>
    </div>
  )
}

/**
 * App - Root application component
 *
 * Manages:
 * - Initial data fetch and auto-refresh (30s interval)
 * - Application state: loading, error, and normal views
 * - View routing: dashboard vs search
 * - Global layout: connection status, header, content, footer
 *
 * State transitions:
 * 1. Loading: isLoading=true, fluxReport=null → shows spinner
 * 2. Error: isLoading=false, fluxReport=null, disconnected → shows error
 * 3. Normal: fluxReport exists → shows dashboard or search view
 */
export function App() {
  // Setup data fetching on component mount
  useEffect(() => {
    // Skip auth check in mock mode (dev with VITE_USE_MOCK_DATA=true)
    if (!shouldUseMockData()) {
      // Check auth-provider cookie first
      // If user is not authenticated, show login page immediately (skip API call)
      const authProvider = parseAuthProviderCookie()
      if (authProvider && authProvider.authenticated === false) {
        authRequired.value = true
        reportLoading.value = false
        return // Don't fetch or set up interval
      }
    }

    // Fetch data immediately on mount
    fetchFluxReport()

    // Setup auto-refresh interval
    const interval = setInterval(fetchFluxReport, POLL_INTERVAL_MS)

    // Cleanup interval on unmount
    return () => clearInterval(interval)
  }, [])

  // AUTH REQUIRED STATE: Show login page when authentication is needed
  // This can be triggered by:
  // 1. Cookie check on mount (authenticated === false)
  // 2. Any API call returning 401 (auth expired)
  if (authRequired.value) {
    return <LoginPage />
  }

  // LOADING STATE: Show spinner while waiting for initial data
  // Only show loading during initial load (no data and no previous error)
  if (reportLoading.value && !reportData.value && !reportError.value) {
    return (
      <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col">
        <ConnectionStatus />
        <div class="flex items-center justify-center flex-1">
          <div class="text-center">
            <FluxOperatorIcon className="animate-spin h-12 w-12 text-flux-blue mx-auto" />
            <p class="mt-4 text-gray-600 dark:text-gray-400">Loading Flux status...</p>
          </div>
        </div>
      </div>
    )
  }

  // ERROR STATE: Failed to load data and no cached data available
  // Shows error message and "retrying" notice (auto-refresh continues)
  // Use reportError (persists during retry, cleared on success) to avoid flickering
  if (!reportData.value && reportError.value) {
    // If reportError.value has the substring 'server not initialized', show a specific message
    const errorMsg = reportError.value?.includes('server not initialized')
      ? 'Server configuration is not initialized'
      : 'Unable to connect to the server'
    return (
      <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col items-center justify-center px-4 py-12 sm:py-0">
        <ConnectionStatus />
        <div class="w-full max-w-md">
          {/* Logo and Title */}
          <div class="flex items-center justify-center gap-3 mb-8">
            <FluxOperatorIcon className="w-12 h-12 text-gray-900 dark:text-white" />
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
              Flux Status
            </h1>
          </div>

          {/* Card */}
          <div class="bg-white dark:bg-gray-800 rounded-lg shadow-md border border-gray-200 dark:border-gray-700 p-8">
            {/* Error Icon */}
            <div class="flex justify-center mb-6">
              <div class="w-16 h-16 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
                <svg class="w-8 h-8 text-red-500 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
              </div>
            </div>

            {/* Message */}
            <div class="text-center mb-6">
              <h2 class="text-xl font-medium text-gray-900 dark:text-white mb-3">
                Flux API Server Unavailable
              </h2>
              <p class="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                {errorMsg}
              </p>
            </div>

            {/* Retry Indicator */}
            <div class="flex items-center justify-center gap-2 text-gray-500 dark:text-gray-400">
              <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
              </svg>
              <span class="text-sm">Retrying automatically...</span>
            </div>
          </div>

          {/* Documentation Link */}
          <div class="mt-6 text-center">
            <a
              href="https://fluxoperator.dev/docs/"
              target="_blank"
              rel="noopener noreferrer"
              class="text-sm text-gray-500 dark:text-gray-400 hover:text-flux-blue dark:hover:text-blue-400 transition-colors inline-flex items-center gap-1"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              Documentation
            </a>
          </div>
        </div>
      </div>
    )
  }

  // Extract spec and metadata from FluxReport for child components
  const { spec, metadata } = reportData.value

  // NORMAL STATE: Render full application layout with routing
  return (
    <LocationProvider>
      <AppContent spec={spec} namespace={metadata?.namespace} />
    </LocationProvider>
  )
}

/**
 * AppContent - Main app content with routing
 * Separated to allow useLocation() hook usage
 */
function AppContent({ spec, namespace }) {
  const location = useLocation()
  const currentPath = location.path
  const isTabView = currentPath === '/favorites' || currentPath === '/events' || currentPath === '/resources'

  return (
    <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col">
      {/* Connection status banner (only visible when disconnected) */}
      <ConnectionStatus />

      {/* Header with navigation and refresh button */}
      <Header />

      {/* Tab Navigation - Show only in tab views */}
      {isTabView && <TabNavigation />}

      {/* Main content area: route-based navigation */}
      <Router>
        <Route path="/" component={ClusterPage} spec={spec} namespace={namespace} />
        <Route path="/favorites" component={FavoritesPage} />
        <Route path="/events" component={EventList} />
        <Route path="/resources" component={ResourceList} />
        <Route path="/resource/:kind/:namespace/:name" component={ResourcePage} />
        <Route path="/user/profile" component={ProfilePage} />
        <Route default component={NotFoundPage} />
      </Router>
    </div>
  )
}
