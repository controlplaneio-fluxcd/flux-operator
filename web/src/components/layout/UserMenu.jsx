// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useRef, useEffect } from 'preact/hooks'
import { signal } from '@preact/signals'
import { themeMode, appliedTheme, cycleTheme, themes } from '../../utils/theme'
import { clearFavorites } from '../../utils/favorites'
import { clearNavHistory } from '../../utils/navHistory'
import { reportData } from '../../app'

// Exported signal to track menu open state
export const userMenuOpen = signal(false)

/**
 * UserMenu component - User dropdown menu in the header
 *
 * Features:
 * - User icon button that toggles a dropdown menu
 * - Displays username and role information
 * - Theme toggle control
 * - Link to provide feedback
 * - Clear local storage action (with confirmation)
 * - Click-outside handling to close dropdown
 */
export function UserMenu() {
  const menuRef = useRef(null)

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event) => {
      if (menuRef.current && !menuRef.current.contains(event.target)) {
        userMenuOpen.value = false
      }
    }

    if (userMenuOpen.value) {
      document.addEventListener('mousedown', handleClickOutside)
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [userMenuOpen.value])

  // Close dropdown on escape key
  useEffect(() => {
    const handleEscape = (event) => {
      if (event.key === 'Escape') {
        userMenuOpen.value = false
      }
    }

    if (userMenuOpen.value) {
      document.addEventListener('keydown', handleEscape)
    }

    return () => {
      document.removeEventListener('keydown', handleEscape)
    }
  }, [userMenuOpen.value])

  const getThemeIcon = () => {
    if (themeMode.value === themes.auto) {
      return (
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
        </svg>
      )
    } else if (appliedTheme.value === themes.dark) {
      return (
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
        </svg>
      )
    } else {
      return (
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
        </svg>
      )
    }
  }

  const getThemeLabel = () => {
    if (themeMode.value === themes.auto) return 'Auto'
    if (themeMode.value === themes.dark) return 'Dark'
    return 'Light'
  }

  const handleThemeToggle = () => {
    cycleTheme()
  }

  const handleClearLocalStorage = () => {
    if (window.confirm('This will delete your favorites and navigation history from local storage. Continue?')) {
      clearFavorites()
      clearNavHistory()
      userMenuOpen.value = false
    }
  }

  return (
    <div class="relative" ref={menuRef}>
      {/* User button */}
      <button
        onClick={() => userMenuOpen.value = !userMenuOpen.value}
        title="User menu"
        aria-label="User menu"
        aria-expanded={userMenuOpen.value}
        aria-haspopup="true"
        class="inline-flex items-center justify-center p-1.5 border border-gray-300 dark:border-gray-600 rounded-md text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue"
      >
        {/* User icon */}
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
        </svg>
      </button>

      {/* Dropdown menu */}
      {userMenuOpen.value && (
        <div class="fixed inset-0 sm:absolute sm:inset-auto sm:right-0 sm:mt-2 sm:w-56 sm:rounded-lg bg-white dark:bg-gray-800 shadow-lg sm:border border-gray-200 dark:border-gray-700 py-1 z-50">
          {/* User info - Avatar, Username and Role, with close button on mobile */}
          <div class="px-4 py-3 flex items-center gap-3">
            {/* User avatar in circle */}
            <div class="w-8 h-8 rounded-full bg-gray-200 dark:bg-gray-600 flex items-center justify-center flex-shrink-0">
              <svg class="w-5 h-5 text-gray-500 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
              </svg>
            </div>
            <div class="flex flex-col min-w-0 flex-1">
              <span class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">{reportData.value?.spec?.userInfo?.username || 'anonymous'}</span>
              <span class="text-xs text-gray-500 dark:text-gray-400 truncate">{reportData.value?.spec?.userInfo?.role || 'unknown'}</span>
            </div>
            {/* Mobile close button */}
            <button
              onClick={() => userMenuOpen.value = false}
              class="sm:hidden p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
              aria-label="Close menu"
            >
              <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* Separator */}
          <div class="my-1 border-t border-gray-200 dark:border-gray-700" />

          {/* Theme toggle */}
          <button
            onClick={handleThemeToggle}
            class="w-full px-4 py-2 flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          >
            <span class="text-gray-500 dark:text-gray-400">
              {getThemeIcon()}
            </span>
            <span>Theme: {getThemeLabel()}</span>
          </button>

          {/* Separator */}
          <div class="my-1 border-t border-gray-200 dark:border-gray-700" />

          {/* Provide feedback */}
          <a
            href="https://github.com/controlplaneio-fluxcd/flux-operator/issues/new?title=[status-page]"
            target="_blank"
            rel="noopener noreferrer"
            class="px-4 py-2 flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            onClick={() => userMenuOpen.value = false}
          >
            <svg class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            <span>Provide feedback</span>
          </a>

          {/* Contact support */}
          <a
            href="mailto:flux-enterprise@control-plane.io"
            class="px-4 py-2 flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            onClick={() => userMenuOpen.value = false}
          >
            <svg class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            <span>Contact support</span>
          </a>

          {/* Separator */}
          <div class="my-1 border-t border-gray-200 dark:border-gray-700" />

          {/* Clear local storage */}
          <button
            onClick={handleClearLocalStorage}
            class="w-full px-4 py-2 flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          >
            <svg class="w-4 h-4 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            <span>Clear local storage</span>
          </button>
        </div>
      )}
    </div>
  )
}
