// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import {
  logSettings,
  DEFAULT_LOG_SETTINGS,
  TAIL_LINES,
  FONT_SIZES,
  getLogSettingsFromStorage,
  resetLogSettings
} from './logSettings'

describe('logSettings utilities', () => {
  beforeEach(() => {
    logSettings.value = { ...DEFAULT_LOG_SETTINGS }
    global.localStorageMock.getItem.mockReset()
    global.localStorageMock.setItem.mockClear()
  })

  describe('exports', () => {
    it('defaults to follow on, formatted, 100 lines, medium font', () => {
      expect(DEFAULT_LOG_SETTINGS).toEqual({ follow: true, formatted: true, tail: 100, fontSize: 'md' })
    })

    it('exposes the selectable tail line values', () => {
      expect(TAIL_LINES).toEqual([100, 500, 1000, 2000, 5000])
    })

    it('exposes the selectable font sizes', () => {
      expect(FONT_SIZES.map(f => f.key)).toEqual(['sm', 'md', 'lg'])
      expect(FONT_SIZES.every(f => typeof f.label === 'string' && typeof f.px === 'number')).toBe(true)
    })
  })

  describe('getLogSettingsFromStorage', () => {
    it('returns the defaults when nothing is stored', () => {
      global.localStorageMock.getItem.mockReturnValue(null)
      expect(getLogSettingsFromStorage()).toEqual(DEFAULT_LOG_SETTINGS)
    })

    it('returns a fresh copy of the defaults, not the shared object', () => {
      global.localStorageMock.getItem.mockReturnValue(null)
      expect(getLogSettingsFromStorage()).not.toBe(DEFAULT_LOG_SETTINGS)
    })

    it('reads a valid stored object', () => {
      global.localStorageMock.getItem.mockReturnValue(JSON.stringify({ follow: false, formatted: false, tail: 500, fontSize: 'lg' }))
      expect(getLogSettingsFromStorage()).toEqual({ follow: false, formatted: false, tail: 500, fontSize: 'lg' })
    })

    it('accepts the 2000 tail value', () => {
      global.localStorageMock.getItem.mockReturnValue(JSON.stringify({ follow: true, formatted: true, tail: 2000, fontSize: 'md' }))
      expect(getLogSettingsFromStorage().tail).toBe(2000)
    })

    it('falls back per field, keeping the valid ones, when a field is invalid', () => {
      // tail 250 is not one of TAIL_LINES and fontSize 'xl' is unknown, so both
      // default; follow/formatted kept.
      global.localStorageMock.getItem.mockReturnValue(JSON.stringify({ follow: false, formatted: true, tail: 250, fontSize: 'xl' }))
      expect(getLogSettingsFromStorage()).toEqual({ follow: false, formatted: true, tail: 100, fontSize: 'md' })
    })

    it('defaults a non-boolean follow/formatted', () => {
      global.localStorageMock.getItem.mockReturnValue(JSON.stringify({ follow: 'yes', formatted: 1, tail: 1000, fontSize: 'sm' }))
      expect(getLogSettingsFromStorage()).toEqual({ follow: true, formatted: true, tail: 1000, fontSize: 'sm' })
    })

    it('defaults the missing fields of a partial object', () => {
      global.localStorageMock.getItem.mockReturnValue(JSON.stringify({ formatted: false }))
      expect(getLogSettingsFromStorage()).toEqual({ follow: true, formatted: false, tail: 100, fontSize: 'md' })
    })

    it('drops unknown fields', () => {
      global.localStorageMock.getItem.mockReturnValue(JSON.stringify({ follow: true, formatted: true, tail: 100, fontSize: 'md', extra: 'x' }))
      expect(getLogSettingsFromStorage()).toEqual(DEFAULT_LOG_SETTINGS)
    })

    it('returns the defaults on malformed JSON', () => {
      global.localStorageMock.getItem.mockReturnValue('{not json')
      expect(getLogSettingsFromStorage()).toEqual(DEFAULT_LOG_SETTINGS)
    })
  })

  describe('persistence', () => {
    it('writes the settings to localStorage when they change', () => {
      logSettings.value = { follow: false, formatted: false, tail: 5000, fontSize: 'lg' }
      expect(global.localStorageMock.setItem).toHaveBeenCalledWith(
        'log-viewer',
        JSON.stringify({ follow: false, formatted: false, tail: 5000, fontSize: 'lg' })
      )
    })
  })

  describe('resetLogSettings', () => {
    it('restores the defaults and persists them', () => {
      logSettings.value = { follow: false, formatted: false, tail: 500 }
      resetLogSettings()
      expect(logSettings.value).toEqual(DEFAULT_LOG_SETTINGS)
      expect(global.localStorageMock.setItem).toHaveBeenLastCalledWith(
        'log-viewer',
        JSON.stringify(DEFAULT_LOG_SETTINGS)
      )
    })
  })
})
