// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { themeMode, appliedTheme, cycleTheme, themes } from '../utils/theme'

/**
 * ThemeToggle component - Button to cycle through theme modes
 *
 * Features:
 * - Cycles through three theme modes: Light → Dark → Auto
 * - Displays appropriate icon for current theme (sun, moon, lightbulb)
 * - Shows current theme label
 * - Tooltip with theme information
 * - Persists theme preference to localStorage
 */
export function ThemeToggle() {
  const getIcon = () => {
    if (themeMode.value === themes.auto) {
      return (
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
        </svg>
      )
    } else if (appliedTheme.value === themes.dark) {
      return (
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
        </svg>
      )
    } else {
      return (
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
        </svg>
      )
    }
  }

  const getLabel = () => {
    if (themeMode.value === themes.auto) return 'Auto'
    if (themeMode.value === themes.dark) return 'Dark'
    return 'Light'
  }

  return (
    <button
      onClick={cycleTheme}
      class="flex items-center space-x-2 px-3 py-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
      title={`Theme: ${getLabel()}`}
    >
      {getIcon()}
      <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{getLabel()}</span>
    </button>
  )
}
