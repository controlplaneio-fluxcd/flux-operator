// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect, useRef } from 'preact/hooks'
import { useLocation } from 'preact-iso'

/**
 * Serializes filter object to URL query string
 * Omits empty/null/undefined values
 *
 * @param {Object} filters - Filter key-value pairs
 * @returns {string} URL query string (without leading '?')
 *
 * @example
 * serializeFilters({ kind: 'GitRepository', namespace: '', name: 'flux' })
 * // Returns: 'kind=GitRepository&name=flux'
 */
export function serializeFilters(filters) {
  const params = new URLSearchParams()

  for (const [key, value] of Object.entries(filters)) {
    if (value && value !== '') {
      params.append(key, value)
    }
  }

  return params.toString()
}

/**
 * Parses query object from URL into filter values
 * Returns empty string for missing parameters
 *
 * @param {Object} query - Query object from useLocation().query
 * @param {Array<string>} filterKeys - Array of expected filter keys
 * @returns {Object} Filter object with all keys (empty string if not present)
 *
 * @example
 * parseFilters({ kind: 'GitRepository' }, ['kind', 'namespace', 'name'])
 * // Returns: { kind: 'GitRepository', namespace: '', name: '' }
 */
export function parseFilters(query, filterKeys) {
  const filters = {}

  for (const key of filterKeys) {
    filters[key] = query[key] || ''
  }

  return filters
}

/**
 * Custom hook that restores filter signals from URL query params on mount
 * Runs once on component mount, reads URL and sets signal values
 *
 * @param {Object} filterSignals - Object mapping filter names to signals
 *   Example: { kind: selectedEventsKind, name: selectedEventsName }
 *
 * @example
 * useRestoreFiltersFromUrl({
 *   kind: selectedEventsKind,
 *   name: selectedEventsName,
 *   namespace: selectedEventsNamespace,
 *   type: selectedEventsSeverity
 * })
 */
export function useRestoreFiltersFromUrl(filterSignals) {
  const location = useLocation()
  const restored = useRef(false)

  useEffect(() => {
    // Only restore once on mount
    if (restored.current) return

    const query = location.query || {}

    // Set each signal from query params
    for (const [key, signal] of Object.entries(filterSignals)) {
      signal.value = query[key] || ''
    }

    restored.current = true
  }, []) // Empty deps - run once on mount
}

/**
 * Custom hook that syncs filter signals to URL query params
 * Updates URL when any filter signal changes (debounced to avoid excessive history entries)
 *
 * @param {Object} filterSignals - Object mapping filter names to signals
 * @param {number} debounceMs - Debounce delay in milliseconds (default: 300)
 *
 * @example
 * useSyncFiltersToUrl({
 *   kind: selectedEventsKind,
 *   name: selectedEventsName,
 *   namespace: selectedEventsNamespace,
 *   type: selectedEventsSeverity
 * }, 300)
 */
export function useSyncFiltersToUrl(filterSignals, debounceMs = 300) {
  const location = useLocation()
  const timeoutRef = useRef(null)
  const isRestoringRef = useRef(true)

  useEffect(() => {
    // Skip first run (let useRestoreFiltersFromUrl handle initial state)
    if (isRestoringRef.current) {
      isRestoringRef.current = false
      return
    }

    // Clear existing timeout
    if (timeoutRef.current) {
      window.clearTimeout(timeoutRef.current)
    }

    // Debounce URL updates
    timeoutRef.current = setTimeout(() => {
      // Build filter object from current signal values
      const filters = {}
      for (const [key, signal] of Object.entries(filterSignals)) {
        filters[key] = signal.value
      }

      // Serialize to query string
      const queryString = serializeFilters(filters)

      // Get current path
      const currentPath = location.path

      // Build new URL
      const newUrl = queryString ? `${currentPath}?${queryString}` : currentPath

      // Only update if URL actually changed
      // Use window.location instead of preact-iso location for accurate comparison
      const currentUrl = `${window.location.pathname}${window.location.search}`

      if (newUrl !== currentUrl) {
        // Use replaceState to avoid cluttering history with every keystroke
        window.history.replaceState(null, '', newUrl)
      }
    }, debounceMs)

    // Cleanup timeout on unmount
    return () => {
      if (timeoutRef.current) {
        window.clearTimeout(timeoutRef.current)
      }
    }
  }, [
    // Watch all signal values
    ...Object.values(filterSignals).map(signal => signal.value),
    location.path,
    debounceMs
  ])
}
