// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect } from 'preact/hooks'
import { signal } from '@preact/signals'
import { fetchWithMock } from './utils/fetch'
import './utils/theme'
import { ConnectionStatus } from './components/ConnectionStatus'
import { Header } from './components/Header'
import { DashboardView } from './components/DashboardView'
import { SearchView } from './components/SearchView'

// Global signals for FluxReport data and application state
// These signals are exported and used by child components throughout the app

// FluxReport data from API (null until first successful fetch)
export const fluxReport = signal(null)

// Timestamp of last successful data fetch (used by ClusterStatus component)
export const lastUpdated = signal(null)

// Initial loading state (prevents flash of error state on first load)
export const isLoading = signal(true)

// Connection status: 'loading' | 'connected' | 'disconnected'
// - 'loading': Currently fetching data
// - 'connected': Successfully connected and fetched data
// - 'disconnected': Failed to fetch data (shows reconnection banner)
export const connectionStatus = signal('loading')

// View toggle signal: false = dashboard view, true = search view
// Dashboard shows overview cards, search shows filterable events/resources
export const showSearchView = signal(false)

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

  try {
    // Fetch FluxReport from API or mock
    fluxReport.value = await fetchWithMock({
      endpoint: '/api/v1/report',
      mockPath: '../mock/report',
      mockExport: 'mockReport'
    })

    // Update connection status and timestamps on success
    connectionStatus.value = 'connected'
    lastUpdated.value = new Date()
    isLoading.value = false
  } catch (error) {
    console.error('Failed to fetch report:', error)

    // Update connection status on failure
    // Auto-refresh will retry after 30 seconds
    connectionStatus.value = 'disconnected'
    lastUpdated.value = new Date()
    isLoading.value = false
  }
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
    // Fetch data immediately on mount
    fetchFluxReport()

    // Setup auto-refresh interval (30 seconds)
    const interval = setInterval(fetchFluxReport, 30000)

    // Cleanup interval on unmount
    return () => clearInterval(interval)
  }, [])

  // LOADING STATE: Show spinner while waiting for initial data
  // Only show loading if we don't have any data yet
  if (isLoading.value && !fluxReport.value) {
    return (
      <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col">
        <ConnectionStatus />
        <div class="flex items-center justify-center flex-1">
          <div class="text-center">
            <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-flux-blue mx-auto"></div>
            <p class="mt-4 text-gray-600 dark:text-gray-400">Loading Flux status...</p>
          </div>
        </div>
      </div>
    )
  }

  // ERROR STATE: Failed to load data and no cached data available
  // Shows error message and "retrying" notice (auto-refresh continues)
  if (!fluxReport.value && connectionStatus.value === 'disconnected') {
    return (
      <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col">
        <ConnectionStatus />
        <div class="flex items-center justify-center flex-1">
          <div class="text-center">
            <p class="text-red-600 dark:text-red-400 text-lg font-semibold">Failed to load Flux report</p>
            <p class="mt-2 text-gray-600 dark:text-gray-400 text-sm">
              Unable to connect to the server. Retrying automatically...
            </p>
          </div>
        </div>
      </div>
    )
  }

  // Extract spec from FluxReport for child components
  const { spec } = fluxReport.value

  // NORMAL STATE: Render full application layout
  return (
    <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col">
      {/* Connection status banner (only visible when disconnected) */}
      <ConnectionStatus />

      {/* Header with navigation and refresh button */}
      <Header />

      {/* Main content area: toggle between search and dashboard views */}
      {showSearchView.value ? (
        <SearchView />
      ) : (
        <DashboardView spec={spec} />
      )}
    </div>
  )
}
