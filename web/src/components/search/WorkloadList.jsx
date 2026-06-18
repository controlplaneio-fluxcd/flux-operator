// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect } from 'preact/hooks'
import { fetchWithMock } from '../../utils/fetch'
import { formatTimestamp } from '../../utils/time'
import { getStatusDotClass } from '../../utils/status'
import { usePageMeta } from '../../utils/meta'
import { workloadKinds } from '../../utils/constants'
import { reportData } from '../../app'
import { FilterForm } from './FilterForm'
import { useRestoreFiltersFromUrl, useSyncFiltersToUrl, getDashboardUrl } from '../../utils/routing'
import { useInfiniteScroll } from '../../utils/scroll'
import { isFavorite, toggleFavorite, favorites } from '../../utils/favorites'

// Workloads data signals
export const workloadsData = signal([])
export const workloadsLoading = signal(false)
export const workloadsError = signal(null)

// Filter signals - NO status signal (workloads have no status of their own)
export const selectedWorkloadKind = signal('')
export const selectedWorkloadName = signal('')
export const selectedWorkloadNamespace = signal('')

// Fetch workloads from API
export async function fetchWorkloadsStatus() {
  workloadsLoading.value = true
  workloadsError.value = null

  const params = new URLSearchParams()
  if (selectedWorkloadKind.value) params.append('kind', selectedWorkloadKind.value)
  if (selectedWorkloadName.value) params.append('name', selectedWorkloadName.value)
  if (selectedWorkloadNamespace.value) params.append('namespace', selectedWorkloadNamespace.value)

  try {
    const data = await fetchWithMock({
      endpoint: `/api/v1/workloads?${params.toString()}`,
      mockPath: '../mock/workloads',
      mockExport: 'getMockWorkloadsList'
    })
    workloadsData.value = data.workloads || []
  } catch (error) {
    console.error('Failed to fetch workloads:', error)
    workloadsError.value = error.message
    workloadsData.value = []
  } finally {
    workloadsLoading.value = false
  }
}

/**
 * WorkloadCard - Individual card displaying a Kubernetes workload and its parent reconciler
 *
 * @param {Object} props
 * @param {Object} props.workload - WorkloadRef object with kind, name, namespace, reconciler ref
 *   fields (reconcilerKind, reconcilerNamespace, reconcilerName, reconcilerStatus) and lastReconciled
 *
 * Features:
 * - Shows workload kind, namespace, and name (links to the workload dashboard)
 * - Displays the parent reconciler reference with a small status badge reflecting the
 *   reconciler's status (never the reconciler message)
 * - Favorite star button (writes via the kind-agnostic favorites store)
 */
function WorkloadCard({ workload }) {
  // Check if workload is a favorite (reactive via favorites signal)
  const isFavorited = favorites.value && isFavorite(workload.kind, workload.namespace, workload.name)

  // Handle favorite toggle
  const handleFavoriteClick = (e) => {
    e.stopPropagation()
    toggleFavorite(workload.kind, workload.namespace, workload.name)
  }

  return (
    <div class="card p-4 hover:shadow-md transition-shadow">
      {/* Header row: star + kind + timestamp */}
      <div class="flex items-center justify-between mb-3">
        <div class="flex items-center gap-3">
          {/* Favorite star button */}
          <button
            onClick={handleFavoriteClick}
            class={`p-0.5 rounded transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue focus:ring-offset-1 ${
              isFavorited
                ? 'text-yellow-500 hover:text-yellow-600'
                : 'text-gray-400 hover:text-flux-blue dark:hover:text-blue-400'
            }`}
            title={isFavorited ? 'Remove from favorites' : 'Add to favorites'}
          >
            <svg class="w-4 h-4" fill={isFavorited ? 'currentColor' : 'none'} stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
            </svg>
          </button>
          <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">
            {workload.kind}
          </span>
        </div>
        <span class="hidden sm:inline text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap ml-4">
          {formatTimestamp(workload.lastReconciled)}
        </span>
      </div>

      {/* Workload namespace/name - clickable link to dashboard */}
      <div class="mb-1 sm:mb-2">
        <a
          href={getDashboardUrl(workload.kind, workload.namespace, workload.name)}
          class="text-sm text-left hover:opacity-80 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue rounded inline-block group"
        >
          <span class="text-gray-500 dark:text-gray-400">{workload.namespace}/</span><span class="font-semibold text-gray-900 dark:text-gray-100 group-hover:text-flux-blue dark:group-hover:text-blue-400">{workload.name}</span><svg class="w-3.5 h-3.5 text-gray-400 group-hover:text-flux-blue dark:group-hover:text-blue-400 transition-colors ml-1 inline-block align-middle" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
        </a>
      </div>

      {/* Mobile timestamp - below namespace/name */}
      <div class="flex sm:hidden items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400 mb-2">
        <svg class="w-3.5 h-3.5 text-gray-400 dark:text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        {formatTimestamp(workload.lastReconciled)}
      </div>

      {/* Parent reconciler reference: a muted "Managed by" line with a small status
          dot. The status belongs to the reconciler, never the workload itself. */}
      <div
        class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400"
        title={`Managed by ${workload.reconcilerKind} ${workload.reconcilerNamespace}/${workload.reconcilerName} (${workload.reconcilerStatus})`}
      >
        <span class="flex-shrink-0">Managed by</span>
        <span class={`w-2 h-2 rounded-full flex-shrink-0 ${getStatusDotClass(workload.reconcilerStatus)}`} />
        <span class="truncate min-w-0">{workload.reconcilerKind}/{workload.reconcilerNamespace}/{workload.reconcilerName}</span>
      </div>
    </div>
  )
}

/**
 * WorkloadList component - Displays and filters Flux-managed Kubernetes workloads
 *
 * Features:
 * - Fetches workloads from the API with optional filters (kind, name, namespace)
 * - Auto-refetches when filter signals change
 * - Displays workloads in card format with the parent reconciler status badge
 * - Shows loading, error, and empty states
 * - No status filter and no status chart (workloads have no status of their own)
 */
export function WorkloadList() {
  usePageMeta('Workloads', 'Workloads dashboard')

  // Restore filter signals from URL query params on mount
  useRestoreFiltersFromUrl({
    kind: selectedWorkloadKind,
    name: selectedWorkloadName,
    namespace: selectedWorkloadNamespace
  })

  // Sync filter signals to URL query params on change (debounced)
  useSyncFiltersToUrl({
    kind: selectedWorkloadKind,
    name: selectedWorkloadName,
    namespace: selectedWorkloadNamespace
  })

  // Fetch workloads on mount and when filters change
  useEffect(() => {
    fetchWorkloadsStatus()
  }, [selectedWorkloadKind.value, selectedWorkloadName.value, selectedWorkloadNamespace.value])

  // Infinite scroll hook - reset when filters change or data refetches
  const { visibleCount, sentinelRef, hasMore, loadMore } = useInfiniteScroll({
    totalItems: workloadsData.value.length,
    pageSize: 100,
    deps: [selectedWorkloadKind.value, selectedWorkloadName.value, selectedWorkloadNamespace.value, workloadsData.value.length]
  })

  // Get visible workloads (slice the array - already sorted by server)
  const visibleWorkloads = workloadsData.value.slice(0, visibleCount)

  const handleClearFilters = () => {
    selectedWorkloadKind.value = ''
    selectedWorkloadName.value = ''
    selectedWorkloadNamespace.value = ''
  }

  return (
    <main data-testid="workload-list" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">
        {/* Page Title */}
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Flux Workloads</h2>
          {/* Workload count */}
          {!workloadsLoading.value && workloadsData.value.length > 0 && (
            <span class="text-sm text-gray-600 dark:text-gray-400">
              {workloadsData.value.length} workloads
            </span>
          )}
        </div>

        {/* Filters */}
        <div class="card p-4">
          <FilterForm
            kindSignal={selectedWorkloadKind}
            nameSignal={selectedWorkloadName}
            namespaceSignal={selectedWorkloadNamespace}
            namespaces={reportData.value?.spec?.namespaces || []}
            kinds={workloadKinds}
            onClear={handleClearFilters}
          />
        </div>

        {/* Error State */}
        {workloadsError.value && (
          <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
            <div class="flex">
              <svg class="w-5 h-5 text-red-400 dark:text-red-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
              </svg>
              <div class="ml-3">
                <p class="text-sm text-red-800 dark:text-red-200">
                  Failed to load workloads: {workloadsError.value}
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Empty State */}
        {!workloadsLoading.value && workloadsData.value.length === 0 && (
          <div class="card py-12">
            <div class="text-center">
              <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
              </svg>
              <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
                No workloads found for the selected filters
              </p>
            </div>
          </div>
        )}

        {/* Workload Cards */}
        {!workloadsLoading.value && workloadsData.value.length > 0 && (
          <div class="space-y-4">
            {visibleWorkloads.map((workload, index) => (
              <WorkloadCard key={`${workload.namespace}-${workload.kind}-${workload.name}-${index}`} workload={workload} />
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
                  Load more workloads ({workloadsData.value.length - visibleCount} remaining)
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </main>
  )
}
