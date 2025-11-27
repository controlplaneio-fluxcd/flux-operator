// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useLocation } from 'preact-iso'
import { fetchFluxReport } from '../../app'
import { ThemeToggle } from './ThemeToggle'
import { QuickSearch, quickSearchOpen } from '../search/QuickSearch'
import { appliedTheme, themes } from '../../utils/theme'

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
  const location = useLocation()
  const currentPath = location.path

  // Check if we're in a tab view (favorites, events, or resources) or resource dashboard
  const isTabView = currentPath === '/favorites' || currentPath === '/events' || currentPath === '/resources'
  const isResourceDashboard = currentPath.startsWith('/resource/')
  const isNotDashboard = isTabView || isResourceDashboard

  // Use appropriate icon based on theme
  const iconSrc = appliedTheme.value === themes.dark ? '/flux-icon-white.svg' : '/flux-icon-black.svg'

  // Handle navigation button click
  const handleToggle = () => {
    quickSearchOpen.value = false
    if (isTabView) {
      // Return to dashboard from tab view
      location.route('/')
    } else {
      // Navigate to favorites page (default tab view)
      location.route('/favorites')
    }
  }

  // Handle logo/title click
  const handleLogoClick = () => {
    quickSearchOpen.value = false
    if (isNotDashboard) {
      // Return to dashboard if not on main dashboard
      location.route('/')
    } else {
      // Trigger report fetch if in dashboard view
      fetchFluxReport()
    }
  }

  return (
    <header class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 shadow-sm transition-colors">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        {/* Mobile: Full-width search when open */}
        {quickSearchOpen.value && (
          <div class="sm:hidden">
            <QuickSearch />
          </div>
        )}

        {/* Desktop layout (and mobile when search is closed) */}
        <div class={`flex items-center justify-between ${quickSearchOpen.value ? 'hidden sm:flex' : ''}`}>
          {/* Left side: Logo and title (or expanded search on desktop) */}
          <div class="flex items-center gap-4 flex-1 min-w-0">
            {/* Logo - always visible */}
            <button
              onClick={handleLogoClick}
              class="flex items-center space-x-4 hover:opacity-80 transition-opacity focus:outline-none flex-shrink-0"
            >
              <img src={iconSrc} alt="Flux CD" class="w-10 h-10" />
              {/* Title - hidden when search is expanded */}
              {!quickSearchOpen.value && (
                <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Flux Status</h1>
              )}
            </button>

            {/* Quick Search - expands to fill space when open (desktop only) */}
            {quickSearchOpen.value && (
              <div class="flex-1 min-w-0 mr-4">
                <QuickSearch />
              </div>
            )}
          </div>

          {/* Right side buttons */}
          <div class="flex items-center space-x-4 flex-shrink-0">
            {/* Quick Search button - shown when search is closed */}
            {!quickSearchOpen.value && (
              <QuickSearch />
            )}
            {/* Navigation Button */}
            <button
              onClick={handleToggle}
              title={isTabView ? 'Back to Dashboard' : 'Browse Resources'}
              class="inline-flex items-center justify-center p-2 border border-gray-300 dark:border-gray-600 rounded-md text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue"
            >
              {isTabView ? (
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
              ) : (
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
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
