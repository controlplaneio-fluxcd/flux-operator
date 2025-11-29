// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal } from '@preact/signals'
import { useEffect, useRef, useState } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../utils/fetch'
import { reportData } from '../../app'
import { fluxKinds } from '../../utils/constants'
import { userMenuOpen } from '../layout/UserMenu'

// QuickSearch state signals
export const quickSearchOpen = signal(false)
export const quickSearchQuery = signal('')
export const quickSearchResults = signal([])
export const quickSearchLoading = signal(false)

// Debounce timer reference
let debounceTimer = null

/**
 * Parse search query to extract namespace/kind filters and search term
 * Supports: ns:<namespace>, kind:<kind>, or both
 * @param {string} query - Raw search query
 * @returns {{ namespace: string|null, kind: string|null, name: string, isSelectingNamespace: boolean, isSelectingKind: boolean, namespacePartial: string, kindPartial: string }}
 */
export function parseSearchQuery(query) {
  const result = {
    namespace: null,
    kind: null,
    name: '',
    isSelectingNamespace: false,
    isSelectingKind: false,
    namespacePartial: '',
    kindPartial: ''
  }

  if (!query) {
    return result
  }

  const lowerQuery = query.toLowerCase()

  // Check if typing ns: prefix (no space after value = still selecting)
  if (lowerQuery.startsWith('ns:') && !query.includes(' ')) {
    result.isSelectingNamespace = true
    result.namespacePartial = query.slice(3)
    return result
  }

  // Check if typing kind: prefix (no space after value = still selecting)
  if (lowerQuery.startsWith('kind:') && !query.includes(' ')) {
    result.isSelectingKind = true
    result.kindPartial = query.slice(5)
    return result
  }

  // Extract completed filters (must have space after value)
  let remaining = query

  // Extract completed namespace filter
  const nsRegex = /ns:([^\s]+)\s/gi
  const nsMatch = nsRegex.exec(query)
  if (nsMatch) {
    result.namespace = nsMatch[1]
    remaining = remaining.replace(nsMatch[0], '')
  }

  // Extract completed kind filter
  const kindRegex = /kind:([^\s]+)\s/gi
  const kindMatch = kindRegex.exec(query)
  if (kindMatch) {
    result.kind = kindMatch[1]
    remaining = remaining.replace(kindMatch[0], '')
  }

  // Check if remaining text is a partial filter
  const remainingTrimmed = remaining.trim()
  const remainingLower = remainingTrimmed.toLowerCase()

  if (remainingLower.startsWith('ns:')) {
    result.isSelectingNamespace = true
    result.namespacePartial = remainingTrimmed.slice(3)
    return result
  }

  if (remainingLower.startsWith('kind:')) {
    result.isSelectingKind = true
    result.kindPartial = remainingTrimmed.slice(5)
    return result
  }

  result.name = remainingTrimmed
  return result
}

/**
 * Fetch search results from API
 */
async function fetchSearchResults(name, namespace, kind) {
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
    if (kind) {
      endpoint += `&kind=${encodeURIComponent(kind)}`
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
 */
function debouncedSearch(name, namespace, kind) {
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
    fetchSearchResults(name, namespace, kind)
  }, 400)
}

/**
 * Get filtered namespace suggestions
 */
function getNamespaceSuggestions(partial) {
  const namespaces = reportData.value?.spec?.namespaces || []
  const filtered = partial
    ? namespaces.filter(ns => ns.toLowerCase().includes(partial.toLowerCase()))
    : namespaces
  return [...filtered].sort().slice(0, 10)
}

/**
 * Get filtered kind suggestions
 */
function getKindSuggestions(partial) {
  const filtered = partial
    ? fluxKinds.filter(k => k.toLowerCase().includes(partial.toLowerCase()))
    : fluxKinds
  return filtered
}

/**
 * Get status dot color
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
 * QuickSearch component with state management
 *
 * Architecture:
 * - selectedNamespace/selectedKind: store the selected filters
 * - inputValue: what's shown in the input field
 * - mode: derived from inputValue (search, selectingNamespace, selectingKind)
 */
export function QuickSearch() {
  const location = useLocation()
  const inputRef = useRef(null)

  // Core state - separate from the query string
  const [selectedNamespace, setSelectedNamespace] = useState(null)
  const [selectedKind, setSelectedKind] = useState(null)
  const [inputValue, setInputValue] = useState('')
  // Track order of filter addition for backspace removal (LIFO)
  const [filterOrder, setFilterOrder] = useState([]) // ['namespace'] or ['kind'] or ['namespace', 'kind'] etc.

  // Selection indices for keyboard navigation
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const [nsSelectedIndex, setNsSelectedIndex] = useState(-1)
  const [kindSelectedIndex, setKindSelectedIndex] = useState(-1)

  // Derive mode from input value
  const lowerInput = inputValue.toLowerCase()
  const isSelectingNamespace = lowerInput.startsWith('ns:')
  const isSelectingKind = lowerInput.startsWith('kind:')
  const namespacePartial = isSelectingNamespace ? inputValue.slice(3) : ''
  const kindPartial = isSelectingKind ? inputValue.slice(5) : ''

  // Check if user is typing a filter prefix (don't search yet)
  const isTypingFilterPrefix = !selectedNamespace && !selectedKind &&
    (lowerInput === 'ns' || lowerInput === 'kind' || lowerInput === 'ns:' || lowerInput === 'kind:' ||
     lowerInput.startsWith('kind') && lowerInput.length < 5)

  // Get suggestions
  const namespaceSuggestions = isSelectingNamespace ? getNamespaceSuggestions(namespacePartial) : []
  const kindSuggestions = isSelectingKind ? getKindSuggestions(kindPartial) : []

  // Sync internal state to exported signal (used by tests)
  useEffect(() => {
    const parts = []
    if (selectedNamespace) parts.push(`ns:${selectedNamespace} `)
    if (selectedKind) parts.push(`kind:${selectedKind} `)
    parts.push(inputValue)
    quickSearchQuery.value = parts.join('')
  }, [selectedNamespace, selectedKind, inputValue])

  // Focus input when search opens
  useEffect(() => {
    if (quickSearchOpen.value && inputRef.current) {
      inputRef.current.focus()
    }
  }, [quickSearchOpen.value])

  // Global "/" keyboard shortcut to open search
  useEffect(() => {
    const handleGlobalKeyDown = (e) => {
      // Don't trigger if already in an input or textarea
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
        return
      }
      if (e.key === '/' && !quickSearchOpen.value) {
        e.preventDefault()
        quickSearchOpen.value = true
      }
    }

    document.addEventListener('keydown', handleGlobalKeyDown)
    return () => document.removeEventListener('keydown', handleGlobalKeyDown)
  }, [])

  // Close search when navigating to another page
  const prevPathRef = useRef(location.path)
  useEffect(() => {
    if (prevPathRef.current !== location.path) {
      prevPathRef.current = location.path
      if (quickSearchOpen.value) {
        handleClose()
      }
    }
  }, [location.path])

  // Close search when user menu opens
  useEffect(() => {
    if (userMenuOpen.value && quickSearchOpen.value) {
      handleClose()
    }
  }, [userMenuOpen.value])

  // Reset indices when suggestions/results change
  useEffect(() => {
    setSelectedIndex(-1)
  }, [quickSearchResults.value])

  useEffect(() => {
    setNsSelectedIndex(-1)
  }, [namespaceSuggestions.length, isSelectingNamespace])

  useEffect(() => {
    setKindSelectedIndex(-1)
  }, [kindSuggestions.length, isSelectingKind])

  // Trigger search when input changes (and not selecting filters)
  useEffect(() => {
    if (!isSelectingNamespace && !isSelectingKind && !isTypingFilterPrefix && inputValue.length >= 2) {
      debouncedSearch(inputValue, selectedNamespace, selectedKind)
    } else if (!isSelectingNamespace && !isSelectingKind && !isTypingFilterPrefix) {
      quickSearchResults.value = []
      quickSearchLoading.value = false
    }
  }, [inputValue, selectedNamespace, selectedKind, isSelectingNamespace, isSelectingKind, isTypingFilterPrefix])

  const handleSearchClick = () => {
    userMenuOpen.value = false
    quickSearchOpen.value = true
  }

  const handleClose = () => {
    quickSearchOpen.value = false
    quickSearchQuery.value = ''
    quickSearchResults.value = []
    quickSearchLoading.value = false
    setSelectedNamespace(null)
    setSelectedKind(null)
    setInputValue('')
    setFilterOrder([])
    setSelectedIndex(-1)
    setNsSelectedIndex(-1)
    setKindSelectedIndex(-1)
    if (debounceTimer) {
      window.clearTimeout(debounceTimer)
    }
  }

  const handleNamespaceSelect = (namespace) => {
    setSelectedNamespace(namespace)
    setInputValue('')
    setFilterOrder(prev => [...prev.filter(f => f !== 'namespace'), 'namespace'])
    setNsSelectedIndex(-1)
    quickSearchResults.value = []
    if (inputRef.current) {
      inputRef.current.focus()
    }
  }

  const handleKindSelect = (kind) => {
    setSelectedKind(kind)
    setInputValue('')
    setFilterOrder(prev => [...prev.filter(f => f !== 'kind'), 'kind'])
    setKindSelectedIndex(-1)
    quickSearchResults.value = []
    if (inputRef.current) {
      inputRef.current.focus()
    }
  }

  const handleInputChange = (e) => {
    setInputValue(e.target.value)
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Escape') {
      handleClose()
      return
    }

    // Handle backspace to remove badges when input is empty (LIFO order)
    if (e.key === 'Backspace' && inputValue === '' && (selectedNamespace || selectedKind)) {
      e.preventDefault()
      // Remove the most recently added filter
      const lastFilter = filterOrder[filterOrder.length - 1]
      if (lastFilter === 'kind') {
        setSelectedKind(null)
        setFilterOrder(prev => prev.slice(0, -1))
      } else if (lastFilter === 'namespace') {
        setSelectedNamespace(null)
        setFilterOrder(prev => prev.slice(0, -1))
      }
      quickSearchResults.value = []
      return
    }

    // Handle namespace suggestions navigation
    if (isSelectingNamespace && namespaceSuggestions.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setNsSelectedIndex(prev => Math.min(prev + 1, namespaceSuggestions.length - 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setNsSelectedIndex(prev => Math.max(prev - 1, -1))
      } else if (e.key === 'Enter' && nsSelectedIndex >= 0) {
        e.preventDefault()
        handleNamespaceSelect(namespaceSuggestions[nsSelectedIndex])
      }
      return
    }

    // Handle kind suggestions navigation
    if (isSelectingKind && kindSuggestions.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setKindSelectedIndex(prev => Math.min(prev + 1, kindSuggestions.length - 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setKindSelectedIndex(prev => Math.max(prev - 1, -1))
      } else if (e.key === 'Enter' && kindSelectedIndex >= 0) {
        e.preventDefault()
        handleKindSelect(kindSuggestions[kindSelectedIndex])
      }
      return
    }

    // Handle search results navigation
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      const maxIndex = quickSearchResults.value.length - 1
      if (maxIndex >= 0) {
        setSelectedIndex(prev => Math.min(prev + 1, maxIndex))
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIndex(prev => Math.max(prev - 1, -1))
    } else if (e.key === 'Enter' && selectedIndex >= 0) {
      e.preventDefault()
      const resource = quickSearchResults.value[selectedIndex]
      if (resource) {
        handleResultClick(resource)
      }
    }
  }

  const handleResultClick = (resource) => {
    location.route(`/resource/${encodeURIComponent(resource.kind)}/${encodeURIComponent(resource.namespace)}/${encodeURIComponent(resource.name)}`)
    handleClose()
  }

  // Determine what to show in the panel - mutually exclusive states
  // Priority: namespace suggestions > kind suggestions > loading > results > empty > hint
  const panelState = (() => {
    if (isSelectingNamespace) return 'namespace'
    if (isSelectingKind) return 'kind'
    if (quickSearchLoading.value) return 'loading'
    if (quickSearchResults.value.length > 0) return 'results'
    // Show empty only if we actually searched (2+ chars, not typing filter prefix)
    if (inputValue.length >= 2 && !isTypingFilterPrefix) return 'empty'
    // Default: show hint
    return 'hint'
  })()

  return (
    <div class="relative">
      {/* Search Button - Icon only on mobile, textbox style on desktop */}
      {!quickSearchOpen.value && (
        <>
          {/* Mobile: Icon button */}
          <button
            onClick={handleSearchClick}
            title="Search (press /)"
            class="sm:hidden inline-flex items-center justify-center p-1.5 border border-gray-300 dark:border-gray-600 rounded-md text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue"
            aria-label="Open search"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </button>
          {/* Desktop: Textbox style button */}
          <button
            onClick={handleSearchClick}
            title="Search (press /)"
            class="hidden sm:inline-flex items-center gap-2 px-2.5 py-1 border border-gray-300 dark:border-gray-600 rounded-md text-gray-500 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 hover:border-gray-400 dark:hover:border-gray-500 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue"
            aria-label="Open search"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <span class="text-sm">Search</span>
            <kbd class="px-1.5 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded">/</kbd>
          </button>
        </>
      )}

      {/* Search Panel */}
      {quickSearchOpen.value && (
        <div class="animate-slide-in-right">
          {/* Search Input Row - inline in header */}
          <div class="flex items-center h-[30px] px-2 border border-gray-300 dark:border-gray-600 rounded-md">
            <svg class="w-4 h-4 text-gray-400 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>

            {/* Filter Badges - rendered in order of addition */}
            {filterOrder.map(filter => {
              if (filter === 'namespace' && selectedNamespace) {
                return (
                  <span key="ns" class="inline-flex items-center px-2 py-0.5 mr-1.5 rounded text-xs font-medium bg-blue-100 dark:bg-blue-900/50 text-blue-800 dark:text-blue-200 flex-shrink-0">
                    ns:{selectedNamespace}
                  </span>
                )
              }
              if (filter === 'kind' && selectedKind) {
                return (
                  <span key="kind" class="inline-flex items-center px-2 py-0.5 mr-1.5 rounded text-xs font-medium bg-green-100 dark:bg-green-900/50 text-green-800 dark:text-green-200 flex-shrink-0">
                    kind:{selectedKind}
                  </span>
                )
              }
              return null
            })}

            <input
              ref={inputRef}
              type="text"
              value={inputValue}
              onInput={handleInputChange}
              onKeyDown={handleKeyDown}
              placeholder={(selectedNamespace || selectedKind) ? 'Search...' : 'Search appliers...'}
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

          {/* Dropdown Panel */}
          <div class="absolute left-0 right-0 mt-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg max-h-80 overflow-y-auto z-50">
            {/* Namespace Suggestions */}
            {panelState === 'namespace' && (
              namespaceSuggestions.length > 0 ? (
                <>
                  <div class="px-3 py-1.5 text-xs text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                    Type or select namespace
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
                </>
              ) : (
                <div class="p-3 text-sm text-gray-500 dark:text-gray-400">
                  {namespacePartial ? 'No matching namespaces' : 'Type to filter namespaces'}
                </div>
              )
            )}

            {/* Kind Suggestions */}
            {panelState === 'kind' && (
              kindSuggestions.length > 0 ? (
                <>
                  <div class="px-3 py-1.5 text-xs text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                    Type or select kind
                  </div>
                  <ul>
                    {kindSuggestions.map((kind, index) => (
                      <li key={kind}>
                        <button
                          onClick={() => handleKindSelect(kind)}
                          class={`w-full text-left py-1.5 px-3 text-sm font-mono focus:outline-none transition-colors ${
                            index === kindSelectedIndex
                              ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-gray-100'
                              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                          }`}
                        >
                          {kind}
                        </button>
                      </li>
                    ))}
                  </ul>
                </>
              ) : (
                <div class="p-3 text-sm text-gray-500 dark:text-gray-400">
                  {kindPartial ? 'No matching kinds' : 'Type to filter kinds'}
                </div>
              )
            )}

            {/* Loading State */}
            {panelState === 'loading' && (
              <div class="p-3 text-sm text-gray-500 dark:text-gray-400 text-center">
                Searching...
              </div>
            )}

            {/* Results List */}
            {panelState === 'results' && (
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
                        <span class="text-sm font-mono text-gray-900 dark:text-gray-100 break-all">
                          <span class="text-gray-500 dark:text-gray-400">{resource.kind}/</span>{resource.namespace}/{resource.name}
                        </span>
                      </div>
                    </button>
                  </li>
                ))}
              </ul>
            )}

            {/* Empty State */}
            {panelState === 'empty' && (
              <div class="p-3 text-sm text-gray-500 dark:text-gray-400">
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

            {/* Search hint */}
            {panelState === 'hint' && (
              <div class="px-3 py-2 text-sm text-gray-500 dark:text-gray-400 space-y-1">
                <div>Type 2+ chars to search or <span class="font-mono">**</span> for most recent</div>
                <div>
                  Apply filters with{' '}
                  <button
                    onClick={() => {
                      setInputValue('ns:')
                      if (inputRef.current) {
                        inputRef.current.focus()
                      }
                    }}
                    class="text-flux-blue hover:underline focus:outline-none"
                  >
                    ns:
                  </button>
                  {' '}and{' '}
                  <button
                    onClick={() => {
                      setInputValue('kind:')
                      if (inputRef.current) {
                        inputRef.current.focus()
                      }
                    }}
                    class="text-flux-blue hover:underline focus:outline-none"
                  >
                    kind:
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
