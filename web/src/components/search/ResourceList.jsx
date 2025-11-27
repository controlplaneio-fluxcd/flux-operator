// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect, useState } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../utils/fetch'
import { formatTimestamp } from '../../utils/time'
import { getStatusBadgeClass } from '../../utils/status'
import { reportData } from '../../app'
import { FilterForm } from './FilterForm'
import { ResourceDetailsView } from './ResourceDetailsView'
import { useRestoreFiltersFromUrl, useSyncFiltersToUrl } from '../../utils/routing'
import { StatusChart } from './StatusChart'
import { useInfiniteScroll } from '../../utils/scroll'

// Resources data signals
export const resourcesData = signal([])
export const resourcesLoading = signal(false)
export const resourcesError = signal(null)

// Filter signals - NO default namespace (show all namespaces)
export const selectedResourceKind = signal('')
export const selectedResourceName = signal('')
export const selectedResourceNamespace = signal('')
export const selectedResourceStatus = signal('')

// Fetch resources from API
export async function fetchResourcesStatus() {
  resourcesLoading.value = true
  resourcesError.value = null

  const params = new URLSearchParams()
  if (selectedResourceKind.value) params.append('kind', selectedResourceKind.value)
  if (selectedResourceName.value) params.append('name', selectedResourceName.value)
  if (selectedResourceNamespace.value) params.append('namespace', selectedResourceNamespace.value)
  if (selectedResourceStatus.value) params.append('status', selectedResourceStatus.value)

  try {
    const data = await fetchWithMock({
      endpoint: `/api/v1/resources?${params.toString()}`,
      mockPath: '../mock/resources',
      mockExport: 'getMockResources'
    })
    resourcesData.value = data.resources || []
  } catch (error) {
    console.error('Failed to fetch resources:', error)
    resourcesError.value = error.message
    resourcesData.value = []
  } finally {
    resourcesLoading.value = false
  }
}


/**
 * ResourceCard - Individual card displaying a Flux resource with status and details
 *
 * @param {Object} props
 * @param {Object} props.resource - Resource object with kind, name, status, message
 *
 * Features:
 * - Shows resource kind, namespace, and name
 * - Displays status badge (Ready, Failed, Progressing, Suspended, Unknown)
 * - Shows status message with expand/collapse for long messages
 * - Displays last reconciled timestamp
 * - Expandable details section showing spec and inventory (lazy-loaded via ResourceDetailsView)
 */
function ResourceCard({ resource }) {
  const location = useLocation()
  const [isExpanded, setIsExpanded] = useState(false)
  const [isDetailsExpanded, setIsDetailsExpanded] = useState(false)

  // Handle resource name click - navigate to dashboard
  const handleResourceClick = () => {
    location.route(`/resource/${encodeURIComponent(resource.kind)}/${encodeURIComponent(resource.namespace)}/${encodeURIComponent(resource.name)}`)
  }

  // Check if message is long or contains newlines
  const isLongMessage = resource.message.length > 150 || resource.message.includes('\n')
  const shouldTruncate = isLongMessage && !isExpanded

  // Truncate to first line or 150 chars
  const getTruncatedMessage = () => {
    const firstLine = resource.message.split('\n')[0]
    if (firstLine.length > 150) {
      return firstLine.substring(0, 150) + '...'
    }
    return firstLine
  }

  const displayMessage = shouldTruncate ? getTruncatedMessage() : resource.message

  return (
    <div class="card p-4 hover:shadow-md transition-shadow">
      {/* Header row: kind + status badge + timestamp */}
      <div class="flex items-center justify-between mb-3">
        <div class="flex items-center gap-3">
          <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">
            {resource.kind}
          </span>
          <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(resource.status)}`}>
            {resource.status}
          </span>
        </div>
        <span class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap ml-4">
          {formatTimestamp(resource.lastReconciled)}
        </span>
      </div>

      {/* Resource namespace/name - clickable link to dashboard */}
      <div class="mb-2">
        <button
          onClick={handleResourceClick}
          class="font-mono text-sm text-left hover:opacity-80 transition-opacity focus:outline-none focus:ring-2 focus:ring-flux-blue focus:ring-offset-2 rounded inline-block group"
        >
          <span class="text-gray-500 dark:text-gray-400">{resource.namespace}/</span><span class="font-semibold text-gray-900 dark:text-gray-100 group-hover:text-flux-blue dark:group-hover:text-blue-400">{resource.name}</span><svg class="w-3.5 h-3.5 text-gray-400 group-hover:text-flux-blue dark:group-hover:text-blue-400 transition-colors ml-1 inline-block align-middle" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
        </button>
      </div>

      {/* Message */}
      <div class="text-sm text-gray-700 dark:text-gray-300">
        <pre class="whitespace-pre-wrap break-words font-sans">
          {displayMessage}
        </pre>
        {isLongMessage && (
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            class="text-flux-blue dark:text-blue-400 text-xs mt-1 hover:underline focus:outline-none"
          >
            {isExpanded ? 'Show less' : 'Show more'}
          </button>
        )}
      </div>

      {/* Details Panel - Spec + Inventory (Lazy Loaded) */}
      <div class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700">
        <button
          onClick={() => setIsDetailsExpanded(!isDetailsExpanded)}
          class="flex items-center space-x-2 text-sm text-gray-700 dark:text-gray-300 hover:text-flux-blue dark:hover:text-blue-400 focus:outline-none transition-colors"
        >
          <svg
            class={`w-4 h-4 transition-transform ${isDetailsExpanded ? 'rotate-90' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
          </svg>
          <span class="font-medium">Details</span>
        </button>
      </div>

      {/* ResourceDetailsView Component */}
      <ResourceDetailsView
        kind={resource.kind}
        name={resource.name}
        namespace={resource.namespace}
        isExpanded={isDetailsExpanded}
      />
    </div>
  )
}

/**
 * ResourceList component - Displays and filters Flux resource statuses
 *
 * Features:
 * - Fetches resource statuses from the API with optional filters (kind, name, namespace, status)
 * - Auto-refetches when filter signals change
 * - Displays resources in card format with status badges and expandable inventory
 * - Sorts resources by last reconciled timestamp (newest first)
 * - Shows loading, error, and empty states
 */
export function ResourceList() {
  // Restore filter signals from URL query params on mount
  useRestoreFiltersFromUrl({
    kind: selectedResourceKind,
    name: selectedResourceName,
    namespace: selectedResourceNamespace,
    status: selectedResourceStatus
  })

  // Sync filter signals to URL query params on change (debounced)
  useSyncFiltersToUrl({
    kind: selectedResourceKind,
    name: selectedResourceName,
    namespace: selectedResourceNamespace,
    status: selectedResourceStatus
  })

  // Fetch resources on mount and when filters change
  useEffect(() => {
    fetchResourcesStatus()
  }, [selectedResourceKind.value, selectedResourceName.value, selectedResourceNamespace.value, selectedResourceStatus.value])

  // Infinite scroll hook - reset when filters change or data refetches
  const { visibleCount, sentinelRef, hasMore, loadMore } = useInfiniteScroll({
    totalItems: resourcesData.value.length,
    pageSize: 100,
    deps: [selectedResourceKind.value, selectedResourceName.value, selectedResourceNamespace.value, selectedResourceStatus.value, resourcesData.value.length]
  })

  // Get visible resources (slice the array - already sorted by server)
  const visibleResources = resourcesData.value.slice(0, visibleCount)

  const handleClearFilters = () => {
    selectedResourceKind.value = ''
    selectedResourceName.value = ''
    selectedResourceNamespace.value = ''
    selectedResourceStatus.value = ''
  }

  return (
    <main data-testid="resource-list" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">
        {/* Page Title */}
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-bold text-gray-900 dark:text-white">Flux Resources</h2>
          {/* Resource count */}
          {!resourcesLoading.value && resourcesData.value.length > 0 && (
            <span class="text-sm text-gray-600 dark:text-gray-400">
              {resourcesData.value.length} resources
            </span>
          )}
        </div>

        {/* Filters */}
        <div class="card p-4">
          <FilterForm
            kindSignal={selectedResourceKind}
            nameSignal={selectedResourceName}
            namespaceSignal={selectedResourceNamespace}
            statusSignal={selectedResourceStatus}
            namespaces={reportData.value?.spec?.namespaces || []}
            onClear={handleClearFilters}
          />
        </div>

        {/* Status Chart */}
        <StatusChart items={resourcesData.value} loading={resourcesLoading.value} mode="resources" />

        {/* Error State */}
        {resourcesError.value && (
          <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
            <div class="flex">
              <svg class="w-5 h-5 text-red-400 dark:text-red-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
              </svg>
              <div class="ml-3">
                <p class="text-sm text-red-800 dark:text-red-200">
                  Failed to load resources: {resourcesError.value}
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Resources List */}
        {/* Empty State */}
        {!resourcesLoading.value && resourcesData.value.length === 0 && (
          <div class="card py-12">
            <div class="text-center">
              <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
              </svg>
              <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
                No resources found for the selected filters
              </p>
            </div>
          </div>
        )}

        {/* Resource Cards */}
        {!resourcesLoading.value && resourcesData.value.length > 0 && (
          <div class="space-y-4">
            {visibleResources.map((resource, index) => (
              <ResourceCard key={`${resource.namespace}-${resource.kind}-${resource.name}-${index}`} resource={resource} />
            ))}

            {/* Sentinel element for infinite scroll */}
            {hasMore && <div ref={sentinelRef} class="h-4" />}

            {/* Load more button - fallback for browsers without IntersectionObserver */}
            {hasMore && !window.IntersectionObserver && (
              <div class="flex justify-center py-4">
                <button
                  onClick={loadMore}
                  class="px-4 py-2 bg-flux-blue text-white rounded-md hover:bg-blue-600 transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue focus:ring-offset-2"
                >
                  Load more resources ({resourcesData.value.length - visibleCount} remaining)
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </main>
  )
}
