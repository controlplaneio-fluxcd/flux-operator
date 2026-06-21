// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal, effect } from '@preact/signals'
import { writeLocalStorage } from './storage'

// LocalStorage key holding the log viewer settings as a single JSON object.
const STORAGE_KEY = 'log-viewer'

// Selectable values for the number of log lines to fetch from the backend, used
// both to render the toggle group and to validate a persisted `tail` value. The
// maximum matches the backend's maxLogTailLines cap and the viewer's
// MAX_BUFFER_LINES.
export const TAIL_LINES = [100, 500, 1000, 2000, 5000]

// Selectable monospace font sizes for the log body. `key` is persisted, `px` is
// applied via the --logs-font-size CSS variable on the log body container.
export const FONT_SIZES = [
  { key: 'sm', label: 'Small', px: 11 },
  { key: 'md', label: 'Medium', px: 12 },
  { key: 'lg', label: 'Large', px: 14 }
]
const FONT_SIZE_KEYS = FONT_SIZES.map((f) => f.key)

// Default log viewer settings, used on first load and as the per-field fallback
// when a stored value is missing or invalid.
export const DEFAULT_LOG_SETTINGS = { follow: true, formatted: true, tail: 100, fontSize: 'md' }

/**
 * Read the log viewer settings from localStorage, validating each field and
 * falling back to its default when missing or invalid. A partial or older stored
 * object still loads (unknown fields are dropped, missing ones defaulted), so the
 * shape can evolve without a version flag.
 *
 * @returns {{follow: boolean, formatted: boolean, tail: number, fontSize: string}} The settings
 */
export const getLogSettingsFromStorage = () => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (!stored) return { ...DEFAULT_LOG_SETTINGS }
    const o = JSON.parse(stored)
    return {
      follow: typeof o.follow === 'boolean' ? o.follow : DEFAULT_LOG_SETTINGS.follow,
      formatted: typeof o.formatted === 'boolean' ? o.formatted : DEFAULT_LOG_SETTINGS.formatted,
      tail: TAIL_LINES.includes(o.tail) ? o.tail : DEFAULT_LOG_SETTINGS.tail,
      fontSize: FONT_SIZE_KEYS.includes(o.fontSize) ? o.fontSize : DEFAULT_LOG_SETTINGS.fontSize
    }
  } catch {
    return { ...DEFAULT_LOG_SETTINGS }
  }
}

// Reactive signal for the log viewer settings, seeded from localStorage.
export const logSettings = signal(getLogSettingsFromStorage())

// Persist the settings to localStorage whenever they change. Only the known keys
// are written (the signal never carries unknown fields), so the stored object
// stays clean.
effect(() => {
  writeLocalStorage(STORAGE_KEY, JSON.stringify(logSettings.value))
})

/**
 * Reset the log viewer settings to their defaults. The change persists via the
 * effect above. An already-open viewer is unaffected — it seeds its state once on
 * mount (via peek, no subscription) — so the reset applies the next time the
 * viewer is opened. Used by the "Clear local storage" action.
 */
export function resetLogSettings() {
  logSettings.value = { ...DEFAULT_LOG_SETTINGS }
}
