// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect, useRef, useState } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../utils/fetch'
import { reportData } from '../../app'

// QuickSearch state signals
export const quickSearchOpen = signal(false)
export const quickSearchQuery = signal('')
export const quickSearchResults = signal([])
export const quickSearchLoading = signal(false)

// Debounce timer reference
let debounceTimer = null

/**
 * Parse search query to extract namespace filter and search term
 * @param {string} query - Raw search query
 * @returns {{ namespace: string|null, name: string, isSelectingNamespace: boolean, namespacePartial: string }}
 */
export function parseSearchQuery(query) {
  if (!query || !query.toLowerCase().startsWith('ns:')) {
    return { namespace: null, name: query || '', isSelectingNamespace: false, namespacePartial: '' }
  }

  // Check if there's a space after ns: prefix (namespace selection complete)
  const spaceIndex = query.indexOf(' ')
  if (spaceIndex === -1) {
    // Still selecting namespace (e.g., "ns:" or "ns:flux")
    return {
      namespace: null,
      name: '',
      isSelectingNamespace: true,
      namespacePartial: query.slice(3) // Everything after "ns:"
    }
  }

  // Namespace selected, extract namespace and search term
  const namespace = query.slice(3, spaceIndex)
  const name = query.slice(spaceIndex + 1)
  return {
    namespace: namespace || null,
    name,
    isSelectingNamespace: false,
    namespacePartial: ''
  }
}

/**
 * Fetch search results from API with debouncing
 * @param {string} name - Search term
 * @param {string|null} namespace - Optional namespace filter
 */
async function fetchSearchResults(name, namespace) {
  if (!name || name.length < 2) {
    quickSearchResults.value = []
    quickSearchLoading.value = false
    return
  }

  quickSearchLoading.value = true

  try {
    let endpoint = `/api/v1/search?name=${encodeURIComponent(name)}`
    if (namespace) {
      endpoint += `&namespace=${encodeURIComponent(namespace)}`
    }
    const data = await fetchWithMock({
      endpoint,
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
 * @param {string} name - Search term
 * @param {string|null} namespace - Optional namespace filter
 */
function debouncedSearch(name, namespace) {
  if (debounceTimer) {
    window.clearTimeout(debounceTimer)
  }

  if (!name || name.length < 2) {
    quickSearchResults.value = []
    quickSearchLoading.value = false
    return
  }

  quickSearchLoading.value = true
  debounceTimer = setTimeout(() => {
    fetchSearchResults(name, namespace)
  }, 300)
}

/**
 * Get filtered namespace suggestions based on partial input
 * @param {string} partial - Partial namespace string to filter by
 * @returns {string[]} - Filtered and sorted namespace suggestions (max 10)
 */
function getNamespaceSuggestions(partial) {
  const namespaces = reportData.value?.spec?.namespaces || []
  const filtered = partial
    ? namespaces.filter(ns => ns.toLowerCase().includes(partial.toLowerCase()))
    : namespaces
  return [...filtered].sort().slice(0, 10)
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
  const [nsSelectedIndex, setNsSelectedIndex] = useState(-1)

  // Parse the current query
  const parsed = parseSearchQuery(quickSearchQuery.value)
  const namespaceSuggestions = parsed.isSelectingNamespace
    ? getNamespaceSuggestions(parsed.namespacePartial)
    : []

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

  // Reset namespace selection index when suggestions change
  useEffect(() => {
    setNsSelectedIndex(-1)
  }, [namespaceSuggestions.length, parsed.isSelectingNamespace])

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
    setSelectedIndex(-1)
    setNsSelectedIndex(-1)
    if (debounceTimer) {
      window.clearTimeout(debounceTimer)
    }
  }

  // Handle input change
  const handleInputChange = (e) => {
    const value = e.target.value
    quickSearchQuery.value = value

    // Parse the query and trigger search if we have a complete namespace + name
    const { namespace, name, isSelectingNamespace } = parseSearchQuery(value)

    // Don't search if user is typing "ns" - they're likely about to type "ns:"
    const isTypingNsPrefix = !namespace && value.toLowerCase().startsWith('ns')

    if (!isSelectingNamespace && name && !isTypingNsPrefix) {
      debouncedSearch(name, namespace)
    } else if (!isSelectingNamespace && !name) {
      // Clear results when no name is entered yet
      quickSearchResults.value = []
      quickSearchLoading.value = false
    } else if (isTypingNsPrefix && !isSelectingNamespace) {
      // Clear results when typing "ns" prefix
      quickSearchResults.value = []
      quickSearchLoading.value = false
    }
  }

  // Handle namespace selection
  const handleNamespaceSelect = (namespace) => {
    quickSearchQuery.value = `ns:${namespace} `
    quickSearchResults.value = []
    setNsSelectedIndex(-1)
    // Focus input after selection
    if (inputRef.current) {
      inputRef.current.focus()
    }
  }

  // Handle keyboard events
  const handleKeyDown = (e) => {
    if (e.key === 'Escape') {
      handleClose()
      return
    }

    // Handle namespace suggestions navigation
    if (parsed.isSelectingNamespace && namespaceSuggestions.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        const maxIndex = namespaceSuggestions.length - 1
        setNsSelectedIndex(prev => prev < maxIndex ? prev + 1 : prev)
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setNsSelectedIndex(prev => prev > 0 ? prev - 1 : -1)
      } else if (e.key === 'Enter' && nsSelectedIndex >= 0) {
        e.preventDefault()
        handleNamespaceSelect(namespaceSuggestions[nsSelectedIndex])
      }
      return
    }

    // Handle search results navigation
    if (e.key === 'ArrowDown') {
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

  // Check if namespace suggestions panel should be shown
  const showNamespaceSuggestions = quickSearchOpen.value && parsed.isSelectingNamespace

  // Don't show results panel if user is typing "ns" prefix (likely about to type "ns:")
  const isTypingNsPrefix = !parsed.namespace && quickSearchQuery.value.toLowerCase().startsWith('ns')

  // Show hint when typing 1-2 chars (without namespace selected)
  const showSearchHint = quickSearchOpen.value &&
    !parsed.namespace &&
    !parsed.isSelectingNamespace &&
    !isTypingNsPrefix &&
    quickSearchQuery.value.length >= 1 &&
    quickSearchQuery.value.length < 2

  // Check if results panel should be shown
  const showResultsPanel = quickSearchOpen.value && !parsed.isSelectingNamespace && !isTypingNsPrefix && (
    quickSearchResults.value.length > 0 ||
    quickSearchLoading.value ||
    (parsed.name.length >= 2 && !quickSearchLoading.value)
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
            {/* Namespace Badge - shown when namespace is selected */}
            {parsed.namespace && (
              <span class="inline-flex items-center px-2 py-0.5 mr-1.5 rounded text-xs font-medium bg-blue-100 dark:bg-blue-900/50 text-blue-800 dark:text-blue-200 flex-shrink-0">
                ns:{parsed.namespace}
              </span>
            )}
            <input
              ref={inputRef}
              type="text"
              value={parsed.namespace ? parsed.name : quickSearchQuery.value}
              onInput={(e) => {
                // When namespace is selected, prepend it to the input value
                if (parsed.namespace) {
                  quickSearchQuery.value = `ns:${parsed.namespace} ${e.target.value}`
                  const { name, namespace } = parseSearchQuery(quickSearchQuery.value)
                  if (name) {
                    debouncedSearch(name, namespace)
                  } else {
                    quickSearchResults.value = []
                    quickSearchLoading.value = false
                  }
                } else {
                  handleInputChange(e)
                }
              }}
              onKeyDown={(e) => {
                // Handle backspace to remove namespace badge when input is empty
                if (e.key === 'Backspace' && parsed.namespace && !parsed.name) {
                  e.preventDefault()
                  quickSearchQuery.value = ''
                  quickSearchResults.value = []
                  return
                }
                handleKeyDown(e)
              }}
              placeholder={parsed.namespace ? 'Search in namespace...' : 'Search appliers...'}
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

          {/* Namespace Suggestions - shown when typing ns: */}
          {showNamespaceSuggestions && namespaceSuggestions.length > 0 && (
            <div class="absolute left-0 right-0 mt-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg max-h-80 overflow-y-auto z-50">
              <div class="px-3 py-1.5 text-xs text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                Select namespace
              </div>
              <ul>
                {namespaceSuggestions.map((ns, index) => (
                  <li key={ns}>
                    <button
                      onClick={() => handleNamespaceSelect(ns)}
                      class={`w-full text-left py-1.5 px-3 text-sm font-mono focus:outline-none transition-colors ${
                        index === nsSelectedIndex
                          ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-gray-100'
                          : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                      }`}
                    >
                      {ns}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Empty namespace suggestions */}
          {showNamespaceSuggestions && namespaceSuggestions.length === 0 && parsed.namespacePartial && (
            <div class="absolute left-0 right-0 mt-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg z-50">
              <div class="p-3 text-sm text-gray-500 dark:text-gray-400 text-center">
                No matching namespaces
              </div>
            </div>
          )}

          {/* Search hint - shown when typing 1 char */}
          {showSearchHint && (
            <div class="absolute left-0 right-0 mt-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg z-50">
              <div class="p-3 text-sm text-gray-500 dark:text-gray-400">
                <p>Type at least 2 characters to search</p>
                <p class="mt-1">
                  <span class="text-gray-400 dark:text-gray-500">Tip:</span>{' '}
                  <button
                    onClick={() => {
                      quickSearchQuery.value = 'ns:'
                      if (inputRef.current) {
                        inputRef.current.focus()
                      }
                    }}
                    class="text-flux-blue hover:underline focus:outline-none"
                  >
                    ns:
                  </button>
                  {' '}to filter by namespace
                </p>
              </div>
            </div>
          )}

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
              {!quickSearchLoading.value && quickSearchResults.value.length === 0 && parsed.name.length >= 2 && (
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
