// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import {
  getStoredVersion,
  checkVersionChange,
  getOrCreateUUID,
  getStoredUpdateInfo,
  checkForUpdates,
  updateInfo
} from './version'

describe('version utilities', () => {
  let reloadMock
  let fetchMock

  beforeEach(() => {
    // Reset localStorage mock
    global.localStorageMock.getItem.mockClear()
    global.localStorageMock.setItem.mockClear()
    global.localStorageMock.clear()

    // Reset crypto mock
    global.cryptoMock.randomUUID.mockClear()
    global.cryptoMock.randomUUID.mockReturnValue('test-uuid-1234')

    // Mock window.location.reload
    reloadMock = vi.fn()
    Object.defineProperty(window, 'location', {
      value: { reload: reloadMock },
      writable: true
    })

    // Mock fetch
    fetchMock = vi.fn()
    global.fetch = fetchMock

    // Reset updateInfo signal
    updateInfo.value = null
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

  describe('getOrCreateUUID', () => {
    it('should return existing UUID from localStorage', () => {
      global.localStorageMock.getItem.mockReturnValue('existing-uuid-5678')

      const result = getOrCreateUUID()

      expect(result).toBe('existing-uuid-5678')
      expect(global.localStorageMock.getItem).toHaveBeenCalledWith('flux-operator-uuid')
      expect(global.cryptoMock.randomUUID).not.toHaveBeenCalled()
      expect(global.localStorageMock.setItem).not.toHaveBeenCalled()
    })

    it('should generate and store new UUID when not present', () => {
      global.localStorageMock.getItem.mockReturnValue(null)

      const result = getOrCreateUUID()

      expect(result).toBe('test-uuid-1234')
      expect(global.cryptoMock.randomUUID).toHaveBeenCalled()
      expect(global.localStorageMock.setItem).toHaveBeenCalledWith('flux-operator-uuid', 'test-uuid-1234')
    })
  })

  describe('getStoredUpdateInfo', () => {
    it('should return null when no update info is stored', () => {
      global.localStorageMock.getItem.mockReturnValue(null)

      const result = getStoredUpdateInfo()

      expect(result).toBeNull()
      expect(global.localStorageMock.getItem).toHaveBeenCalledWith('flux-operator-update-info')
    })

    it('should return parsed update info when present', () => {
      const updateData = { latest: 'v0.35.0', current: 'v0.34.0', isOutdated: true }
      global.localStorageMock.getItem.mockReturnValue(JSON.stringify(updateData))

      const result = getStoredUpdateInfo()

      expect(result).toEqual(updateData)
    })

    it('should return null when JSON is invalid', () => {
      global.localStorageMock.getItem.mockReturnValue('invalid-json')

      const result = getStoredUpdateInfo()

      expect(result).toBeNull()
    })
  })

  describe('checkForUpdates', () => {
    const productionEnv = { MODE: 'production' }
    const developmentEnv = { MODE: 'development' }

    it('should skip API call in non-production mode', async () => {
      await checkForUpdates('v0.34.0', 'v2.7.0', developmentEnv)

      expect(fetchMock).not.toHaveBeenCalled()
    })

    it('should make POST request with correct payload', async () => {
      global.localStorageMock.getItem.mockReturnValue('existing-uuid')
      fetchMock.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ latest: 'v0.35.0', current: 'v0.34.0', isOutdated: true })
      })

      await checkForUpdates('v0.34.0', 'v2.7.0', productionEnv)

      expect(fetchMock).toHaveBeenCalledWith(
        'https://fluxoperator.dev/api/v1/version',
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ version: 'v0.34.0', flux_version: 'v2.7.0', uuid: 'existing-uuid' })
        }
      )
    })

    it('should store successful response in localStorage and signal', async () => {
      global.localStorageMock.getItem.mockReturnValue('existing-uuid')
      const responseData = { latest: 'v0.35.0', current: 'v0.34.0', isOutdated: true }
      fetchMock.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(responseData)
      })

      await checkForUpdates('v0.34.0', 'v2.7.0', productionEnv)

      expect(global.localStorageMock.setItem).toHaveBeenCalledWith(
        'flux-operator-update-info',
        JSON.stringify(responseData)
      )
      expect(updateInfo.value).toEqual(responseData)
    })

    it('should silently ignore HTTP errors', async () => {
      global.localStorageMock.getItem.mockReturnValue('existing-uuid')
      fetchMock.mockResolvedValue({ ok: false, status: 502 })

      // Should not throw
      await checkForUpdates('v0.34.0', 'v2.7.0', productionEnv)

      expect(updateInfo.value).toBeNull()
      expect(global.localStorageMock.setItem).not.toHaveBeenCalledWith(
        'flux-operator-update-info',
        expect.anything()
      )
    })

    it('should silently ignore network errors', async () => {
      global.localStorageMock.getItem.mockReturnValue('existing-uuid')
      fetchMock.mockRejectedValue(new Error('Network error'))

      // Should not throw
      await checkForUpdates('v0.34.0', 'v2.7.0', productionEnv)

      expect(updateInfo.value).toBeNull()
    })
  })
})
