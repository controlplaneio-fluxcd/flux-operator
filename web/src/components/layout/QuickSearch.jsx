// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect, useRef, useState } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../utils/fetch'

// QuickSearch state signals
export const quickSearchOpen = signal(false)
export const quickSearchQuery = signal('')
export const quickSearchResults = signal([])
export const quickSearchLoading = signal(false)

// Debounce timer reference
let debounceTimer = null

/**
 * Fetch search results from API with debouncing
 * @param {string} query - Search term
 */
async function fetchSearchResults(query) {
  if (!query || query.length < 2) {
    quickSearchResults.value = []
    quickSearchLoading.value = false
    return
  }

  quickSearchLoading.value = true

  try {
    const data = await fetchWithMock({
      endpoint: `/api/v1/search?name=${encodeURIComponent(query)}`,
      mockPath: '../mock/resources',
      mockExport: 'getMockSearchResults'
    })
    quickSearchResults.value = data.resources || []
  } catch (error) {
    console.error('Failed to fetch search results:', error)
    quickSearchResults.value = []
  } finally {
    quickSearchLoading.value = false
  }
}

/**
 * Debounced search function
 * @param {string} query - Search term
 */
function debouncedSearch(query) {
  if (debounceTimer) {
    window.clearTimeout(debounceTimer)
  }

  if (!query || query.length < 2) {
    quickSearchResults.value = []
    quickSearchLoading.value = false
    return
  }

  quickSearchLoading.value = true
  debounceTimer = setTimeout(() => {
    fetchSearchResults(query)
  }, 300)
}

/**
 * Get status dot color based on resource status
 * @param {string} status - Resource status
 * @returns {string} - Tailwind CSS classes for the dot
 */
function getStatusDotClass(status) {
  switch (status) {
  case 'Ready':
    return 'bg-green-500'
  case 'Failed':
    return 'bg-red-500'
  case 'Progressing':
    return 'bg-blue-500'
  case 'Suspended':
    return 'bg-yellow-500'
  case 'Unknown':
  default:
    return 'bg-gray-500'
  }
}

/**
 * QuickSearch component - Header search with animated input and dropdown results
 *
 * Features:
 * - Search button that toggles the search input
 * - Animated input field that slides in from the right
 * - Close button to clear and dismiss
 * - Escape key to close
 * - Debounced API search (300ms)
 * - Results dropdown with status dots
 * - Click result to navigate to resources page
 */
export function QuickSearch() {
  const location = useLocation()
  const inputRef = useRef(null)
  const [selectedIndex, setSelectedIndex] = useState(-1)

  // Focus input when search opens
  useEffect(() => {
    if (quickSearchOpen.value && inputRef.current) {
      inputRef.current.focus()
    }
  }, [quickSearchOpen.value])

  // Reset selected index when results change
  useEffect(() => {
    setSelectedIndex(-1)
  }, [quickSearchResults.value])

  // Handle search button click
  const handleSearchClick = () => {
    quickSearchOpen.value = true
  }

  // Handle close button click
  const handleClose = () => {
    quickSearchOpen.value = false
    quickSearchQuery.value = ''
    quickSearchResults.value = []
    quickSearchLoading.value = false
    if (debounceTimer) {
      window.clearTimeout(debounceTimer)
    }
  }

  // Handle input change
  const handleInputChange = (e) => {
    const value = e.target.value
    quickSearchQuery.value = value
    debouncedSearch(value)
  }

  // Handle keyboard events
  const handleKeyDown = (e) => {
    if (e.key === 'Escape') {
      handleClose()
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      const maxIndex = quickSearchResults.value.length - 1
      if (maxIndex >= 0) {
        setSelectedIndex(prev => prev < maxIndex ? prev + 1 : prev)
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIndex(prev => prev > 0 ? prev - 1 : -1)
    } else if (e.key === 'Enter' && selectedIndex >= 0) {
      e.preventDefault()
      const resource = quickSearchResults.value[selectedIndex]
      if (resource) {
        handleResultClick(resource)
      }
    }
  }

  // Handle result click - navigate to resource dashboard
  const handleResultClick = (resource) => {
    location.route(`/resource/${encodeURIComponent(resource.kind)}/${encodeURIComponent(resource.namespace)}/${encodeURIComponent(resource.name)}`)
    handleClose()
  }

  // Check if results panel should be shown
  const showResultsPanel = quickSearchOpen.value && (
    quickSearchResults.value.length > 0 ||
    quickSearchLoading.value ||
    (quickSearchQuery.value.length >= 2 && !quickSearchLoading.value)
  )

  return (
    <div class="relative">
      {/* Search Button - shown when search is closed */}
      {!quickSearchOpen.value && (
        <button
          onClick={handleSearchClick}
          title="Search"
          class="inline-flex items-center justify-center p-2 border border-gray-300 dark:border-gray-600 rounded-md text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue"
          aria-label="Open search"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>
      )}

      {/* Search Panel - shown when search is open */}
      {quickSearchOpen.value && (
        <div class="animate-slide-in-right">
          {/* Search Input Row - matches button height */}
          <div class="flex items-center px-3 h-[38px] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg">
            <svg class="w-4 h-4 text-gray-400 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              ref={inputRef}
              type="text"
              value={quickSearchQuery.value}
              onInput={handleInputChange}
              onKeyDown={handleKeyDown}
              placeholder="Search appliers..."
              class="flex-1 min-w-0 text-sm text-gray-900 dark:text-gray-100 bg-transparent placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none"
            />
            <button
              onClick={handleClose}
              class="ml-2 p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 focus:outline-none flex-shrink-0"
              aria-label="Close search"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* Results Area - positioned absolutely below input */}
          {showResultsPanel && (
            <div class="absolute left-0 right-0 mt-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg max-h-80 overflow-y-auto z-50">
              {/* Loading State */}
              {quickSearchLoading.value && (
                <div class="p-3 text-sm text-gray-500 dark:text-gray-400 text-center">
                  Searching...
                </div>
              )}

              {/* Results List */}
              {!quickSearchLoading.value && quickSearchResults.value.length > 0 && (
                <ul>
                  {quickSearchResults.value.map((resource, index) => (
                    <li key={`${resource.kind}-${resource.namespace}-${resource.name}-${index}`}>
                      <button
                        onClick={() => handleResultClick(resource)}
                        class={`w-full text-left py-1 px-2 focus:outline-none transition-colors ${
                          index === selectedIndex
                            ? 'bg-gray-100 dark:bg-gray-700'
                            : 'hover:bg-gray-50 dark:hover:bg-gray-700'
                        }`}
                      >
                        <div class="flex items-center gap-1.5">
                          <span class={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${getStatusDotClass(resource.status)}`} />
                          <span class="text-xs sm:text-sm font-mono text-gray-900 dark:text-gray-100 break-all">
                            <span class="text-gray-500 dark:text-gray-400">{resource.kind}/</span>{resource.namespace}/{resource.name}
                          </span>
                        </div>
                      </button>
                    </li>
                  ))}
                </ul>
              )}

              {/* Empty State */}
              {!quickSearchLoading.value && quickSearchResults.value.length === 0 && quickSearchQuery.value.length >= 2 && (
                <div class="p-3 text-sm text-gray-500 dark:text-gray-400 text-center">
                  <p>No resources found</p>
                  <button
                    onClick={() => {
                      location.route('/resources')
                      handleClose()
                    }}
                    class="mt-2 text-flux-blue hover:underline focus:outline-none"
                  >
                    Browse all resources →
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
