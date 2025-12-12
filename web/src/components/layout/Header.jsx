// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useLocation } from 'preact-iso'
import { fetchFluxReport } from '../../app'
import { UserMenu } from './UserMenu'
import { QuickSearch, quickSearchOpen } from '../search/QuickSearch'

/**
 * FluxIcon component - Flux Operator logo SVG
 * Uses currentColor so it adapts to text color (dark in light mode, white in dark mode)
 */
function FluxIcon({ className }) {
  return (
    <svg class={className} viewBox="0 0 64 64" fill="currentColor">
      <path d="M49.0358 52.966C39.6585 60.5986 26.248 61.0543 16.4252 54.0796C6.13019 46.767 2.30811 33.2284 6.99163 21.7686L13.4211 25.0927C10.4886 32.8982 12.7235 41.7046 19.2592 47.1881C23.9542 51.1254 30.4604 52.5884 36.662 51.101L35.5459 46.438C30.8112 47.5734 25.8768 46.4802 22.3401 43.5158C16.9155 38.9615 15.4012 31.389 18.655 25.1004L19.7584 22.9692L4.83995 15.2598L3.73915 17.3884C-3.50565 31.375 0.756753 48.8303 13.6476 57.99C21.2828 63.4095 30.7536 65.0236 39.4857 62.9129C43.9926 61.8236 48.3036 59.7449 52.063 56.6857C53.174 55.7794 54.2032 54.7746 55.1862 53.7302L50.7484 51.4364C50.1968 51.9689 49.6297 52.4834 49.0358 52.966Z"/>
      <path d="M50.3696 6.01048C38.9315 -2.1252 23.2451 -1.69128 12.2268 7.06904C10.9865 8.05208 9.84603 9.14776 8.77467 10.2934L13.2099 12.5846C13.8486 11.9689 14.5116 11.375 15.2092 10.8194C24.5686 3.38264 37.8819 3.00888 47.5907 9.9132C57.8742 17.2284 61.6873 30.7618 57.5222 42.4802L51.0928 39.1561C53.5056 31.101 51.2835 22.3049 44.7696 16.8201C39.9516 12.7586 33.2867 11.3506 26.9238 12.9865L28.0694 17.6418C32.9104 16.3593 38.0534 17.4294 41.6835 20.4886C47.0915 25.0428 48.5955 32.605 45.3468 38.8847L44.2435 41.0159L59.1644 48.7254L60.2627 46.5967C67.4985 32.6166 63.2476 15.1714 50.3708 6.0092"/>
    </svg>
  )
}

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

  // Check if we're in a tab view (favorites, events, or resources) or resource dashboard
  const isTabView = currentPath === '/favorites' || currentPath === '/events' || currentPath === '/resources'
  const isResourceDashboard = currentPath.startsWith('/resource/')
  const isNotDashboard = isTabView || isResourceDashboard

  // Handle browse resources button click
  const handleBrowseResources = () => {
    quickSearchOpen.value = false
    location.route('/favorites')
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
              <FluxIcon className="w-7 h-7 sm:w-8 sm:h-8 text-gray-900 dark:text-white" />
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
