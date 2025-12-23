// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useCallback } from 'preact/hooks'

/**
 * Parses a URL hash fragment into panel and tab parts
 * Format: #panel-tab (e.g., #reconciler-events, #inventory-graph)
 *
 * @param {string} hash - The hash string (with or without leading #)
 * @returns {{ panel: string, tab: string } | null} Parsed parts or null if invalid
 */
export function parseHash(hash) {
  if (!hash) return null

  // Remove leading # if present
  const cleanHash = hash.startsWith('#') ? hash.slice(1) : hash

  if (!cleanHash) return null

  // Split on first hyphen only (tab names might contain hyphens)
  const hyphenIndex = cleanHash.indexOf('-')
  if (hyphenIndex === -1) return null

  const panel = cleanHash.slice(0, hyphenIndex)
  const tab = cleanHash.slice(hyphenIndex + 1)

  if (!panel || !tab) return null

  return { panel, tab }
}

/**
 * Builds a hash string from panel and tab
 *
 * @param {string} panel - Panel identifier (e.g., 'reconciler', 'inventory')
 * @param {string} tab - Tab identifier (e.g., 'events', 'graph')
 * @returns {string} Hash string with leading # (e.g., '#reconciler-events')
 */
export function buildHash(panel, tab) {
  return `#${panel}-${tab}`
}

/**
 * Custom hook for syncing tab state with URL hash fragment
 *
 * Features:
 * - Reads initial tab from URL hash on mount
 * - Updates URL hash when tab changes (using replaceState, not pushState)
 * - Listens for hashchange events (browser back/forward, manual URL edit)
 * - Only responds to hash changes matching the panel prefix
 * - Optionally scrolls the panel element into view when hash matches on load
 *
 * @param {string} panel - Panel identifier (e.g., 'reconciler', 'inventory')
 * @param {string} defaultTab - Default tab when hash doesn't match this panel
 * @param {string[]} validTabs - Array of valid tab names for this panel
 * @param {string} [scrollElementId] - Optional element ID to scroll into view when hash matches
 * @returns {[string, function]} Tuple of [activeTab, setActiveTab]
 *
 * @example
 * const [activeTab, setActiveTab] = useHashTab('reconciler', 'overview', ['overview', 'history', 'events', 'spec', 'status'], 'reconciler-panel')
 * // URL: /resource/GitRepo/flux-system/repo#reconciler-events
 * // activeTab = 'events', and the reconciler-panel element scrolls into view
 */
export function useHashTab(panel, defaultTab, validTabs, scrollElementId) {
  // Initialize state from current hash or default
  const getInitialTab = () => {
    if (typeof window === 'undefined') return defaultTab

    const parsed = parseHash(window.location.hash)
    if (parsed && parsed.panel === panel && validTabs.includes(parsed.tab)) {
      return parsed.tab
    }
    return defaultTab
  }

  const [activeTab, setActiveTabState] = useState(getInitialTab)

  // Setter that also updates the URL hash
  const setActiveTab = useCallback((newTab) => {
    setActiveTabState(newTab)

    if (typeof window === 'undefined') return

    // Build new URL with hash
    const newHash = buildHash(panel, newTab)
    const currentUrl = window.location.pathname + window.location.search

    // Use replaceState to avoid cluttering browser history
    window.history.replaceState(null, '', currentUrl + newHash)
  }, [panel])

  // Listen for hashchange events (back/forward, manual URL edit)
  useEffect(() => {
    if (typeof window === 'undefined') return

    const handleHashChange = () => {
      const parsed = parseHash(window.location.hash)

      if (parsed && parsed.panel === panel && validTabs.includes(parsed.tab)) {
        // Hash matches our panel - update tab
        setActiveTabState(parsed.tab)
      }
      // If hash doesn't match our panel, keep current tab (don't reset to default)
    }

    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [panel, validTabs])

  // Sync with hash on mount and when panel/validTabs change
  // This handles navigation to a different resource that has the same panel
  useEffect(() => {
    if (typeof window === 'undefined') return

    const parsed = parseHash(window.location.hash)
    if (parsed && parsed.panel === panel && validTabs.includes(parsed.tab)) {
      setActiveTabState(parsed.tab)

      // Scroll the panel element into view if scrollElementId is provided
      if (scrollElementId) {
        // Use requestAnimationFrame to ensure DOM is ready
        window.requestAnimationFrame(() => {
          const element = document.getElementById(scrollElementId)
          if (element) {
            element.scrollIntoView({ behavior: 'smooth', block: 'start' })
          }
        })
      }
    } else {
      // Reset to default if hash doesn't match this panel
      setActiveTabState(defaultTab)
    }
  }, [panel, defaultTab, validTabs, scrollElementId])

  return [activeTab, setActiveTab]
}

/**
 * Clears the URL hash fragment
 * Useful when navigating away from a resource page
 */
export function clearHash() {
  if (typeof window === 'undefined') return

  const currentUrl = window.location.pathname + window.location.search
  window.history.replaceState(null, '', currentUrl)
}
