// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { EventList } from './EventList'
import { ResourceList } from './ResourceList'
import { activeSearchTab } from '../app'

/**
 * SearchView component - Tabbed view for searching Events and Resources
 *
 * Provides a tab interface to switch between EventList and ResourceList views.
 * The active tab state is managed by the activeSearchTab signal.
 */
export function SearchView() {
  return (
    <div class="flex flex-col h-full">
      {/* Tabs */}
      <div class="border-b border-gray-200 dark:border-gray-700">
        <nav class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 -mb-px flex space-x-8" aria-label="Tabs">
          <button
            onClick={() => activeSearchTab.value = 'events'}
            class={`${
              activeSearchTab.value === 'events'
                ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
            } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm transition-colors`}
          >
            Events
          </button>
          <button
            onClick={() => activeSearchTab.value = 'resources'}
            class={`${
              activeSearchTab.value === 'resources'
                ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
            } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm transition-colors`}
          >
            Resources
          </button>
        </nav>
      </div>

      {/* Tab Content */}
      <div class="flex-grow">
        {activeSearchTab.value === 'events' ? (
          <EventList />
        ) : (
          <ResourceList />
        )}
      </div>
    </div>
  )
}
