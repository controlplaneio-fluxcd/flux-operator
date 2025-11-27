// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useRef, useEffect } from 'preact/hooks'

/**
 * FavoritesSearch - Search input for filtering favorites with kind:/ns: filter support
 *
 * @param {Object} props
 * @param {Function} props.onFilter - Callback when filter changes: ({ namespace, kind, name }) => void
 * @param {Function} props.onClose - Callback to close search and return to status bar
 * @param {Array} props.namespaces - Available namespaces for suggestions
 * @param {Array} props.kinds - Available kinds for suggestions
 */
export function FavoritesSearch({ onFilter, onClose, namespaces = [], kinds = [] }) {
  const inputRef = useRef(null)

  // Core state - separate from the query string
  const [selectedNamespace, setSelectedNamespace] = useState(null)
  const [selectedKind, setSelectedKind] = useState(null)
  const [inputValue, setInputValue] = useState('')
  // Track order of filter addition for backspace removal (LIFO)
  const [filterOrder, setFilterOrder] = useState([])

  // Selection indices for keyboard navigation
  const [nsSelectedIndex, setNsSelectedIndex] = useState(-1)
  const [kindSelectedIndex, setKindSelectedIndex] = useState(-1)

  // Derive mode from input value
  const lowerInput = inputValue.toLowerCase()
  const isSelectingNamespace = lowerInput.startsWith('ns:')
  const isSelectingKind = lowerInput.startsWith('kind:')
  const namespacePartial = isSelectingNamespace ? inputValue.slice(3) : ''
  const kindPartial = isSelectingKind ? inputValue.slice(5) : ''

  // Get filtered suggestions
  const namespaceSuggestions = isSelectingNamespace
    ? (namespacePartial
      ? namespaces.filter(ns => ns.toLowerCase().includes(namespacePartial.toLowerCase()))
      : namespaces
    ).slice(0, 10)
    : []

  const kindSuggestions = isSelectingKind
    ? (kindPartial
      ? kinds.filter(k => k.toLowerCase().includes(kindPartial.toLowerCase()))
      : kinds
    ).slice(0, 10)
    : []

  // Focus input on mount
  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.focus()
    }
  }, [])

  // Reset indices when suggestions change
  useEffect(() => {
    setNsSelectedIndex(-1)
  }, [namespaceSuggestions.length, isSelectingNamespace])

  useEffect(() => {
    setKindSelectedIndex(-1)
  }, [kindSuggestions.length, isSelectingKind])

  // Emit filter changes
  useEffect(() => {
    const searchName = (!isSelectingNamespace && !isSelectingKind) ? inputValue : ''
    onFilter({
      namespace: selectedNamespace,
      kind: selectedKind,
      name: searchName
    })
  }, [selectedNamespace, selectedKind, inputValue, isSelectingNamespace, isSelectingKind, onFilter])

  const handleNamespaceSelect = (namespace) => {
    setSelectedNamespace(namespace)
    setInputValue('')
    setFilterOrder(prev => [...prev.filter(f => f !== 'namespace'), 'namespace'])
    setNsSelectedIndex(-1)
    if (inputRef.current) {
      inputRef.current.focus()
    }
  }

  const handleKindSelect = (kind) => {
    setSelectedKind(kind)
    setInputValue('')
    setFilterOrder(prev => [...prev.filter(f => f !== 'kind'), 'kind'])
    setKindSelectedIndex(-1)
    if (inputRef.current) {
      inputRef.current.focus()
    }
  }

  const handleInputChange = (e) => {
    setInputValue(e.target.value)
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Escape') {
      onClose()
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
    }
  }

  // Determine what to show
  const showNamespaceSuggestions = isSelectingNamespace
  const showKindSuggestions = isSelectingKind

  return (
    <div class="relative flex-1">
      {/* Search Input Row */}
      <div class="flex items-center px-3 h-[38px] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md">
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
          placeholder={(selectedNamespace || selectedKind) ? 'Search...' : 'Search... (filter with ns: or kind:)'}
          class="flex-1 min-w-0 text-sm text-gray-900 dark:text-gray-100 bg-transparent placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none"
        />

        <button
          onClick={onClose}
          class="ml-2 p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 focus:outline-none flex-shrink-0"
          aria-label="Close search"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      {/* Namespace Suggestions */}
      {showNamespaceSuggestions && namespaceSuggestions.length > 0 && (
        <div class="absolute left-0 right-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg max-h-60 overflow-y-auto z-50">
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
      {showNamespaceSuggestions && namespaceSuggestions.length === 0 && namespacePartial && (
        <div class="absolute left-0 right-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg z-50">
          <div class="p-3 text-sm text-gray-500 dark:text-gray-400 text-center">
            No matching namespaces
          </div>
        </div>
      )}

      {/* Kind Suggestions */}
      {showKindSuggestions && kindSuggestions.length > 0 && (
        <div class="absolute left-0 right-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg max-h-60 overflow-y-auto z-50">
          <div class="px-3 py-1.5 text-xs text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
            Select kind
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
        </div>
      )}

      {/* Empty kind suggestions */}
      {showKindSuggestions && kindSuggestions.length === 0 && kindPartial && (
        <div class="absolute left-0 right-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg z-50">
          <div class="p-3 text-sm text-gray-500 dark:text-gray-400 text-center">
            No matching kinds
          </div>
        </div>
      )}
    </div>
  )
}
