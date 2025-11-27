// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import {
  favorites,
  getFavoriteKey,
  isFavorite,
  addFavorite,
  removeFavorite,
  toggleFavorite,
  reorderFavorites
} from './favorites'

describe('favorites utilities', () => {
  beforeEach(() => {
    // Clear favorites and localStorage before each test
    favorites.value = []
    global.localStorageMock.clear()
    global.localStorageMock.getItem.mockClear()
    global.localStorageMock.setItem.mockClear()
  })

  describe('getFavoriteKey', () => {
    it('should generate a key in format kind/namespace/name', () => {
      const key = getFavoriteKey('FluxInstance', 'flux-system', 'flux')
      expect(key).toBe('FluxInstance/flux-system/flux')
    })

    it('should handle special characters in namespace and name', () => {
      const key = getFavoriteKey('Kustomization', 'my-namespace', 'my-app-config')
      expect(key).toBe('Kustomization/my-namespace/my-app-config')
    })
  })

  describe('isFavorite', () => {
    it('should return false when favorites is empty', () => {
      expect(isFavorite('FluxInstance', 'flux-system', 'flux')).toBe(false)
    })

    it('should return true when resource is in favorites', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]
      expect(isFavorite('FluxInstance', 'flux-system', 'flux')).toBe(true)
    })

    it('should return false when resource is not in favorites', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]
      expect(isFavorite('FluxInstance', 'flux-system', 'other')).toBe(false)
    })

    it('should match by kind, namespace, and name exactly', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]
      // Different kind
      expect(isFavorite('ResourceSet', 'flux-system', 'flux')).toBe(false)
      // Different namespace
      expect(isFavorite('FluxInstance', 'default', 'flux')).toBe(false)
      // Different name
      expect(isFavorite('FluxInstance', 'flux-system', 'flux2')).toBe(false)
    })
  })

  describe('addFavorite', () => {
    it('should add a new favorite to the beginning of the list', () => {
      addFavorite('FluxInstance', 'flux-system', 'flux')
      expect(favorites.value).toHaveLength(1)
      expect(favorites.value[0]).toEqual({
        kind: 'FluxInstance',
        namespace: 'flux-system',
        name: 'flux'
      })
    })

    it('should add new favorites at the beginning (top of list)', () => {
      addFavorite('FluxInstance', 'flux-system', 'flux')
      addFavorite('ResourceSet', 'flux-system', 'cluster')

      expect(favorites.value).toHaveLength(2)
      expect(favorites.value[0]).toEqual({
        kind: 'ResourceSet',
        namespace: 'flux-system',
        name: 'cluster'
      })
      expect(favorites.value[1]).toEqual({
        kind: 'FluxInstance',
        namespace: 'flux-system',
        name: 'flux'
      })
    })

    it('should not add duplicate favorites', () => {
      addFavorite('FluxInstance', 'flux-system', 'flux')
      addFavorite('FluxInstance', 'flux-system', 'flux')

      expect(favorites.value).toHaveLength(1)
    })

    it('should sync to localStorage when adding', () => {
      addFavorite('FluxInstance', 'flux-system', 'flux')

      expect(global.localStorageMock.setItem).toHaveBeenCalledWith(
        'favorites',
        JSON.stringify([{ kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }])
      )
    })
  })

  describe('removeFavorite', () => {
    it('should remove an existing favorite', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' },
        { kind: 'ResourceSet', namespace: 'flux-system', name: 'cluster' }
      ]

      removeFavorite('FluxInstance', 'flux-system', 'flux')

      expect(favorites.value).toHaveLength(1)
      expect(favorites.value[0]).toEqual({
        kind: 'ResourceSet',
        namespace: 'flux-system',
        name: 'cluster'
      })
    })

    it('should do nothing when removing non-existent favorite', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]

      removeFavorite('FluxInstance', 'flux-system', 'nonexistent')

      expect(favorites.value).toHaveLength(1)
    })

    it('should sync to localStorage when removing', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]
      global.localStorageMock.setItem.mockClear()

      removeFavorite('FluxInstance', 'flux-system', 'flux')

      expect(global.localStorageMock.setItem).toHaveBeenCalledWith(
        'favorites',
        JSON.stringify([])
      )
    })
  })

  describe('toggleFavorite', () => {
    it('should add favorite when not already favorited', () => {
      toggleFavorite('FluxInstance', 'flux-system', 'flux')

      expect(favorites.value).toHaveLength(1)
      expect(isFavorite('FluxInstance', 'flux-system', 'flux')).toBe(true)
    })

    it('should remove favorite when already favorited', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]

      toggleFavorite('FluxInstance', 'flux-system', 'flux')

      expect(favorites.value).toHaveLength(0)
      expect(isFavorite('FluxInstance', 'flux-system', 'flux')).toBe(false)
    })
  })

  describe('reorderFavorites', () => {
    it('should replace favorites with new order', () => {
      favorites.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' },
        { kind: 'ResourceSet', namespace: 'flux-system', name: 'cluster' },
        { kind: 'Kustomization', namespace: 'flux-system', name: 'app' }
      ]

      const newOrder = [
        { kind: 'Kustomization', namespace: 'flux-system', name: 'app' },
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' },
        { kind: 'ResourceSet', namespace: 'flux-system', name: 'cluster' }
      ]

      reorderFavorites(newOrder)

      expect(favorites.value).toEqual(newOrder)
    })

    it('should sync reordered list to localStorage', () => {
      const newOrder = [
        { kind: 'ResourceSet', namespace: 'flux-system', name: 'cluster' }
      ]
      global.localStorageMock.setItem.mockClear()

      reorderFavorites(newOrder)

      expect(global.localStorageMock.setItem).toHaveBeenCalledWith(
        'favorites',
        JSON.stringify(newOrder)
      )
    })
  })

  describe('localStorage persistence', () => {
    it('should initialize from localStorage on module load', async () => {
      const storedFavorites = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]
      global.localStorageMock.getItem.mockReturnValueOnce(JSON.stringify(storedFavorites))

      vi.resetModules()
      const { favorites: freshFavorites } = await vi.importActual('./favorites')

      expect(global.localStorageMock.getItem).toHaveBeenCalledWith('favorites')
      expect(freshFavorites.value).toEqual(storedFavorites)
    })

    it('should handle invalid JSON in localStorage gracefully', async () => {
      global.localStorageMock.getItem.mockReturnValueOnce('invalid json')

      vi.resetModules()
      const { favorites: freshFavorites } = await vi.importActual('./favorites')

      expect(freshFavorites.value).toEqual([])
    })

    it('should handle empty localStorage', async () => {
      global.localStorageMock.getItem.mockReturnValueOnce(null)

      vi.resetModules()
      const { favorites: freshFavorites } = await vi.importActual('./favorites')

      expect(freshFavorites.value).toEqual([])
    })
  })
})
