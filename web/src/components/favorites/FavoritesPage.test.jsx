// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/preact'
import { FavoritesPage } from './FavoritesPage'
import { favorites, reorderFavorites, removeFavorite } from '../../utils/favorites'
import { fetchWithMock } from '../../utils/fetch'
import { POLL_INTERVAL_MS } from '../../utils/constants'

// Mock fetchWithMock
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

// Mock favorites utilities
vi.mock('../../utils/favorites', async () => {
  const actual = await vi.importActual('../../utils/favorites')
  return {
    ...actual,
    reorderFavorites: vi.fn(),
    removeFavorite: vi.fn()
  }
})

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    route: mockRoute
  })
}))

// Mock child components
vi.mock('./FavoritesHeader', () => ({
  FavoritesHeader: ({
    resources,
    loading,
    editMode,
    onEditModeToggle,
    onSaveOrder,
    onCancelEdit,
    onFilter,
    onStatusFilter
  }) => (
    <div data-testid="favorites-header">
      <span data-testid="header-loading">{loading ? 'loading' : 'loaded'}</span>
      <span data-testid="header-count">{resources?.length || 0}</span>
      <span data-testid="header-edit-mode">{editMode ? 'edit' : 'normal'}</span>
      <button data-testid="toggle-edit" onClick={onEditModeToggle}>Toggle Edit</button>
      <button data-testid="save-order" onClick={onSaveOrder}>Save</button>
      <button data-testid="cancel-edit" onClick={onCancelEdit}>Cancel</button>
      <button data-testid="filter-ns" onClick={() => onFilter({ namespace: 'flux-system', kind: null, name: '' })}>Filter NS</button>
      <button data-testid="filter-kind" onClick={() => onFilter({ namespace: null, kind: 'FluxInstance', name: '' })}>Filter Kind</button>
      <button data-testid="filter-name" onClick={() => onFilter({ namespace: null, kind: null, name: 'flux' })}>Filter Name</button>
      <button data-testid="filter-combined" onClick={() => onFilter({ namespace: 'flux-system', kind: 'FluxInstance', name: '' })}>Filter Combined</button>
      <button data-testid="clear-filter" onClick={() => onFilter({ namespace: null, kind: null, name: '' })}>Clear Filter</button>
      <button data-testid="status-filter-btn" onClick={() => onStatusFilter('Ready')}>Filter Ready</button>
      <button data-testid="clear-status-filter" onClick={() => onStatusFilter(null)}>Clear Status</button>
    </div>
  )
}))

vi.mock('./FavoriteCard', () => ({
  FavoriteCard: ({ favorite, resourceData }) => (
    <div data-testid={`favorite-card-${favorite.name}`}>
      <span data-testid="card-kind">{favorite.kind}</span>
      <span data-testid="card-namespace">{favorite.namespace}</span>
      <span data-testid="card-name">{favorite.name}</span>
      <span data-testid="card-status">{resourceData?.status || 'null'}</span>
    </div>
  )
}))

describe('FavoritesPage component', () => {
  const mockFavorites = [
    { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' },
    { kind: 'ResourceSet', namespace: 'flux-system', name: 'cluster' },
    { kind: 'Kustomization', namespace: 'default', name: 'app' }
  ]

  // All mock resources for the favorites endpoint
  const allMockResources = [
    { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux', status: 'Ready', lastReconciled: '2024-01-15T10:00:00Z' },
    { kind: 'ResourceSet', namespace: 'flux-system', name: 'cluster', status: 'Failed', lastReconciled: '2024-01-15T09:00:00Z' },
    { kind: 'Kustomization', namespace: 'default', name: 'app', status: 'Progressing', lastReconciled: '2024-01-15T08:00:00Z' }
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    // Clear favorites
    favorites.value = []

    // Setup default mock response for POST /api/v1/favorites
    // Returns only the resources that match the requested favorites
    fetchWithMock.mockImplementation(({ body }) => {
      const requestedFavorites = body?.favorites || []
      const matchedResources = allMockResources.filter(resource =>
        requestedFavorites.some(fav =>
          fav.kind === resource.kind &&
          fav.namespace === resource.namespace &&
          fav.name === resource.name
        )
      )
      return Promise.resolve({ resources: matchedResources })
    })
  })

  describe('empty state', () => {
    it('should render empty state when no favorites', async () => {
      favorites.value = []

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByText('No favorites yet')).toBeInTheDocument()
      })
    })

    it('should show "Browse resources" button in empty state', async () => {
      favorites.value = []

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByText('Browse resources')).toBeInTheDocument()
      })
    })

    it('should have correct href on browse resources link', async () => {
      favorites.value = []

      render(<FavoritesPage />)

      await waitFor(() => {
        const browseLink = screen.getByText('Browse resources')
        expect(browseLink).toHaveAttribute('href', '/resources')
      })
    })

    it('should show localStorage tip in empty state', async () => {
      favorites.value = []

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByText(/stored in your browser/)).toBeInTheDocument()
      })
    })
  })

  describe('loading state', () => {
    it('should show loading indicator initially', () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      // Header should receive loading=true initially
      expect(screen.getByTestId('header-loading')).toHaveTextContent('loading')
    })

    it('should show loaded state after data is fetched', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('header-loading')).toHaveTextContent('loaded')
      })
    })
  })

  describe('displaying favorites', () => {
    it('should render favorite cards for each favorite', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
        expect(screen.getByTestId('favorite-card-cluster')).toBeInTheDocument()
        expect(screen.getByTestId('favorite-card-app')).toBeInTheDocument()
      })
    })

    it('should pass resource data to favorite cards', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        const fluxCard = screen.getByTestId('favorite-card-flux')
        expect(fluxCard.querySelector('[data-testid="card-status"]')).toHaveTextContent('Ready')

        const clusterCard = screen.getByTestId('favorite-card-cluster')
        expect(clusterCard.querySelector('[data-testid="card-status"]')).toHaveTextContent('Failed')
      })
    })

    it('should show favorites count in title section', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByText('3 resources')).toBeInTheDocument()
      })
    })
  })

  describe('filtering', () => {
    it('should filter favorites by namespace', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Click filter button to filter by flux-system namespace
      const filterBtn = screen.getByTestId('filter-ns')
      fireEvent.click(filterBtn)

      await waitFor(() => {
        // Should show flux-system favorites only
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
        expect(screen.getByTestId('favorite-card-cluster')).toBeInTheDocument()
        // Default namespace favorite should be filtered out
        expect(screen.queryByTestId('favorite-card-app')).not.toBeInTheDocument()
      })
    })

    it('should filter favorites by status', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Click status filter button
      const statusFilterBtn = screen.getByTestId('status-filter-btn')
      fireEvent.click(statusFilterBtn)

      await waitFor(() => {
        // Should show only Ready favorites
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
        expect(screen.queryByTestId('favorite-card-cluster')).not.toBeInTheDocument() // Failed
        expect(screen.queryByTestId('favorite-card-app')).not.toBeInTheDocument() // Progressing
      })
    })

    it('should clear status filter', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Apply and then clear status filter
      const statusFilterBtn = screen.getByTestId('status-filter-btn')
      fireEvent.click(statusFilterBtn)

      await waitFor(() => {
        expect(screen.queryByTestId('favorite-card-cluster')).not.toBeInTheDocument()
      })

      const clearBtn = screen.getByTestId('clear-status-filter')
      fireEvent.click(clearBtn)

      await waitFor(() => {
        // All favorites should be visible again
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
        expect(screen.getByTestId('favorite-card-cluster')).toBeInTheDocument()
        expect(screen.getByTestId('favorite-card-app')).toBeInTheDocument()
      })
    })

    it('should show "No favorites match your search" when filter matches nothing', async () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]

      fetchWithMock.mockResolvedValue({
        resources: [
          { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux', status: 'Failed' }
        ]
      })

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Filter by Ready status (but resource is Failed)
      const statusFilterBtn = screen.getByTestId('status-filter-btn')
      fireEvent.click(statusFilterBtn)

      await waitFor(() => {
        expect(screen.getByText('No favorites match your search')).toBeInTheDocument()
      })
    })
  })

  describe('edit mode', () => {
    it('should enter edit mode when toggle is clicked', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      const toggleBtn = screen.getByTestId('toggle-edit')
      fireEvent.click(toggleBtn)

      await waitFor(() => {
        expect(screen.getByTestId('header-edit-mode')).toHaveTextContent('edit')
      })
    })

    it('should exit edit mode when cancel is clicked', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Enter edit mode
      const toggleBtn = screen.getByTestId('toggle-edit')
      fireEvent.click(toggleBtn)

      await waitFor(() => {
        expect(screen.getByTestId('header-edit-mode')).toHaveTextContent('edit')
      })

      // Cancel edit mode
      const cancelBtn = screen.getByTestId('cancel-edit')
      fireEvent.click(cancelBtn)

      await waitFor(() => {
        expect(screen.getByTestId('header-edit-mode')).toHaveTextContent('normal')
      })
    })
  })

  describe('error handling', () => {
    it('should show error message when fetch fails', async () => {
      favorites.value = mockFavorites
      fetchWithMock.mockRejectedValue(new Error('Network error'))

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByText(/Failed to load favorites/)).toBeInTheDocument()
      })
    })

    it('should handle missing resources gracefully', async () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'deleted-resource' }
      ]

      fetchWithMock.mockResolvedValue({ resources: [] })

      render(<FavoritesPage />)

      await waitFor(() => {
        // Card should still render but with null resourceData
        const card = screen.getByTestId('favorite-card-deleted-resource')
        expect(card.querySelector('[data-testid="card-status"]')).toHaveTextContent('null')
      })
    })
  })

  describe('page title', () => {
    it('should display "Favorites" title', () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      expect(screen.getByText('Favorites')).toBeInTheDocument()
    })
  })

  describe('data-testid', () => {
    it('should have favorites-page data-testid on main element', () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      expect(screen.getByTestId('favorites-page')).toBeInTheDocument()
    })
  })

  describe('edit mode - additional tests', () => {
    it('should show draggable items with drag handles in edit mode', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Enter edit mode
      const toggleBtn = screen.getByTestId('toggle-edit')
      fireEvent.click(toggleBtn)

      await waitFor(() => {
        // Edit mode items should have draggable attribute
        const editItems = document.querySelectorAll('[draggable="true"]')
        expect(editItems.length).toBe(3)

        // Should show drag handle icons (horizontal lines)
        const dragHandles = document.querySelectorAll('path[d="M4 8h16M4 16h16"]')
        expect(dragHandles.length).toBe(3)
      })
    })

    it('should call reorderFavorites when save is clicked', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Enter edit mode
      const toggleBtn = screen.getByTestId('toggle-edit')
      fireEvent.click(toggleBtn)

      await waitFor(() => {
        expect(screen.getByTestId('header-edit-mode')).toHaveTextContent('edit')
      })

      // Save order
      const saveBtn = screen.getByTestId('save-order')
      fireEvent.click(saveBtn)

      expect(reorderFavorites).toHaveBeenCalled()
    })

    it('should exit edit mode when Escape key is pressed', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Enter edit mode
      const toggleBtn = screen.getByTestId('toggle-edit')
      fireEvent.click(toggleBtn)

      await waitFor(() => {
        expect(screen.getByTestId('header-edit-mode')).toHaveTextContent('edit')
      })

      // Press Escape
      fireEvent.keyDown(window, { key: 'Escape' })

      await waitFor(() => {
        expect(screen.getByTestId('header-edit-mode')).toHaveTextContent('normal')
      })
    })

    it('should show delete button for each item in edit mode', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Enter edit mode
      const toggleBtn = screen.getByTestId('toggle-edit')
      fireEvent.click(toggleBtn)

      await waitFor(() => {
        // Delete buttons (trash icons) should be present
        const deleteButtons = screen.getAllByTitle('Remove from favorites')
        expect(deleteButtons.length).toBe(3)
      })
    })

    it('should call removeFavorite when delete button is clicked', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Enter edit mode
      const toggleBtn = screen.getByTestId('toggle-edit')
      fireEvent.click(toggleBtn)

      await waitFor(() => {
        expect(screen.getByTestId('header-edit-mode')).toHaveTextContent('edit')
      })

      // Click first delete button
      const deleteButtons = screen.getAllByTitle('Remove from favorites')
      fireEvent.click(deleteButtons[0])

      expect(removeFavorite).toHaveBeenCalledWith('FluxInstance', 'flux-system', 'flux')
    })
  })

  describe('filtering - additional tests', () => {
    it('should filter favorites by kind', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Click filter button to filter by FluxInstance kind
      const filterBtn = screen.getByTestId('filter-kind')
      fireEvent.click(filterBtn)

      await waitFor(() => {
        // Should show only FluxInstance favorites
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
        expect(screen.queryByTestId('favorite-card-cluster')).not.toBeInTheDocument() // ResourceSet
        expect(screen.queryByTestId('favorite-card-app')).not.toBeInTheDocument() // Kustomization
      })
    })

    it('should filter favorites by name (text search)', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Click filter button to search for "flux"
      const filterBtn = screen.getByTestId('filter-name')
      fireEvent.click(filterBtn)

      await waitFor(() => {
        // Should show favorites containing "flux" in name, namespace, or kind
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument() // name matches
        expect(screen.getByTestId('favorite-card-cluster')).toBeInTheDocument() // namespace flux-system matches
        expect(screen.queryByTestId('favorite-card-app')).not.toBeInTheDocument() // no match
      })
    })

    it('should filter favorites by combined namespace and kind', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
      })

      // Click filter button to filter by flux-system namespace AND FluxInstance kind
      const filterBtn = screen.getByTestId('filter-combined')
      fireEvent.click(filterBtn)

      await waitFor(() => {
        // Should show only FluxInstance in flux-system
        expect(screen.getByTestId('favorite-card-flux')).toBeInTheDocument()
        expect(screen.queryByTestId('favorite-card-cluster')).not.toBeInTheDocument() // ResourceSet (wrong kind)
        expect(screen.queryByTestId('favorite-card-app')).not.toBeInTheDocument() // default namespace
      })
    })
  })

  describe('auto-refresh', () => {
    beforeEach(() => {
      vi.useFakeTimers()
    })

    afterEach(() => {
      vi.useRealTimers()
    })

    it('should set up 30-second refresh interval', async () => {
      favorites.value = mockFavorites
      const setIntervalSpy = vi.spyOn(global, 'setInterval')

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('header-loading')).toHaveTextContent('loaded')
      })

      expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), POLL_INTERVAL_MS)
    })

    it('should call fetchWithMock on refresh interval', async () => {
      favorites.value = mockFavorites

      render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('header-loading')).toHaveTextContent('loaded')
      })

      // Clear the initial call count
      fetchWithMock.mockClear()

      // Advance timer by 30 seconds
      vi.advanceTimersByTime(POLL_INTERVAL_MS)

      expect(fetchWithMock).toHaveBeenCalledTimes(1)
    })

    it('should clear interval on unmount', async () => {
      favorites.value = mockFavorites
      const clearIntervalSpy = vi.spyOn(global, 'clearInterval')

      const { unmount } = render(<FavoritesPage />)

      await waitFor(() => {
        expect(screen.getByTestId('header-loading')).toHaveTextContent('loaded')
      })

      unmount()

      expect(clearIntervalSpy).toHaveBeenCalled()
    })
  })
})
