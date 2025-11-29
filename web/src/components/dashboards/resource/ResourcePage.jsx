// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../../utils/fetch'
import { usePrismTheme } from '../common/yaml'
import { formatTime } from '../../../utils/time'
import { usePageTitle } from '../../../utils/title'
import { isFavorite, toggleFavorite, favorites } from '../../../utils/favorites'
import { ReconcilerPanel } from './ReconcilerPanel'
import { SourcePanel } from './SourcePanel'
import { InventoryPanel } from './InventoryPanel'
import { ArtifactPanel } from './ArtifactPanel'
import { ExportedInputsPanel } from './ExportedInputsPanel'
import { InputsPanel } from './InputsPanel'

/**
 * Get status styling info
 */
function getStatusInfo(status) {
  switch (status) {
  case 'Ready':
    return {
      color: 'text-success',
      bgColor: 'bg-green-50',
      borderColor: 'border-success',
      icon: (
        <svg class="w-10 h-10 text-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
      )
    }
  case 'Failed':
    return {
      color: 'text-danger',
      bgColor: 'bg-red-50',
      borderColor: 'border-danger',
      icon: (
        <svg class="w-10 h-10 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      )
    }
  case 'Progressing':
    return {
      color: 'text-blue-600 dark:text-blue-400',
      bgColor: 'bg-blue-50',
      borderColor: 'border-blue-500',
      icon: (
        <svg class="w-10 h-10 text-blue-600 dark:text-blue-400 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      )
    }
  case 'Suspended':
    return {
      color: 'text-yellow-600 dark:text-yellow-400',
      bgColor: 'bg-yellow-50',
      borderColor: 'border-yellow-500',
      icon: (
        <svg class="w-10 h-10 text-yellow-600 dark:text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    }
  default:
    return {
      color: 'text-gray-600 dark:text-gray-400',
      bgColor: 'bg-gray-50',
      borderColor: 'border-gray-400',
      icon: (
        <svg class="w-10 h-10 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    }
  }
}

/**
 * ResourcePage - Full page dashboard for a single Flux resource
 */
export function ResourcePage({ kind, namespace, name }) {
  const location = useLocation()

  // Set page title to resource name
  usePageTitle(name)

  // State
  const [resourceData, setResourceData] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [lastUpdatedAt, setLastUpdatedAt] = useState(null)

  // Load Prism theme based on current app theme
  usePrismTheme()

  // Reset state when navigating to a different resource
  useEffect(() => {
    setResourceData(null)
    setLoading(true)
    setError(null)
  }, [kind, namespace, name])

  // Fetch data
  useEffect(() => {
    const fetchData = async () => {
      // Clear error before fetching (will be set again if fetch fails)
      setError(null)

      const params = new URLSearchParams({ kind, name, namespace })

      try {
        const resourceResp = await fetchWithMock({
          endpoint: `/api/v1/resource?${params.toString()}`,
          mockPath: '../mock/resource',
          mockExport: 'getMockResource'
        })

        setResourceData(resourceResp)
        setLastUpdatedAt(new Date())
        setError(null) // Clear error on success
      } catch (err) {
        setError(err.message)
        // Don't clear existing data on error - keep showing stale data
      } finally {
        setLoading(false)
      }
    }

    // Fetch data immediately
    fetchData()

    // Setup auto-refresh interval (30 seconds)
    const interval = setInterval(fetchData, 30000)

    // Cleanup interval on unmount or when dependencies change
    return () => clearInterval(interval)
  }, [kind, namespace, name])

  // Derived data
  const status = resourceData?.status?.reconcilerRef?.status || 'Unknown'
  const statusInfo = getStatusInfo(status)
  const hasSource = resourceData?.status?.sourceRef
  const isSourceResource = resourceData?.apiVersion?.startsWith('source.toolkit.fluxcd.io/') && resourceData?.kind !== 'HelmChart'
  const isResourceSetInputProvider = resourceData?.kind === 'ResourceSetInputProvider'
  const isResourceSet = resourceData?.kind === 'ResourceSet'

  // Check if resource is a favorite (reactive via favorites signal)
  // Access favorites.value to subscribe to changes and trigger re-renders
  const isFavorited = favorites.value && isFavorite(kind, namespace, name)

  // Handle favorite toggle
  const handleFavoriteClick = (e) => {
    e.stopPropagation()
    toggleFavorite(kind, namespace, name)
  }

  // Navigate to another resource
  const handleNavigate = (item) => {
    const ns = item.namespace || namespace
    location.route(`/resource/${encodeURIComponent(item.kind)}/${encodeURIComponent(ns)}/${encodeURIComponent(item.name)}`)
  }

  // Loading state - only show spinner on initial load when no data exists
  if (loading && !resourceData) {
    return (
      <main data-testid="resource-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="flex items-center justify-center p-8">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-flux-blue"></div>
          <span class="ml-3 text-gray-600 dark:text-gray-400">Loading resource...</span>
        </div>
      </main>
    )
  }

  // Error state - only show error screen when no data exists (initial load failure)
  // On auto-refresh failure, keep showing stale data
  if (error && !resourceData) {
    return (
      <main data-testid="resource-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
          <p class="text-sm text-red-800 dark:text-red-200">Failed to load resource: {error}</p>
        </div>
      </main>
    )
  }

  // Not found state - check if resourceData is empty or missing required fields
  if (!resourceData || !resourceData.metadata || !resourceData.metadata.name) {
    return (
      <main data-testid="resource-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="card bg-red-50 dark:bg-opacity-20 border-2 border-danger">
          <div class="flex items-center space-x-4">
            <div class="flex-shrink-0">
              <div class="w-16 h-16 rounded-full bg-red-50 dark:bg-opacity-30 flex items-center justify-center">
                <svg class="w-10 h-10 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
              </div>
            </div>
            <div class="flex-grow min-w-0">
              <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{kind}</span>
              <h1 class="text-2xl font-bold text-gray-900 dark:text-white font-mono break-all">
                {name} not found
              </h1>
              <span class="text-sm text-gray-500 dark:text-gray-400">Namespace: {namespace}</span>
            </div>
          </div>
        </div>
      </main>
    )
  }

  return (
    <main data-testid="resource-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">

        {/* Header */}
        <div class={`card ${statusInfo.bgColor} dark:bg-opacity-20 border-2 ${statusInfo.borderColor}`}>
          <div class="flex items-center space-x-4">
            <div class="flex-shrink-0">
              <div class={`w-16 h-16 rounded-full ${statusInfo.bgColor} dark:bg-opacity-30 flex items-center justify-center`}>
                {statusInfo.icon}
              </div>
            </div>
            <div class="flex-grow min-w-0">
              <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{kind}</span>
              <h1 class="text-lg sm:text-2xl font-bold text-gray-900 dark:text-white font-mono break-all flex items-center gap-2">
                {name}
                <button
                  onClick={handleFavoriteClick}
                  class={`flex-shrink-0 transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue focus:ring-offset-1 rounded ${
                    isFavorited
                      ? 'text-yellow-500 hover:text-yellow-600'
                      : 'text-gray-300 dark:text-gray-600 hover:text-yellow-500'
                  }`}
                  title={isFavorited ? 'Remove from favorites' : 'Add to favorites'}
                >
                  <svg class="w-5 h-5 sm:w-6 sm:h-6" fill={isFavorited ? 'currentColor' : 'none'} stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
                  </svg>
                </button>
              </h1>
              <span class="text-xs sm:text-sm text-gray-500 dark:text-gray-400">Namespace: {namespace}</span>
            </div>
            <div class="hidden md:block text-right flex-shrink-0">
              <div class="text-sm text-gray-600 dark:text-gray-400">Last Updated</div>
              <div class="text-lg font-semibold text-gray-900 dark:text-white">{formatTime(lastUpdatedAt)}</div>
            </div>
          </div>
        </div>

        {/* Reconciler Section */}
        <ReconcilerPanel
          kind={kind}
          name={name}
          namespace={namespace}
          resourceData={resourceData}
        />

        {/* Artifact Section - for source resources only */}
        {isSourceResource && (
          <ArtifactPanel resourceData={resourceData} />
        )}

        {/* Exported Inputs Section - for ResourceSetInputProvider only */}
        {isResourceSetInputProvider && (
          <ExportedInputsPanel resourceData={resourceData} />
        )}

        {/* Inputs Section - for ResourceSet only */}
        {isResourceSet && (
          <InputsPanel resourceData={resourceData} namespace={namespace} />
        )}

        {/* Managed Objects Section */}
        <InventoryPanel
          resourceData={resourceData}
          onNavigate={handleNavigate}
        />

        {/* Source Section */}
        {hasSource && (
          <SourcePanel
            sourceRef={resourceData.status.sourceRef}
            namespace={namespace}
          />
        )}

      </div>
    </main>
  )
}