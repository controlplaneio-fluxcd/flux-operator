// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useCallback, useRef } from 'preact/hooks'

/** @typedef {{ getValidTabs: () => string[], getActiveTab: () => string, setActiveTab: (tab: string) => void }} HashPanelHandlers */

const hashPanelRegistry = new Map()

/**
 * Registers a hash-synced panel for keyboard tab cycling.
 *
 * @param {string} panel
 * @param {HashPanelHandlers} handlers
 */
export function registerHashPanel(panel, handlers) {
  hashPanelRegistry.set(panel, handlers)
}

/**
 * Unregisters a hash-synced panel.
 *
 * @param {string} panel
 */
export function unregisterHashPanel(panel) {
  hashPanelRegistry.delete(panel)
}

/** Clears all registered hash panels (for tests). */
export function clearHashPanelRegistry() {
  hashPanelRegistry.clear()
}

/**
 * Cycles the active tab in the current or first visible hash panel.
 *
 * @param {number} direction - -1 for previous, 1 for next
 * @returns {boolean} True when a tab was cycled
 */
export function cycleHashTab(direction) {
  const parsed = parseHash(window.location.hash)
  let panel = parsed?.panel
  let entry = panel ? hashPanelRegistry.get(panel) : null

  if (!entry || entry.getValidTabs().length === 0) {
    for (const [name, handlers] of hashPanelRegistry) {
      if (handlers.getValidTabs().length > 0) {
        panel = name
        entry = handlers
        break
      }
    }
  }

  if (!entry) {
    return false
  }

  const validTabs = entry.getValidTabs()
  if (validTabs.length === 0) {
    return false
  }

  const { getActiveTab, setActiveTab } = entry
  const hashTab = parsed?.panel === panel ? parsed.tab : null
  const activeTab = hashTab && validTabs.includes(hashTab) ? hashTab : getActiveTab()
  const idx = validTabs.indexOf(activeTab)
  const currentIdx = idx === -1 ? 0 : idx
  const nextIdx = (currentIdx + direction + validTabs.length) % validTabs.length
  setActiveTab(validTabs[nextIdx])
  return true
}

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
 * - Optionally scrolls the panel element into view on mount and hash navigation
 *   (not when validTabs identity alone changes)
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
  const validTabsRef = useRef(validTabs)
  validTabsRef.current = validTabs

  // Setter that also updates the URL hash
  const setActiveTab = useCallback((newTab) => {
    if (!validTabsRef.current.includes(newTab)) {
      return
    }
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

      if (parsed && parsed.panel === panel && validTabsRef.current.includes(parsed.tab)) {
        // Hash matches our panel - update tab
        setActiveTabState(parsed.tab)
      }
      // If hash doesn't match our panel, keep current tab (don't reset to default)
    }

    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [panel])

  // Sync tab state with hash on mount and when panel/validTabs change.
  // Scroll is intentionally separate so poll-driven validTabs identity changes
  // do not yank the viewport back to the hashed panel.
  useEffect(() => {
    if (typeof window === 'undefined') return

    const parsed = parseHash(window.location.hash)
    if (parsed && parsed.panel === panel && validTabs.includes(parsed.tab)) {
      setActiveTabState(parsed.tab)
    } else {
      // Reset to default if hash doesn't match this panel
      setActiveTabState(defaultTab)
    }
  }, [panel, defaultTab, validTabs])

  // Scroll into view only on initial mount and actual hash navigation.
  useEffect(() => {
    if (typeof window === 'undefined' || !scrollElementId) return

    const scrollIfHashMatches = () => {
      const parsed = parseHash(window.location.hash)
      if (!parsed || parsed.panel !== panel || !validTabsRef.current.includes(parsed.tab)) {
        return
      }
      window.requestAnimationFrame(() => {
        const element = document.getElementById(scrollElementId)
        if (element) {
          element.scrollIntoView({ behavior: 'smooth', block: 'start' })
        }
      })
    }

    scrollIfHashMatches()
    window.addEventListener('hashchange', scrollIfHashMatches)
    return () => window.removeEventListener('hashchange', scrollIfHashMatches)
  }, [panel, scrollElementId])

  const activeTabRef = useRef(activeTab)
  activeTabRef.current = activeTab

  useEffect(() => {
    const tabs = validTabsRef.current
    if (tabs.length === 0) {
      unregisterHashPanel(panel)
      return
    }

    if (!tabs.includes(activeTabRef.current)) {
      const fallback = tabs.includes(defaultTab) ? defaultTab : tabs[0]
      setActiveTab(fallback)
    }
  }, [validTabs, defaultTab, panel, setActiveTab])

  useEffect(() => {
    if (validTabsRef.current.length === 0) {
      return
    }

    registerHashPanel(panel, {
      getValidTabs: () => validTabsRef.current,
      getActiveTab: () => activeTabRef.current,
      setActiveTab,
    })
    return () => unregisterHashPanel(panel)
  }, [panel, validTabs, setActiveTab])

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
