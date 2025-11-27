// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo } from 'preact/hooks'
import { FavoritesSearch } from './FavoritesSearch'
import { getStatusBarColor } from '../../utils/status'

/**
 * Calculate status percentages for favorites
 */
function calculateStatusPercentages(items) {
  if (!items || items.length === 0) {
    return []
  }

  const totalCount = items.length
  const statusCounts = {}

  items.forEach(item => {
    const status = item.status || 'Unknown'
    statusCounts[status] = (statusCounts[status] || 0) + 1
  })

  const statusBars = Object.entries(statusCounts).map(([status, count]) => ({
    status,
    count,
    percentage: (count / totalCount) * 100
  }))

  return statusBars.sort((a, b) => b.percentage - a.percentage)
}

/**
 * FavoritesHeader - Header with status bar, edit order button, and search
 *
 * @param {Object} props
 * @param {Array} props.resources - Array of favorite resources with status data
 * @param {boolean} props.loading - Whether data is loading
 * @param {boolean} props.editMode - Whether edit mode is active
 * @param {Function} props.onEditModeToggle - Callback to toggle edit mode
 * @param {Function} props.onSaveOrder - Callback to save new order
 * @param {Function} props.onCancelEdit - Callback to cancel edit mode
 * @param {Function} props.onFilter - Callback when filter changes
 * @param {Function} props.onStatusFilter - Callback when status bar is clicked
 * @param {string} props.statusFilter - Currently active status filter
 * @param {Array} props.namespaces - Available namespaces for search suggestions
 * @param {Array} props.kinds - Available kinds for search suggestions
 */
export function FavoritesHeader({
  resources,
  loading,
  editMode,
  onEditModeToggle,
  onSaveOrder,
  onCancelEdit,
  onFilter,
  onStatusFilter,
  statusFilter,
  namespaces,
  kinds
}) {
  const [searchMode, setSearchMode] = useState(false)
  const [hoveredBar, setHoveredBar] = useState(null)

  const statusBars = useMemo(() => {
    if (loading || !resources || resources.length === 0) {
      return []
    }
    return calculateStatusPercentages(resources)
  }, [loading, resources])

  const handleSearchOpen = () => {
    setSearchMode(true)
  }

  const handleSearchClose = () => {
    setSearchMode(false)
    onFilter({ namespace: null, kind: null, name: '' })
  }

  // Edit mode: show Save and Cancel icon buttons
  if (editMode) {
    return (
      <div class="card p-4">
        <div class="flex items-center justify-between">
          <div class="text-sm text-gray-600 dark:text-gray-400">
            Drag to reorder favorites
          </div>
          <div class="flex items-center gap-1">
            {/* Cancel button */}
            <button
              onClick={onCancelEdit}
              class="inline-flex items-center justify-center p-2 text-gray-600 dark:text-gray-400 hover:text-red-500 dark:hover:text-red-400 transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue rounded-md"
              title="Cancel"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
            {/* Save button */}
            <button
              onClick={onSaveOrder}
              class="inline-flex items-center justify-center p-2 text-gray-600 dark:text-gray-400 hover:text-green-500 dark:hover:text-green-400 transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue rounded-md"
              title="Save"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    )
  }

  // Search mode: show search input
  if (searchMode) {
    return (
      <div class="card p-4">
        <FavoritesSearch
          onFilter={onFilter}
          onClose={handleSearchClose}
          namespaces={namespaces}
          kinds={kinds}
        />
      </div>
    )
  }

  // Normal mode: show status bar with edit and search buttons on the same line
  return (
    <div class="card p-4">
      {/* Status bar with buttons on the right */}
      <div class="flex items-center gap-3">
        {/* Status bar - takes remaining space */}
        <div class="relative flex gap-0 flex-1" style={{ height: '32px' }}>
          {loading ? (
            <div class="w-full h-full bg-gray-200 dark:bg-gray-700 animate-pulse" />
          ) : statusBars.length === 0 ? (
            <div class="w-full h-full bg-gray-200 dark:bg-gray-700" />
          ) : (
            statusBars.map((bar, index) => {
              const colorClass = getStatusBarColor(bar.status)
              const isActive = statusFilter === bar.status
              const isFaded = statusFilter && !isActive

              return (
                <div
                  key={bar.status}
                  class="relative group"
                  style={{ flex: `0 0 ${bar.percentage}%` }}
                  onMouseEnter={() => setHoveredBar(index)}
                  onMouseLeave={() => setHoveredBar(null)}
                  onClick={() => onStatusFilter(statusFilter === bar.status ? null : bar.status)}
                >
                  <div class={`h-full ${colorClass} cursor-pointer transition-opacity ${isFaded ? 'opacity-30' : 'hover:opacity-80'}`} />

                  {/* Tooltip - hidden on mobile */}
                  {hoveredBar === index && (
                    <div class="hidden md:block absolute bottom-full left-1/2 -translate-x-1/2 mb-2 z-10 pointer-events-none">
                      <div class="bg-gray-900 dark:bg-gray-800 text-white text-xs rounded-lg py-2 px-3 shadow-lg whitespace-nowrap">
                        <div class="font-semibold">{bar.status}</div>
                        <div class="text-gray-300 mt-1">Count: {bar.count}</div>
                        <div class="text-gray-300">Percentage: {bar.percentage.toFixed(1)}%</div>
                        {isActive && <div class="text-blue-300 mt-1">Click to clear filter</div>}
                        <div class="absolute top-full left-1/2 -translate-x-1/2 -mt-px">
                          <div class="border-4 border-transparent border-t-gray-900 dark:border-t-gray-800"></div>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              )
            })
          )}
        </div>

        {/* Right: Edit order and Search buttons */}
        <div class="flex items-center gap-1 flex-shrink-0">
          {/* Edit order button */}
          <button
            onClick={onEditModeToggle}
            class="inline-flex items-center justify-center p-2 text-gray-600 dark:text-gray-400 hover:text-flux-blue dark:hover:text-blue-400 transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue rounded-md"
            title="Edit order"
            disabled={!resources || resources.length === 0}
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4.5h14.25M3 9h9.75M3 13.5h9.75m4.5-4.5v12m0 0-3.75-3.75M17.25 21 21 17.25" />
            </svg>
          </button>

          {/* Search button */}
          <button
            onClick={handleSearchOpen}
            class="inline-flex items-center justify-center p-2 text-gray-600 dark:text-gray-400 hover:text-flux-blue dark:hover:text-blue-400 transition-colors focus:outline-none focus:ring-2 focus:ring-flux-blue rounded-md"
            title="Search favorites"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  )
}
