// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect } from 'preact/hooks'
import { fetchWithMock } from '../../utils/fetch'
import { formatTimestamp } from '../../utils/time'
import { getStatusBadgeClass } from '../../utils/status'
import { usePageMeta } from '../../utils/meta'
import { reportData } from '../../app'
import { FilterForm } from './FilterForm'
import { FilterBar } from './FilterBar'
import { ResourceDetailsView } from './ResourceDetailsView'
import { getDashboardUrl, useRestoreFiltersFromUrl, useSyncFiltersToUrl } from '../../utils/routing'
import { StatusChart } from './StatusChart'
import { useInfiniteScroll } from '../../utils/scroll'
import { isFavorite, toggleFavorite, favorites } from '../../utils/favorites'
import { Star, KindChip, NameLink, Chevron, Spinner, useDisclosure, Reveal, patchRowInSignal } from '../common/rowKit'

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
 * ResourceRow - Compact two-line responsive row for a Flux resource.
 *
 * @param {Object} props
 * @param {Object} props.resource - Resource object with kind, namespace, name,
 *   status, message and lastReconciled
 *
 * Layout:
 * - Line 1: favorite star, desktop colored kind chip, namespace/name dashboard
 *   link (full-width on mobile, fixed-width NameLink on desktop), desktop status
 *   message, desktop timestamp, and the expand button.
 * - Line 2 (mobile only): colored kind chip, status word and timestamp.
 *
 * Expanding the row lazily mounts {@link ResourceDetailsView}: the expand button
 * spins while the panel fetches, then the panel animates open once `onReady`
 * fires. Collapsing unmounts the panel, so each expand re-fetches fresh data.
 */
function ResourceRow({ resource }) {
  const d = useDisclosure()

  // Check if resource is a favorite (reactive via favorites signal).
  // Access favorites.value to subscribe to changes and trigger re-renders.
  const isFavorited = favorites.value && isFavorite(resource.kind, resource.namespace, resource.name)

  // Toggle favorite without expanding the row.
  const handleFavoriteClick = (e) => {
    e.stopPropagation()
    toggleFavorite(resource.kind, resource.namespace, resource.name)
  }

  const dashboardUrl = getDashboardUrl(resource.kind, resource.namespace, resource.name)
  const chipColor = getStatusBadgeClass(resource.status)
  const chipTitle = `${resource.kind} · ${resource.status}`

  // When the detail panel finishes loading, refresh this row's summary from the
  // detail's reconcilerRef (computed by the same server NewResourceStatus the list
  // uses), so a row that listed as e.g. Failed updates if the resource is now Ready
  // by the time it is expanded. No-op when the summary is unchanged.
  const handleData = (data) => {
    const ref = data?.status?.reconcilerRef
    if (!ref) return
    patchRowInSignal(resourcesData, resource, { status: ref.status, message: ref.message, lastReconciled: ref.lastReconciled })
  }

  return (
    <div class="border-b border-gray-100 dark:border-gray-700/60 last:border-0">
      <div class="px-3 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/30">
        <div class="flex items-center gap-2.5">
          <Star active={isFavorited} onClick={handleFavoriteClick} />
          <KindChip kind={resource.kind} colorClass={chipColor} title={chipTitle} cls="hidden sm:inline-block" />
          <NameLink href={dashboardUrl} namespace={resource.namespace} name={resource.name} />
          <span class="hidden sm:block flex-1 min-w-0 truncate text-xs text-gray-500 dark:text-gray-400">{resource.message}</span>
          <span class="hidden sm:block shrink-0 text-xs text-gray-400 dark:text-gray-500 whitespace-nowrap tabular-nums">{formatTimestamp(resource.lastReconciled)}</span>
          <button onClick={d.toggle} class="shrink-0 rounded p-0.5 text-gray-400 hover:text-flux-blue" title="Details" aria-label="Toggle details">{d.loading ? <Spinner /> : <Chevron open={d.open} />}</button>
        </div>
        {/* Mobile-only second line: colored kind pill + status word + timestamp. */}
        <div class="sm:hidden mt-1 pl-[30px] flex items-center gap-2 text-xs text-gray-400 dark:text-gray-500">
          <KindChip kind={resource.kind} colorClass={chipColor} title={chipTitle} />
          <span class="whitespace-nowrap"><span class="capitalize">{resource.status}</span> {formatTimestamp(resource.lastReconciled)}</span>
        </div>
      </div>
      <Reveal open={d.open}>
        <div class="pl-3 sm:pl-[42px] pr-3 pt-1 pb-4">
          {d.mounted && (
            <ResourceDetailsView
              kind={resource.kind}
              name={resource.name}
              namespace={resource.namespace}
              status={resource.status}
              isExpanded
              onReady={d.onReady}
              onData={handleData}
            />
          )}
        </div>
      </Reveal>
    </div>
  )
}

/**
 * ResourceList component - Displays and filters Flux resource statuses
 *
 * Features:
 * - Fetches resource statuses from the API with optional filters (kind, name, namespace, status)
 * - Auto-refetches when filter signals change
 * - Displays resources as compact two-line responsive rows with a colored kind
 *   chip and an expandable, lazily loaded detail panel
 * - Sorts resources by last reconciled timestamp (newest first)
 * - Shows loading, error, and empty states
 */
export function ResourceList() {
  usePageMeta('Resources', 'Resources dashboard')

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
    <main data-testid="resource-list" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-6 pb-8 flex-grow w-full">
      <div class="space-y-3">
        {/* Toolbar: count + mobile Filters toggle, FilterForm, desktop StatusChart */}
        <FilterBar
          count={resourcesData.value.length}
          label="resources"
          loading={resourcesLoading.value}
          statusChart={
            <StatusChart
              items={resourcesData.value}
              loading={resourcesLoading.value}
              mode="resources"
              compact
              onBarClick={(status) => {
                selectedResourceStatus.value = selectedResourceStatus.value === status ? '' : status
              }}
            />
          }
        >
          <FilterForm
            kindSignal={selectedResourceKind}
            nameSignal={selectedResourceName}
            namespaceSignal={selectedResourceNamespace}
            statusSignal={selectedResourceStatus}
            namespaces={reportData.value?.spec?.namespaces || []}
            onClear={handleClearFilters}
            onRefresh={fetchResourcesStatus}
          />
        </FilterBar>

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

        {/* Resource Rows — dense list; rows provide their own padding, so drop the
            card's heavy padding on mobile and keep a light inset on desktop. */}
        {!resourcesLoading.value && resourcesData.value.length > 0 && (
          <div class="card overflow-hidden p-0 sm:p-2">
            {visibleResources.map((resource, index) => (
              <ResourceRow key={`${resource.namespace}-${resource.kind}-${resource.name}-${index}`} resource={resource} />
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
