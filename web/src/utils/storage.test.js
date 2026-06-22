// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { writeLocalStorage, writeSessionStorage } from './storage'

describe('storage write guards', () => {
  beforeEach(() => {
    global.localStorageMock.setItem.mockReset()
    window.sessionStorage.clear()
  })

  describe('writeLocalStorage', () => {
    it('writes through to localStorage.setItem', () => {
      writeLocalStorage('k', 'v')
      expect(global.localStorageMock.setItem).toHaveBeenCalledWith('k', 'v')
    })

    it('swallows a throwing setItem (Safari private mode / quota)', () => {
      global.localStorageMock.setItem.mockImplementation(() => {
        throw new Error('QuotaExceededError')
      })
      expect(() => writeLocalStorage('k', 'v')).not.toThrow()
    })
  })

  describe('writeSessionStorage', () => {
    it('writes through to sessionStorage', () => {
      writeSessionStorage('k', 'v')
      expect(window.sessionStorage.getItem('k')).toBe('v')
    })

    it('swallows a throwing setItem (Safari private mode / quota)', () => {
      const original = window.sessionStorage
      Object.defineProperty(window, 'sessionStorage', {
        value: { setItem: () => { throw new Error('QuotaExceededError') } },
        configurable: true,
        writable: true
      })
      try {
        expect(() => writeSessionStorage('k', 'v')).not.toThrow()
      } finally {
        Object.defineProperty(window, 'sessionStorage', {
          value: original,
          configurable: true,
          writable: true
        })
      }
    })
  })
})
