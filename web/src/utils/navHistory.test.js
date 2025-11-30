// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import {
  navHistory,
  getNavHistoryKey,
  isHomePage,
  addToNavHistory,
  clearNavHistory
} from './navHistory'

describe('navHistory utilities', () => {
  beforeEach(() => {
    // Clear navHistory and localStorage before each test
    navHistory.value = []
    global.localStorageMock.clear()
    global.localStorageMock.getItem.mockClear()
    global.localStorageMock.setItem.mockClear()
  })

  describe('getNavHistoryKey', () => {
    it('should generate a key in format kind/namespace/name', () => {
      const key = getNavHistoryKey('FluxInstance', 'flux-system', 'flux')
      expect(key).toBe('FluxInstance/flux-system/flux')
    })

    it('should handle special characters in namespace and name', () => {
      const key = getNavHistoryKey('Kustomization', 'my-namespace', 'my-app-config')
      expect(key).toBe('Kustomization/my-namespace/my-app-config')
    })
  })

  describe('isHomePage', () => {
    it('should return true for FluxReport kind', () => {
      expect(isHomePage('FluxReport')).toBe(true)
    })

    it('should return false for other kinds', () => {
      expect(isHomePage('FluxInstance')).toBe(false)
      expect(isHomePage('Kustomization')).toBe(false)
      expect(isHomePage('HelmRelease')).toBe(false)
      expect(isHomePage('ResourceSet')).toBe(false)
    })
  })

  describe('addToNavHistory', () => {
    it('should add a new entry to the beginning of the list', () => {
      addToNavHistory('FluxInstance', 'flux-system', 'flux')
      expect(navHistory.value).toHaveLength(1)
      expect(navHistory.value[0]).toEqual({
        kind: 'FluxInstance',
        namespace: 'flux-system',
        name: 'flux'
      })
    })

    it('should add new entries at the beginning (most recent first)', () => {
      addToNavHistory('FluxInstance', 'flux-system', 'flux')
      addToNavHistory('ResourceSet', 'flux-system', 'cluster')

      expect(navHistory.value).toHaveLength(2)
      expect(navHistory.value[0]).toEqual({
        kind: 'ResourceSet',
        namespace: 'flux-system',
        name: 'cluster'
      })
      expect(navHistory.value[1]).toEqual({
        kind: 'FluxInstance',
        namespace: 'flux-system',
        name: 'flux'
      })
    })

    it('should move existing entry to top when added again', () => {
      addToNavHistory('FluxInstance', 'flux-system', 'flux')
      addToNavHistory('ResourceSet', 'flux-system', 'cluster')
      addToNavHistory('Kustomization', 'flux-system', 'app')

      // Add first entry again - should move to top
      addToNavHistory('FluxInstance', 'flux-system', 'flux')

      expect(navHistory.value).toHaveLength(3)
      expect(navHistory.value[0]).toEqual({
        kind: 'FluxInstance',
        namespace: 'flux-system',
        name: 'flux'
      })
      expect(navHistory.value[1]).toEqual({
        kind: 'Kustomization',
        namespace: 'flux-system',
        name: 'app'
      })
      expect(navHistory.value[2]).toEqual({
        kind: 'ResourceSet',
        namespace: 'flux-system',
        name: 'cluster'
      })
    })

    it('should limit history to 5 entries', () => {
      addToNavHistory('Resource1', 'ns1', 'name1')
      addToNavHistory('Resource2', 'ns2', 'name2')
      addToNavHistory('Resource3', 'ns3', 'name3')
      addToNavHistory('Resource4', 'ns4', 'name4')
      addToNavHistory('Resource5', 'ns5', 'name5')
      addToNavHistory('Resource6', 'ns6', 'name6')

      expect(navHistory.value).toHaveLength(5)
      // Most recent should be first
      expect(navHistory.value[0]).toEqual({
        kind: 'Resource6',
        namespace: 'ns6',
        name: 'name6'
      })
      // Oldest (Resource1) should be removed
      expect(navHistory.value.find(e => e.kind === 'Resource1')).toBeUndefined()
    })

    it('should not increase count when moving existing entry to top', () => {
      addToNavHistory('Resource1', 'ns1', 'name1')
      addToNavHistory('Resource2', 'ns2', 'name2')
      addToNavHistory('Resource3', 'ns3', 'name3')
      addToNavHistory('Resource4', 'ns4', 'name4')
      addToNavHistory('Resource5', 'ns5', 'name5')

      // Re-add existing entry
      addToNavHistory('Resource3', 'ns3', 'name3')

      expect(navHistory.value).toHaveLength(5)
      expect(navHistory.value[0]).toEqual({
        kind: 'Resource3',
        namespace: 'ns3',
        name: 'name3'
      })
    })

    it('should sync to localStorage when adding', () => {
      addToNavHistory('FluxInstance', 'flux-system', 'flux')

      expect(global.localStorageMock.setItem).toHaveBeenCalledWith(
        'nav-history',
        JSON.stringify([{ kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }])
      )
    })
  })

  describe('clearNavHistory', () => {
    it('should clear all entries', () => {
      navHistory.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' },
        { kind: 'ResourceSet', namespace: 'flux-system', name: 'cluster' }
      ]

      clearNavHistory()

      expect(navHistory.value).toHaveLength(0)
    })

    it('should sync to localStorage when clearing', () => {
      navHistory.value = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]
      global.localStorageMock.setItem.mockClear()

      clearNavHistory()

      expect(global.localStorageMock.setItem).toHaveBeenCalledWith(
        'nav-history',
        JSON.stringify([])
      )
    })
  })

  describe('localStorage persistence', () => {
    it('should initialize from localStorage on module load', async () => {
      const storedHistory = [
        { kind: 'FluxInstance', namespace: 'flux-system', name: 'flux' }
      ]
      global.localStorageMock.getItem.mockReturnValueOnce(JSON.stringify(storedHistory))

      vi.resetModules()
      const { navHistory: freshNavHistory } = await vi.importActual('./navHistory')

      expect(global.localStorageMock.getItem).toHaveBeenCalledWith('nav-history')
      expect(freshNavHistory.value).toEqual(storedHistory)
    })

    it('should handle invalid JSON in localStorage gracefully', async () => {
      global.localStorageMock.getItem.mockReturnValueOnce('invalid json')

      vi.resetModules()
      const { navHistory: freshNavHistory } = await vi.importActual('./navHistory')

      expect(freshNavHistory.value).toEqual([])
    })

    it('should handle empty localStorage', async () => {
      global.localStorageMock.getItem.mockReturnValueOnce(null)

      vi.resetModules()
      const { navHistory: freshNavHistory } = await vi.importActual('./navHistory')

      expect(freshNavHistory.value).toEqual([])
    })
  })
})
