// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect, useRef } from 'preact/hooks'
import { useLocation } from 'preact-iso'
import { parseDetailRoute } from './routing'
import { cycleHashTab } from './hash'
import {
  keyboardShortcutsOpen,
  G_CHORD_TIMEOUT_MS,
  NAV_ROUTES,
  shouldIgnoreShortcut,
  closeOverlays,
} from './keyboardShortcuts'
import { refreshCurrentView, toggleFavoriteFromShortcut, copyLinkFromShortcut, openLogsFromShortcut, cycleSectionTab, SECTION_TAB_PATHS } from './keyboardShortcutActions'

/**
 * useKeyboardShortcuts - registers global keyboard shortcuts for the Flux Status Page.
 *
 * Phase 1: `?` help modal toggle, `g`-chord view navigation.
 * Phase 2: `Shift+R` refresh, `s` favorite, `c` copy link, `l` open logs.
 * Phase 3: `[` / `]` tab cycling on section and detail views.
 */
export function useKeyboardShortcuts() {
  const location = useLocation()
  const pendingG = useRef(false)
  const chordTimer = useRef(null)

  useEffect(() => {
    const clearChord = () => {
      pendingG.current = false
      if (chordTimer.current) {
        clearTimeout(chordTimer.current)
        chordTimer.current = null
      }
    }

    const handleKeyDown = (e) => {
      if (e.key === '?' || (e.key === '/' && e.shiftKey)) {
        if (!shouldIgnoreShortcut(e) || keyboardShortcutsOpen.value) {
          e.preventDefault()
          keyboardShortcutsOpen.value = !keyboardShortcutsOpen.value
        }
        return
      }

      if (keyboardShortcutsOpen.value) {
        return
      }

      if (e.key === 'g' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        if (shouldIgnoreShortcut(e, { allowOverlays: true })) {
          return
        }
        e.preventDefault()
        closeOverlays()
        clearChord()
        pendingG.current = true
        chordTimer.current = setTimeout(clearChord, G_CHORD_TIMEOUT_MS)
        return
      }

      if (pendingG.current) {
        const route = NAV_ROUTES[e.key]
        if (route) {
          e.preventDefault()
          clearChord()
          closeOverlays()
          location.route(route)
        } else {
          clearChord()
        }
        return
      }

      if (shouldIgnoreShortcut(e)) {
        return
      }

      if (e.key === 'R' && e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault()
        void refreshCurrentView(location.path).catch(() => {})
        return
      }

      if (e.key === 's' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        if (toggleFavoriteFromShortcut(location.path)) {
          e.preventDefault()
        }
        return
      }

      if (e.key === 'c' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        if (parseDetailRoute(location.path)) {
          e.preventDefault()
          void copyLinkFromShortcut(location.path)
        }
        return
      }

      if (e.key === 'l' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        if (openLogsFromShortcut(location.path)) {
          e.preventDefault()
        }
        return
      }

      // Allow Option/AltGr (needed to type [ ] on DE and other layouts). Still
      // block Cmd+[ (browser back/forward) and Ctrl+[ without Alt.
      if ((e.key === '[' || e.key === ']') && !e.metaKey && !(e.ctrlKey && !e.altKey)) {
        const direction = e.key === ']' ? 1 : -1
        let handled = false
        if (SECTION_TAB_PATHS.includes(location.path)) {
          handled = cycleSectionTab(location.path, direction, location.route)
        } else if (parseDetailRoute(location.path)) {
          handled = cycleHashTab(direction)
        }
        if (handled) {
          e.preventDefault()
        }
        return
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      clearChord()
    }
  }, [location])
}
