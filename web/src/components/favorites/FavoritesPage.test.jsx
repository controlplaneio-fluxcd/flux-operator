// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/preact'
import { FavoritesPage } from './FavoritesPage'
import { favorites } from '../../utils/favorites'
import { fetchWithMock } from '../../utils/fetch'

// Mock fetchWithMock
vi.mock('../../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

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
      <button data-testid="filter-btn" onClick={() => onFilter({ namespace: 'flux-system', kind: null, name: '' })}>Filter NS</button>
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

  const mockResourcesResponse = {
    resources: [
      { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux', status: 'Ready', lastReconciled: '2024-01-15T10:00:00Z' },
      { kind: 'ResourceSet', namespace: 'flux-system', name: 'cluster', status: 'Failed', lastReconciled: '2024-01-15T09:00:00Z' }
    ]
  }

  const mockDefaultResourcesResponse = {
    resources: [
      { kind: 'Kustomization', namespace: 'default', name: 'app', status: 'Progressing', lastReconciled: '2024-01-15T08:00:00Z' }
    ]
  }

  beforeEach(() => {
    vi.clearAllMocks()
    // Clear favorites
    favorites.value = []

    // Setup default mock responses
    fetchWithMock.mockImplementation(({ endpoint }) => {
      if (endpoint.includes('namespace=flux-system')) {
        return Promise.resolve(mockResourcesResponse)
      }
      if (endpoint.includes('namespace=default')) {
        return Promise.resolve(mockDefaultResourcesResponse)
      }
      return Promise.resolve({ resources: [] })
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

    it('should navigate to /resources when browse button is clicked', async () => {
      favorites.value = []

      render(<FavoritesPage />)

      await waitFor(() => {
        const browseButton = screen.getByText('Browse resources')
        fireEvent.click(browseButton)
        expect(mockRoute).toHaveBeenCalledWith('/resources')
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
      const filterBtn = screen.getByTestId('filter-btn')
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
})
