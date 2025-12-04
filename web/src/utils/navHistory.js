// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { signal, effect } from '@preact/signals'

// LocalStorage key for navigation history
const STORAGE_KEY = 'nav-history'

// Maximum number of entries in navigation history
const MAX_HISTORY_SIZE = 5

/**
 * Get navigation history from localStorage
 * @returns {Array} Array of history objects [{kind, namespace, name}, ...]
 */
const getNavHistoryFromStorage = () => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored ? JSON.parse(stored) : []
  } catch {
    return []
  }
}

/**
 * Generate a unique key for a history entry
 * @param {string} kind - Resource kind
 * @param {string} namespace - Resource namespace
 * @param {string} name - Resource name
 * @returns {string} Unique key in format "kind/namespace/name"
 */
export function getNavHistoryKey(kind, namespace, name) {
  return `${kind}/${namespace}/${name}`
}

/**
 * Check if an entry represents the home page (FluxReport)
 * @param {string} kind - Resource kind
 * @returns {boolean} True if entry is the home page
 */
export function isHomePage(kind) {
  return kind === 'FluxReport'
}

// Reactive signal for navigation history list
// New entries are added to the beginning of the array (most recent first)
export const navHistory = signal(getNavHistoryFromStorage())

// Sync navigation history to localStorage whenever it changes
effect(() => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(navHistory.value))
})

/**
 * Add a resource to navigation history (at the beginning of the list)
 * If the resource already exists, it is moved to the top
 * History is limited to MAX_HISTORY_SIZE entries
 * @param {string} kind - Resource kind
 * @param {string} namespace - Resource namespace
 * @param {string} name - Resource name
 */
export function addToNavHistory(kind, namespace, name) {
  const key = getNavHistoryKey(kind, namespace, name)

  // Remove existing entry if present (to move it to top)
  const filtered = navHistory.value.filter(
    entry => getNavHistoryKey(entry.kind, entry.namespace, entry.name) !== key
  )

  // Add new entry to beginning and limit size
  navHistory.value = [{ kind, namespace, name }, ...filtered].slice(0, MAX_HISTORY_SIZE)
}

/**
 * Clear all navigation history
 */
export function clearNavHistory() {
  navHistory.value = []
}
