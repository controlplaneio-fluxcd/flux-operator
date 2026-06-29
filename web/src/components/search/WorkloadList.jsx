// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect, useState } from 'preact/hooks'
import { fetchWithMock } from '../../utils/fetch'
import { formatTimestamp } from '../../utils/time'
import { getStatusDotClass, getWorkloadStatusBadgeClass } from '../../utils/status'
import { usePageMeta } from '../../utils/meta'
import { workloadKinds } from '../../utils/constants'
import { reportData } from '../../app'
import { FilterForm } from './FilterForm'
import { FilterBar } from './FilterBar'
import { WorkloadDetailsView } from './WorkloadDetailsView'
import { Star, KindChip, NEUTRAL_CHIP, NameLink, Chevron, Spinner, useDisclosure, Reveal, patchRowInSignal } from '../common/rowKit'
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
 * WorkloadRow - Compact list row for a Kubernetes workload and its parent reconciler
 *
 * @param {Object} props
 * @param {Object} props.workload - WorkloadRef object with kind, name, namespace, reconciler ref
 *   fields (reconcilerKind, reconcilerNamespace, reconcilerName, reconcilerStatus) and lastReconciled
 *
 * Features:
 * - A neutral kind chip: a workload carries no status of its own in the list (the
 *   index only knows the reconciler's status), so the chip is never colored by status.
 * - Desktop: a "managed by" line referencing the parent reconciler, with a small
 *   status dot that reflects the reconciler's status (never the reconciler message).
 * - Mobile: a second line with the neutral chip and the reconcile time (a workload
 *   has no status word of its own to show).
 * - Favorite star (kind-agnostic favorites store) and an expand chevron that lazily
 *   mounts {@link WorkloadDetailsView}, spinning until the detail fetch settles.
 */
function WorkloadRow({ workload }) {
  // Disclosure state: the chevron spins while the lazily mounted detail panel
  // fetches, then the panel animates open via Reveal.
  const d = useDisclosure()

  // The workload's own status (Current/Idle/InProgress/…) is only known once the
  // detail panel has fetched it; the list index carries no workload status. We
  // capture it on expand to color the kind chip, and fall back to the neutral
  // chip while collapsed.
  const [liveStatus, setLiveStatus] = useState(null)

  // Check if workload is a favorite (reactive via favorites signal)
  const isFavorited = favorites.value && isFavorite(workload.kind, workload.namespace, workload.name)

  // Handle favorite toggle (stop propagation so the row click is unaffected)
  const handleFavoriteClick = (e) => {
    e.stopPropagation()
    toggleFavorite(workload.kind, workload.namespace, workload.name)
  }

  const dashboardUrl = getDashboardUrl(workload.kind, workload.namespace, workload.name)
  const reconcilerTitle = `Managed by ${workload.reconcilerKind} ${workload.reconcilerNamespace}/${workload.reconcilerName} (${workload.reconcilerStatus})`
  const chipTitle = `${workload.kind} · reconciler ${workload.reconcilerStatus}`

  // Color the kind chip by the workload's own status only while the panel is open
  // (and once its status is known); collapsed rows keep the neutral chip.
  const chipColor = d.open && liveStatus ? getWorkloadStatusBadgeClass(liveStatus) : NEUTRAL_CHIP

  // When the detail panel finishes loading, refresh this row's owning-reconciler
  // summary from the detail's enriched reconciler (its status.reconcilerRef is the
  // same NewResourceStatus the workloads index uses), so a row that listed a stale
  // reconciler status/time updates on expand. No-op when unchanged.
  const handleData = (data) => {
    // Capture the workload's own status to color the kind chip while expanded.
    setLiveStatus(data?.workloadInfo?.status || null)
    const ref = data?.workloadInfo?.reconciler?.status?.reconcilerRef
    if (!ref) return
    patchRowInSignal(workloadsData, workload, { reconcilerStatus: ref.status, lastReconciled: ref.lastReconciled })
  }

  return (
    <div class="border-b border-gray-100 dark:border-gray-700/60 last:border-0">
      <div class="px-3 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/30">
        <div class="flex items-center gap-2.5">
          <Star active={isFavorited} onClick={handleFavoriteClick} />
          {/* Neutral chip in the list; colored by the workload's own status once
              expanded (see chipColor). */}
          <KindChip kind={workload.kind} colorClass={chipColor} title={chipTitle} cls="hidden sm:inline-block" />
          <NameLink href={dashboardUrl} namespace={workload.namespace} name={workload.name} />
          {/* Desktop: parent reconciler reference with a status dot. The status
              belongs to the reconciler, never the workload itself. */}
          <span class="hidden sm:flex flex-1 min-w-0 items-center gap-1.5 truncate text-xs text-gray-500 dark:text-gray-400" title={reconcilerTitle}>
            <span class="text-gray-400 dark:text-gray-500 flex-shrink-0">managed by</span>
            <span class={`w-2 h-2 rounded-full flex-shrink-0 ${getStatusDotClass(workload.reconcilerStatus)}`} />
            <span class="truncate min-w-0">{workload.reconcilerKind}/{workload.reconcilerNamespace}/{workload.reconcilerName}</span>
          </span>
          <span class="hidden sm:block shrink-0 text-xs text-gray-400 dark:text-gray-500 whitespace-nowrap tabular-nums">{formatTimestamp(workload.lastReconciled)}</span>
          <button onClick={d.toggle} class="shrink-0 rounded p-0.5 text-gray-400 hover:text-flux-blue" title="Details" aria-label="Toggle details">{d.loading ? <Spinner /> : <Chevron open={d.open} />}</button>
        </div>
        {/* Mobile-only second line: neutral kind pill + reconciled time. A workload
            has no status of its own in the list, so we state the reconcile time
            instead of a (reconciler) status word. */}
        <div class="sm:hidden mt-1 pl-[30px] flex items-center gap-2 text-xs text-gray-400 dark:text-gray-500">
          <KindChip kind={workload.kind} colorClass={chipColor} title={chipTitle} />
          <span class="whitespace-nowrap">Reconciled {formatTimestamp(workload.lastReconciled)}</span>
        </div>
      </div>
      <Reveal open={d.open}>
        <div class="pl-3 sm:pl-[42px] pr-3 pt-1 pb-4">
          {/* Lazily mounted on expand; unmounted on collapse so each expand
              re-fetches. onReady flips the disclosure once the fetch settles. */}
          {d.mounted && (
            <WorkloadDetailsView
              kind={workload.kind}
              name={workload.name}
              namespace={workload.namespace}
              reconcilerKind={workload.reconcilerKind}
              reconcilerNamespace={workload.reconcilerNamespace}
              reconcilerName={workload.reconcilerName}
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
 * WorkloadList component - Displays and filters Flux-managed Kubernetes workloads
 *
 * Features:
 * - Fetches workloads from the API with optional filters (kind, name, namespace)
 * - Auto-refetches when filter signals change
 * - Renders workloads as compact rows that expand into an inline detail panel
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
    <main data-testid="workload-list" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-6 pb-8 flex-grow w-full">
      <div class="space-y-3">
        {/* Filters - mobile-collapsing toolbar; no status chart for workloads */}
        <FilterBar count={workloadsData.value.length} label="workloads" loading={workloadsLoading.value}>
          <FilterForm
            kindSignal={selectedWorkloadKind}
            nameSignal={selectedWorkloadName}
            namespaceSignal={selectedWorkloadNamespace}
            namespaces={reportData.value?.spec?.namespaces || []}
            kinds={workloadKinds}
            onClear={handleClearFilters}
            onRefresh={fetchWorkloadsStatus}
          />
        </FilterBar>

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

        {/* Workload Rows - dense list; rows provide their own padding, so drop the
            card's heavy padding on mobile (keep a little on desktop). */}
        {!workloadsLoading.value && workloadsData.value.length > 0 && (
          <div class="card overflow-hidden p-0 sm:p-2">
            {visibleWorkloads.map((workload, index) => (
              <WorkloadRow key={`${workload.namespace}-${workload.kind}-${workload.name}-${index}`} workload={workload} />
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
