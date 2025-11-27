// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { getStoredVersion, checkVersionChange } from './version'

describe('version utilities', () => {
  let reloadMock

  beforeEach(() => {
    // Reset localStorage mock
    global.localStorageMock.getItem.mockClear()
    global.localStorageMock.setItem.mockClear()
    global.localStorageMock.clear()

    // Mock window.location.reload
    reloadMock = vi.fn()
    Object.defineProperty(window, 'location', {
      value: { reload: reloadMock },
      writable: true
    })
  })

  describe('getStoredVersion', () => {
    it('should return null when no version is stored', () => {
      global.localStorageMock.getItem.mockReturnValue(null)

      const result = getStoredVersion()

      expect(result).toBeNull()
      expect(global.localStorageMock.getItem).toHaveBeenCalledWith('flux-operator-version')
    })

    it('should return the stored version when present', () => {
      global.localStorageMock.getItem.mockReturnValue('v0.33.0')

      const result = getStoredVersion()

      expect(result).toBe('v0.33.0')
      expect(global.localStorageMock.getItem).toHaveBeenCalledWith('flux-operator-version')
    })
  })

  describe('checkVersionChange', () => {
    it('should store version on first call without triggering reload', () => {
      global.localStorageMock.getItem.mockReturnValue(null)

      const result = checkVersionChange('v0.33.0')

      expect(result).toBe(false)
      expect(global.localStorageMock.setItem).toHaveBeenCalledWith('flux-operator-version', 'v0.33.0')
      expect(reloadMock).not.toHaveBeenCalled()
    })

    it('should trigger reload when version changes', () => {
      global.localStorageMock.getItem.mockReturnValue('v0.32.0')

      const result = checkVersionChange('v0.33.0')

      expect(result).toBe(true)
      expect(global.localStorageMock.setItem).toHaveBeenCalledWith('flux-operator-version', 'v0.33.0')
      expect(reloadMock).toHaveBeenCalled()
    })

    it('should not reload when version is the same', () => {
      global.localStorageMock.getItem.mockReturnValue('v0.33.0')

      const result = checkVersionChange('v0.33.0')

      expect(result).toBe(false)
      expect(global.localStorageMock.setItem).toHaveBeenCalledWith('flux-operator-version', 'v0.33.0')
      expect(reloadMock).not.toHaveBeenCalled()
    })

    it('should handle undefined version gracefully', () => {
      global.localStorageMock.getItem.mockReturnValue('v0.33.0')

      const result = checkVersionChange(undefined)

      expect(result).toBe(false)
      expect(global.localStorageMock.setItem).not.toHaveBeenCalled()
      expect(reloadMock).not.toHaveBeenCalled()
    })

    it('should handle null version gracefully', () => {
      global.localStorageMock.getItem.mockReturnValue('v0.33.0')

      const result = checkVersionChange(null)

      expect(result).toBe(false)
      expect(global.localStorageMock.setItem).not.toHaveBeenCalled()
      expect(reloadMock).not.toHaveBeenCalled()
    })

    it('should handle empty string version gracefully', () => {
      global.localStorageMock.getItem.mockReturnValue('v0.33.0')

      const result = checkVersionChange('')

      expect(result).toBe(false)
      expect(global.localStorageMock.setItem).not.toHaveBeenCalled()
      expect(reloadMock).not.toHaveBeenCalled()
    })
  })
})
