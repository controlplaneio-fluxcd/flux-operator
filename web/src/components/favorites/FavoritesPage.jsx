// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useMemo, useCallback, useRef } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { fetchWithMock } from '../../utils/fetch'
import { favorites, reorderFavorites, getFavoriteKey, removeFavorite } from '../../utils/favorites'
import { usePageTitle } from '../../utils/title'
import { FavoritesHeader } from './FavoritesHeader'
import { FavoriteCard } from './FavoriteCard'
import { FluxOperatorIcon } from '../common/icons'

/**
 * FavoritesPage - Main page displaying favorite resources
 *
 * Features:
 * - Loads favorite resources from localStorage
 * - Fetches current status for each favorite from API
 * - Auto-refreshes every 30 seconds
 * - Supports filtering by kind, namespace, name
 * - Supports drag-and-drop reordering in edit mode
 */
export function FavoritesPage() {
  usePageTitle('Favorites')
  const location = useLocation()

  // State
  const [resourcesData, setResourcesData] = useState({}) // Map of key -> resource data
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false) // Delayed refresh indicator
  const [error, setError] = useState(null)
  const [editMode, setEditMode] = useState(false)
  const [editOrder, setEditOrder] = useState([]) // Temporary order during edit
  const [draggedIndex, setDraggedIndex] = useState(null)
  const [filter, setFilter] = useState({ namespace: null, kind: null, name: '' })
  const [statusFilter, setStatusFilter] = useState(null)

  // Track if initial load has completed (to avoid showing loading on refresh)
  const initialLoadDone = useRef(false)
  const refreshTimeoutRef = useRef(null)

  // Get current favorites from signal
  const currentFavorites = favorites.value

  // Extract unique namespaces and kinds from favorites for search suggestions
  const namespaces = useMemo(() => {
    return [...new Set(currentFavorites.map(f => f.namespace))].sort()
  }, [currentFavorites])

  const kinds = useMemo(() => {
    return [...new Set(currentFavorites.map(f => f.kind))].sort()
  }, [currentFavorites])

  // Fetch resource data for all favorites
  const fetchFavoritesData = useCallback(async () => {
    if (currentFavorites.length === 0) {
      setResourcesData({})
      setLoading(false)
      initialLoadDone.current = true
      return
    }

    // Only show loading on initial load, not on refresh
    if (!initialLoadDone.current) {
      setLoading(true)
    } else {
      // For refreshes, show spinner after 300ms delay
      refreshTimeoutRef.current = window.setTimeout(() => {
        setRefreshing(true)
      }, 300)
    }
    setError(null)

    try {
      // Send all favorites in a single POST request
      const data = await fetchWithMock({
        endpoint: '/api/v1/favorites',
        mockPath: '../mock/resources',
        mockExport: 'getMockFavorites',
        method: 'POST',
        body: { favorites: currentFavorites }
      })

      // Build results map from returned resources
      const results = {}
      const resources = data.resources || []

      // First, mark all favorites as not found
      currentFavorites.forEach(fav => {
        const key = getFavoriteKey(fav.kind, fav.namespace, fav.name)
        results[key] = null
      })

      // Then, update with found resources
      resources.forEach(resource => {
        const key = getFavoriteKey(resource.kind, resource.namespace, resource.name)
        results[key] = {
          status: resource.status,
          lastReconciled: resource.lastReconciled,
          message: resource.message
        }
      })

      setResourcesData(results)
    } catch (err) {
      // Only show error panel on initial load, not on refresh
      if (!initialLoadDone.current) {
        setError(err.message)
      }
      // Don't clear existing data on error - keep showing stale data
    } finally {
      // Clear the delayed refresh timeout and hide spinner
      if (refreshTimeoutRef.current) {
        window.clearTimeout(refreshTimeoutRef.current)
        refreshTimeoutRef.current = null
      }
      setRefreshing(false)
      setLoading(false)
      initialLoadDone.current = true
    }
  }, [currentFavorites])

  // Fetch data on mount and when favorites change
  useEffect(() => {
    fetchFavoritesData()

    // Auto-refresh every 30 seconds
    const interval = setInterval(fetchFavoritesData, 30000)
    return () => clearInterval(interval)
  }, [fetchFavoritesData])

  // Build list of favorites with their resource data
  const favoritesWithData = useMemo(() => {
    return currentFavorites.map(fav => {
      const key = getFavoriteKey(fav.kind, fav.namespace, fav.name)
      return {
        ...fav,
        resourceData: resourcesData[key] || null
      }
    })
  }, [currentFavorites, resourcesData])

  // Apply filters
  const filteredFavorites = useMemo(() => {
    return favoritesWithData.filter(fav => {
      if (filter.namespace && fav.namespace !== filter.namespace) return false
      if (filter.kind && fav.kind !== filter.kind) return false
      if (statusFilter && (fav.resourceData?.status || 'Unknown') !== statusFilter) return false
      if (filter.name) {
        const searchLower = filter.name.toLowerCase()
        const matchesName = fav.name.toLowerCase().includes(searchLower)
        const matchesNamespace = fav.namespace.toLowerCase().includes(searchLower)
        const matchesKind = fav.kind.toLowerCase().includes(searchLower)
        if (!matchesName && !matchesNamespace && !matchesKind) return false
      }
      return true
    })
  }, [favoritesWithData, filter, statusFilter])

  // Extract just resources array for status chart (use all favorites, not filtered by status)
  const resourcesForChart = useMemo(() => {
    // Apply namespace/kind/name filters but not status filter for the chart
    return favoritesWithData
      .filter(fav => {
        if (filter.namespace && fav.namespace !== filter.namespace) return false
        if (filter.kind && fav.kind !== filter.kind) return false
        if (filter.name) {
          const searchLower = filter.name.toLowerCase()
          const matchesName = fav.name.toLowerCase().includes(searchLower)
          const matchesNamespace = fav.namespace.toLowerCase().includes(searchLower)
          const matchesKind = fav.kind.toLowerCase().includes(searchLower)
          if (!matchesName && !matchesNamespace && !matchesKind) return false
        }
        return true
      })
      .map(f => ({
        status: f.resourceData?.status || 'Unknown'
      }))
  }, [favoritesWithData, filter])

  // Edit mode handlers
  const handleEditModeToggle = () => {
    if (!editMode) {
      setEditOrder([...currentFavorites])
    }
    setEditMode(!editMode)
  }

  const handleSaveOrder = () => {
    reorderFavorites(editOrder)
    setEditMode(false)
  }

  const handleCancelEdit = useCallback(() => {
    setEditOrder([])
    setEditMode(false)
  }, [])

  // Handle Escape key to cancel edit mode
  useEffect(() => {
    if (!editMode) return

    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        handleCancelEdit()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [editMode, handleCancelEdit])

  // Drag and drop handlers (desktop)
  const handleDragStart = (index) => {
    setDraggedIndex(index)
  }

  const handleDragOver = (e, index) => {
    e.preventDefault()
    if (draggedIndex === null || draggedIndex === index) return

    // Reorder the editOrder array
    const newOrder = [...editOrder]
    const [draggedItem] = newOrder.splice(draggedIndex, 1)
    newOrder.splice(index, 0, draggedItem)
    setEditOrder(newOrder)
    setDraggedIndex(index)
  }

  const handleDragEnd = () => {
    setDraggedIndex(null)
  }

  // Touch handlers for mobile reordering
  const touchStartY = useRef(null)
  const touchStartIndex = useRef(null)

  const handleTouchStart = (e, index) => {
    touchStartY.current = e.touches[0].clientY
    touchStartIndex.current = index
    setDraggedIndex(index)
  }

  const handleTouchMove = (e) => {
    if (touchStartIndex.current === null) return

    const touch = e.touches[0]
    const element = e.currentTarget
    const rect = element.getBoundingClientRect()
    const itemHeight = rect.height + 8 // height + gap

    // Calculate how many positions we've moved
    const deltaY = touch.clientY - touchStartY.current
    const positionDelta = Math.round(deltaY / itemHeight)

    if (positionDelta !== 0) {
      const newIndex = Math.max(0, Math.min(editOrder.length - 1, touchStartIndex.current + positionDelta))

      if (newIndex !== draggedIndex) {
        const newOrder = [...editOrder]
        const [draggedItem] = newOrder.splice(draggedIndex, 1)
        newOrder.splice(newIndex, 0, draggedItem)
        setEditOrder(newOrder)
        setDraggedIndex(newIndex)
      }
    }
  }

  const handleTouchEnd = () => {
    touchStartY.current = null
    touchStartIndex.current = null
    setDraggedIndex(null)
  }

  // Filter handler
  const handleFilter = useCallback((newFilter) => {
    setFilter(newFilter)
  }, [])

  // Status filter handler
  const handleStatusFilter = useCallback((status) => {
    setStatusFilter(status)
  }, [])

  // Empty state
  if (!loading && currentFavorites.length === 0) {
    return (
      <main data-testid="favorites-page" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
        <div class="space-y-6">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Favorites</h2>

          <div class="card py-12">
            <div class="text-center">
              <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
              </svg>
              <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-white">No favorites yet</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Mark resources as favorites for quick access.
              </p>
              <div class="mt-6">
                <button
                  onClick={() => location.route('/resources')}
                  class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-flux-blue hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue"
                >
                  Browse resources
                </button>
              </div>
              <p class="mt-6 text-sm text-gray-400 dark:text-gray-500">
                Favorites are stored in your browser's local storage
              </p>
            </div>
          </div>
        </div>
      </main>
    )
  }

  return (
    <main data-testid="favorites-page" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">
        {/* Page Title */}
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Favorites</h2>
          {/* Favorites count with refresh indicator */}
          {!loading && filteredFavorites.length > 0 && (
            <span class="text-sm text-gray-600 dark:text-gray-400 flex items-center gap-2">
              {refreshing && (
                <div class="animate-spin rounded-full h-3 w-3 border-b-2 border-gray-400"></div>
              )}
              {filteredFavorites.length} resources
            </span>
          )}
        </div>

        {/* Header with status bar and controls */}
        <FavoritesHeader
          resources={resourcesForChart}
          loading={loading}
          editMode={editMode}
          onEditModeToggle={handleEditModeToggle}
          onSaveOrder={handleSaveOrder}
          onCancelEdit={handleCancelEdit}
          onFilter={handleFilter}
          onStatusFilter={handleStatusFilter}
          statusFilter={statusFilter}
          namespaces={namespaces}
          kinds={kinds}
        />

        {/* Error State */}
        {error && (
          <div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
            <div class="flex">
              <svg class="w-5 h-5 text-red-400 dark:text-red-600" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
              </svg>
              <div class="ml-3">
                <p class="text-sm text-red-800 dark:text-red-200">
                  Failed to load favorites: {error}
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Edit Mode: Simple draggable list */}
        {editMode && (
          <div class="space-y-2">
            {editOrder.map((fav, index) => (
              <div
                key={getFavoriteKey(fav.kind, fav.namespace, fav.name)}
                draggable
                onDragStart={() => handleDragStart(index)}
                onDragOver={(e) => handleDragOver(e, index)}
                onDragEnd={handleDragEnd}
                onTouchStart={(e) => handleTouchStart(e, index)}
                onTouchMove={handleTouchMove}
                onTouchEnd={handleTouchEnd}
                class={`card p-3 flex items-center gap-3 cursor-move touch-none ${
                  draggedIndex === index ? 'opacity-50' : ''
                }`}
              >
                {/* Drag handle */}
                <svg class="w-5 h-5 text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                </svg>
                {/* Resource info */}
                <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase flex-shrink-0">
                  {fav.kind}
                </span>
                <span class="text-sm text-gray-900 dark:text-gray-100 truncate flex-grow">
                  {fav.namespace}/{fav.name}
                </span>
                {/* Delete button */}
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    removeFavorite(fav.kind, fav.namespace, fav.name)
                    setEditOrder(editOrder.filter((_, i) => i !== index))
                  }}
                  class="p-1 rounded text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors flex-shrink-0"
                  title="Remove from favorites"
                >
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Normal Mode: Favorite cards grid */}
        {!editMode && !loading && filteredFavorites.length > 0 && (
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {filteredFavorites.map((fav) => (
              <FavoriteCard
                key={getFavoriteKey(fav.kind, fav.namespace, fav.name)}
                favorite={fav}
                resourceData={fav.resourceData}
              />
            ))}
          </div>
        )}

        {/* No results after filtering */}
        {!editMode && !loading && filteredFavorites.length === 0 && currentFavorites.length > 0 && (
          <div class="card py-8">
            <div class="text-center">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                No favorites match your search
              </p>
            </div>
          </div>
        )}

        {/* Loading state */}
        {loading && (
          <div class="flex items-center justify-center p-8">
            <FluxOperatorIcon className="animate-spin h-8 w-8 text-flux-blue" />
            <span class="ml-3 text-gray-600 dark:text-gray-400">Loading favorites...</span>
          </div>
        )}
      </div>
    </main>
  )
}
