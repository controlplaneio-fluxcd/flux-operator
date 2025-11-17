// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { showSearchView, fetchFluxReport } from '../app'
import { ThemeToggle } from './ThemeToggle'
import { appliedTheme, themes } from '../utils/theme'

/**
 * Header component - Main application header with navigation and controls
 *
 * Features:
 * - Flux branding with theme-aware icon
 * - Toggle button to switch between Dashboard and Search views
 * - Refresh button to manually fetch latest data
 * - Theme toggle control
 * - Responsive design
 */
export function Header() {
  // Use appropriate icon based on theme
  const iconSrc = appliedTheme.value === themes.dark ? '/flux-icon-white.svg' : '/flux-icon-black.svg'

  // Handle navigation button click
  const handleToggle = () => {
    // Toggle between search view and dashboard
    showSearchView.value = !showSearchView.value
  }

  // Handle logo/title click
  const handleLogoClick = () => {
    if (showSearchView.value) {
      // Return to dashboard if in search view
      showSearchView.value = false
    } else {
      // Trigger report fetch if in dashboard view
      fetchFluxReport()
    }
  }

  return (
    <header class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 shadow-sm transition-colors">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <div class="flex items-center justify-between">
          <button
            onClick={handleLogoClick}
            class="flex items-center space-x-4 hover:opacity-80 transition-opacity focus:outline-none"
          >
            <img src={iconSrc} alt="Flux CD" class="w-10 h-10" />
            <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Flux Status</h1>
          </button>
          <div class="flex items-center space-x-4">
            {/* Navigation Button */}
            <button
              onClick={handleToggle}
              class="inline-flex items-center justify-center p-2 border border-gray-300 dark:border-gray-600 rounded-md text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue"
            >
              {showSearchView.value ? (
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
              ) : (
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              )}
            </button>

            <div class="hidden md:block">
              <ThemeToggle />
            </div>
          </div>
        </div>
      </div>
    </header>
  )
}
