// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect, useState } from 'preact/hooks'
import { fetchWithMock } from '../../utils/fetch'
import { formatTimestamp } from '../../utils/time'
import { getEventBadgeClass } from '../../utils/status'
import { usePageMeta } from '../../utils/meta'
import { reportData } from '../../app'
import { FilterForm } from './FilterForm'
import { FilterBar } from './FilterBar'
import { Chevron, KindChip, NameLink, Reveal } from '../common/rowKit'
import { getDashboardUrl, useRestoreFiltersFromUrl, useSyncFiltersToUrl } from '../../utils/routing'
import { StatusChart } from './StatusChart'
import { useInfiniteScroll } from '../../utils/scroll'

// Events data signals
export const eventsData = signal([])
export const eventsLoading = signal(false)
export const eventsError = signal(null)

// Filter signals
export const selectedEventKind = signal('')
export const selectedEventName = signal('')
export const selectedEventNamespace = signal('')
export const selectedEventSeverity = signal('')

// Fetch events from API
export async function fetchEvents() {
  eventsLoading.value = true
  eventsError.value = null

  const params = new URLSearchParams()
  if (selectedEventKind.value) params.append('kind', selectedEventKind.value)
  if (selectedEventName.value) params.append('name', selectedEventName.value)
  if (selectedEventNamespace.value) params.append('namespace', selectedEventNamespace.value)
  if (selectedEventSeverity.value) params.append('type', selectedEventSeverity.value)

  try {
    const data = await fetchWithMock({
      endpoint: `/api/v1/events?${params.toString()}`,
      mockPath: '../mock/events',
      mockExport: 'getMockEvents'
    })
    eventsData.value = data.events || []
  } catch (error) {
    console.error('Failed to fetch events:', error)
    eventsError.value = error.message
    eventsData.value = []
  } finally {
    eventsLoading.value = false
  }
}


/**
 * EventRow - compact list row for a single Kubernetes event.
 *
 * @param {Object} props
 * @param {Object} props.event - Event with type, involvedObject, namespace, message, lastTimestamp
 *
 * The kind chip is colored by event severity (Normal -> Info, Warning). On
 * desktop the row shows the namespace/name link, the truncated message and the
 * timestamp on one line; on mobile the link sits on the top row and a second
 * line repeats the colored chip beside the severity word and timestamp. The
 * namespace/name link routes to the involved object's dashboard. Events already
 * carry their full message in the list, so expanding reveals it inline (a dark
 * rounded block, newlines preserved) instantly with a plain chevron and no
 * spinner — there is nothing to fetch.
 */
function EventRow({ event }) {
  const [open, setOpen] = useState(false)

  // Parse involvedObject "Kind/name" into kind and name.
  const [kind, name] = event.involvedObject.split('/')

  // Build the dashboard URL, routing workload-kind events to the workload dashboard.
  const resourceUrl = getDashboardUrl(kind, event.namespace, name)

  // One-word severity label for the chip tooltip and the mobile second line.
  const severity = event.type === 'Warning' ? 'Warning' : 'Info'
  const chipColor = getEventBadgeClass(event.type)
  const chipTitle = `${kind} · ${severity}`

  return (
    <div class="border-b border-gray-100 dark:border-gray-700/60 last:border-0">
      <div class="px-3 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/30">
        <div class="flex items-center gap-2.5">
          <KindChip kind={kind} colorClass={chipColor} title={chipTitle} cls="hidden sm:inline-block" />
          <NameLink href={resourceUrl} namespace={event.namespace} name={name} cls="rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue" />
          <span class="hidden sm:block flex-1 min-w-0 truncate text-xs text-gray-500 dark:text-gray-400">{event.message}</span>
          <span class="hidden sm:block shrink-0 text-xs text-gray-400 dark:text-gray-500 whitespace-nowrap tabular-nums">{formatTimestamp(event.lastTimestamp)}</span>
          <button onClick={() => setOpen(!open)} class="shrink-0 rounded p-0.5 text-gray-400 hover:text-flux-blue" title="Details" aria-label="Toggle event details"><Chevron open={open} /></button>
        </div>
        {/* Mobile-only second line: colored kind pill + severity word + timestamp. */}
        <div class="sm:hidden mt-1 flex items-center gap-2 text-xs text-gray-400 dark:text-gray-500">
          <KindChip kind={kind} colorClass={chipColor} title={chipTitle} />
          <span class="whitespace-nowrap"><span class="capitalize">{severity}</span> {formatTimestamp(event.lastTimestamp)}</span>
        </div>
      </div>
      {/* Events carry the full message in the list, so there is nothing to fetch:
          the panel reveals instantly (no spinner) with the same animation. */}
      <Reveal open={open}>
        <div class="px-3 pt-1 pb-4">
          {/* A long event message is capped at 60vh and scrolls inside, matching
              the resource/workload detail panels. */}
          <div class="bg-gray-100 dark:bg-gray-900/70 rounded-md px-3 py-2 text-xs text-gray-700 dark:text-gray-300 max-h-[60vh] overflow-y-auto">
            <pre class="whitespace-pre-wrap break-words font-sans leading-relaxed">{event.message}</pre>
          </div>
        </div>
      </Reveal>
    </div>
  )
}

/**
 * EventList component - Displays and filters Kubernetes events for Flux resources
 *
 * Features:
 * - Fetches events from the API with optional filters (kind, name, namespace, severity)
 * - Auto-refetches when filter signals change
 * - Displays events as compact, expandable rows with the full message inline
 * - Shows loading, error, and empty states
 */
export function EventList() {
  usePageMeta('Events', 'Events dashboard')

  // Restore filter signals from URL query params on mount
  useRestoreFiltersFromUrl({
    kind: selectedEventKind,
    name: selectedEventName,
    namespace: selectedEventNamespace,
    type: selectedEventSeverity
  })

  // Sync filter signals to URL query params on change (debounced)
  useSyncFiltersToUrl({
    kind: selectedEventKind,
    name: selectedEventName,
    namespace: selectedEventNamespace,
    type: selectedEventSeverity
  })

  // Fetch events on mount and when filters change
  useEffect(() => {
    fetchEvents()
  }, [selectedEventKind.value, selectedEventName.value, selectedEventNamespace.value, selectedEventSeverity.value])

  // Infinite scroll hook - reset when filters change or data refetches
  const { visibleCount, sentinelRef, hasMore, loadMore } = useInfiniteScroll({
    totalItems: eventsData.value.length,
    pageSize: 100,
    deps: [selectedEventKind.value, selectedEventName.value, selectedEventNamespace.value, selectedEventSeverity.value, eventsData.value.length]
  })

  // Get visible events (slice the array)
  const visibleEvents = eventsData.value.slice(0, visibleCount)

  const handleClearFilters = () => {
    selectedEventKind.value = ''
    selectedEventName.value = ''
    selectedEventNamespace.value = ''
    selectedEventSeverity.value = ''
  }

  return (
    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-6 pb-8 flex-grow w-full">
      <div class="space-y-3">
        {/* Filters + status chart toolbar */}
        <FilterBar
          count={eventsData.value.length}
          label="events"
          loading={eventsLoading.value}
          statusChart={
            <StatusChart
              items={eventsData.value}
              loading={eventsLoading.value}
              mode="events"
              compact
              onBarClick={(status) => {
                selectedEventSeverity.value = selectedEventSeverity.value === status ? '' : status
              }}
            />
          }
        >
          <FilterForm
            kindSignal={selectedEventKind}
            nameSignal={selectedEventName}
            namespaceSignal={selectedEventNamespace}
            severitySignal={selectedEventSeverity}
            namespaces={reportData.value?.spec?.namespaces || []}
            onClear={handleClearFilters}
            onRefresh={fetchEvents}
          />
        </FilterBar>

        {/* Error State */}
        {eventsError.value && (
          <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
            <div class="flex">
              <svg class="w-5 h-5 text-red-400 dark:text-red-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
              </svg>
              <div class="ml-3">
                <p class="text-sm text-red-800 dark:text-red-200">
                  Failed to load events: {eventsError.value}
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Empty State */}
        {!eventsLoading.value && eventsData.value.length === 0 && (
          <div class="card py-12">
            <div class="text-center">
              <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
              </svg>
              <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
                No events found for the selected filters
              </p>
            </div>
          </div>
        )}

        {/* Events List — rows provide their own padding, so drop the card's
            heavy padding on mobile (keep it on desktop). */}
        {!eventsLoading.value && eventsData.value.length > 0 && (
          <div class="card overflow-hidden p-0 sm:p-2">
            {visibleEvents.map((event, index) => (
              <EventRow key={`${event.involvedObject}-${event.lastTimestamp}-${index}`} event={event} />
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
                  Load more events ({eventsData.value.length - visibleCount} remaining)
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </main>
  )
}
