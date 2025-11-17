// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect } from 'preact/hooks'
import { useState } from 'preact/hooks'
import { fetchWithMock } from '../utils/fetch'
import { formatTimestamp } from '../utils/time'
import { fluxReport } from '../app'
import { FilterForm } from './FilterForm'

// Events data signals
export const eventsData = signal([])
export const eventsLoading = signal(false)
export const eventsError = signal(null)

// Filter signals
export const selectedEventsKind = signal('')
export const selectedEventsName = signal('')
export const selectedEventsNamespace = signal('')
export const selectedEventsSeverity = signal('')

// Fetch events from API
export async function fetchEvents() {
  eventsLoading.value = true
  eventsError.value = null

  const params = new URLSearchParams()
  if (selectedEventsKind.value) params.append('kind', selectedEventsKind.value)
  if (selectedEventsName.value) params.append('name', selectedEventsName.value)
  if (selectedEventsNamespace.value) params.append('namespace', selectedEventsNamespace.value)
  if (selectedEventsSeverity.value) params.append('type', selectedEventsSeverity.value)

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

// Get status badge color and styling for events
function getEventStatusBadgeClass(type) {
  switch (type) {
  case 'Normal':
    return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
  case 'Warning':
  default:
    return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
  }
}

/**
 * EventCard - Individual card displaying a Kubernetes event
 *
 * @param {Object} props
 * @param {Object} props.event - Event object with type, message, involvedObject, timestamp
 *
 * Features:
 * - Shows resource kind and name from involvedObject
 * - Displays event severity badge (Info for Normal, Warning for Warning)
 * - Shows event message with expand/collapse for long messages
 * - Displays namespace and timestamp
 */
function EventCard({ event }) {
  const [isExpanded, setIsExpanded] = useState(false)

  // Parse involvedObject to get kind and name
  const [kind, name] = event.involvedObject.split('/')

  // Map event type to display status
  const displayStatus = event.type === 'Normal' ? 'Info' : 'Warning'

  // Check if message is long or contains newlines
  const isLongMessage = event.message.length > 150 || event.message.includes('\n')
  const shouldTruncate = isLongMessage && !isExpanded

  // Truncate to first line or 150 chars
  const getTruncatedMessage = () => {
    const firstLine = event.message.split('\n')[0]
    if (firstLine.length > 150) {
      return firstLine.substring(0, 150) + '...'
    }
    return firstLine
  }

  const displayMessage = shouldTruncate ? getTruncatedMessage() : event.message

  return (
    <div class="card p-4 hover:shadow-md transition-shadow">
      {/* Header row: kind + status badge + timestamp */}
      <div class="flex items-center justify-between mb-3">
        <div class="flex items-center gap-3">
          <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">
            {kind}
          </span>
          <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getEventStatusBadgeClass(event.type)}`}>
            {displayStatus}
          </span>
        </div>
        <span class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap ml-4">
          {formatTimestamp(event.lastTimestamp)}
        </span>
      </div>

      {/* Resource namespace/name */}
      <div class="mb-2">
        <span class="font-mono text-sm text-gray-500 dark:text-gray-400">
          {event.namespace}/
        </span>
        <span class="font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">
          {name}
        </span>
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
    </div>
  )
}

/**
 * EventList component - Displays and filters Kubernetes events for Flux resources
 *
 * Features:
 * - Fetches events from the API with optional filters (kind, name, namespace, severity)
 * - Auto-refetches when filter signals change
 * - Displays events in card format with expandable messages
 * - Shows loading, error, and empty states
 */
export function EventList() {
  // Fetch events on mount and when filters change
  useEffect(() => {
    fetchEvents()
  }, [selectedEventsKind.value, selectedEventsName.value, selectedEventsNamespace.value, selectedEventsSeverity.value])

  const handleClearFilters = () => {
    selectedEventsKind.value = ''
    selectedEventsName.value = ''
    selectedEventsNamespace.value = ''
    selectedEventsSeverity.value = ''
  }

  return (
    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">
        {/* Page Title */}
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-bold text-gray-900 dark:text-white">Flux Events</h2>
          {/* Event count */}
          {!eventsLoading.value && eventsData.value.length > 0 && (
            <span class="text-sm text-gray-600 dark:text-gray-400">
              {eventsData.value.length} events
            </span>
          )}
        </div>

        {/* Filters */}
        <div class="card p-4">
          <FilterForm
            kindSignal={selectedEventsKind}
            nameSignal={selectedEventsName}
            namespaceSignal={selectedEventsNamespace}
            severitySignal={selectedEventsSeverity}
            namespaces={fluxReport.value?.spec?.namespaces || []}
            onClear={handleClearFilters}
          />
        </div>

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

        {/* Events List */}
        {/* Loading State */}
        {eventsLoading.value && (
          <div class="card py-12">
            <div class="flex items-center justify-center">
              <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-flux-blue"></div>
              <span class="ml-3 text-gray-600 dark:text-gray-400">Loading events...</span>
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

        {/* Events Cards */}
        {!eventsLoading.value && eventsData.value.length > 0 && (
          <div class="space-y-4">
            {eventsData.value.map((event, index) => (
              <EventCard key={`${event.involvedObject}-${event.lastTimestamp}-${index}`} event={event} />
            ))}
          </div>
        )}
      </div>
    </main>
  )
}
