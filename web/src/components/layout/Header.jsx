// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useLocation } from 'preact-iso'
import { fetchFluxReport } from '../../app'
import { UserMenu } from './UserMenu'
import { QuickSearch, quickSearchOpen } from '../search/QuickSearch'
import { FluxOperatorIcon } from './Icons'

/**
 * Header component - Main application header with navigation and controls
 *
 * Features:
 * - Flux branding with theme-aware icon
 * - Browse resources button to navigate to resources view
 * - Refresh button to manually fetch latest data
 * - Theme toggle control
 * - Responsive design
 */
export function Header() {
  const location = useLocation()
  const currentPath = location.path

  // Handle browse resources button click
  const handleBrowseResources = () => {
    quickSearchOpen.value = false
    location.route('/favorites')
  }

  // Handle logo/title click
  const handleLogoClick = () => {
    quickSearchOpen.value = false
    if (currentPath === '/') {
      // Refresh data if already on home page
      fetchFluxReport()
    } else {
      // Navigate to home page from any other route
      location.route('/')
    }
  }

  return (
    <header class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 shadow-sm transition-colors">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2 sm:py-3">
        {/* Mobile: Full-width search when open */}
        {quickSearchOpen.value && (
          <div class="sm:hidden">
            <QuickSearch />
          </div>
        )}

        {/* Desktop layout (and mobile when search is closed) */}
        <div class={`flex items-center justify-between ${quickSearchOpen.value ? 'hidden sm:flex' : ''}`}>
          {/* Left side: Logo and title (or expanded search on desktop) */}
          <div class="flex items-center gap-3 sm:gap-4 flex-1 min-w-0">
            {/* Logo - always visible */}
            <button
              onClick={handleLogoClick}
              class="flex items-center space-x-2 sm:space-x-4 hover:opacity-80 transition-opacity focus:outline-none flex-shrink-0"
              aria-label="Flux CD"
            >
              <FluxOperatorIcon className="w-7 h-7 sm:w-8 sm:h-8 text-gray-900 dark:text-white" />
              {/* Title - hidden when search is expanded */}
              {!quickSearchOpen.value && (
                <h1 class="text-lg sm:text-xl font-semibold text-gray-900 dark:text-white">Flux Status</h1>
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
          <div class="flex items-center space-x-3 sm:space-x-4 flex-shrink-0">
            {/* Quick Search button - shown when search is closed */}
            {!quickSearchOpen.value && (
              <>
                <QuickSearch />
                {/* Separator - desktop only */}
                <div class="hidden sm:block w-px h-5 bg-gray-300 dark:bg-gray-600" />
              </>
            )}
            {/* Navigation and User buttons - closer together on desktop */}
            <div class="flex items-center space-x-3 sm:space-x-2">
              {/* Browse Resources Button */}
              <button
                onClick={handleBrowseResources}
                title="Browse Resources"
                class="inline-flex items-center justify-center p-1.5 border border-gray-300 dark:border-gray-600 rounded-md text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3.75 9.776c.112-.017.227-.026.344-.026h15.812c.117 0 .232.009.344.026m-16.5 0a2.25 2.25 0 0 0-1.883 2.542l.857 6a2.25 2.25 0 0 0 2.227 1.932H19.05a2.25 2.25 0 0 0 2.227-1.932l.857-6a2.25 2.25 0 0 0-1.883-2.542m-16.5 0V6A2.25 2.25 0 0 1 6 3.75h3.879a1.5 1.5 0 0 1 1.06.44l2.122 2.12a1.5 1.5 0 0 0 1.06.44H18A2.25 2.25 0 0 1 20.25 9v.776" />
                </svg>
              </button>

              <UserMenu />
            </div>
          </div>
        </div>
      </div>
    </header>
  )
}
