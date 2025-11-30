// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useLocation } from 'preact-iso'
import { fetchFluxReport } from '../../app'
import { UserMenu } from './UserMenu'
import { QuickSearch, quickSearchOpen } from '../search/QuickSearch'

/**
 * FluxIcon component - Flux CD logo SVG
 * Uses currentColor so it adapts to text color (dark in light mode, white in dark mode)
 */
function FluxIcon({ className }) {
  return (
    <svg class={className} viewBox="48 -2 263 364" fill="currentColor">
      <path d="M178.17168 173.50075c-.1203-.01154-.2406-.0235-.36059-.03858.11991.01825.24023.0246.36059.03858zm.59967.04272a10.136 10.136 0 0 0 1.14594 0q-.57285.02673-1.14594 0zm2.10621-.0813c-.11988.01508-.24012.027-.36036.03858.1203-.01398.24054-.02033.36036-.03858zm118.08722-92.75763L184.89446 6.54433a10.18235 10.18235 0 0 0-11.10022 0L59.72393 80.70454a10.18249 10.18249 0 0 0 0 17.07392l107.61285 69.962V116.8694a6.00343 6.00343 0 0 0-6.00341-6.00342h-7.913a6.00321 6.00321 0 0 1-5.199-9.005l25.924-44.902a6.00355 6.00355 0 0 1 10.39837 0l25.92383 44.902a6.0033 6.0033 0 0 1-5.1991 9.005h-7.91278a6.00343 6.00343 0 0 0-6.00341 6.00342v50.87091l107.6125-69.96185a10.1825 10.1825 0 0 0 0-17.07392z"/>
      <path d="M218.86377 209.9618l-27.51159-17.886v8.4759a215.88 215.88 0 0 0 27.51159 9.4101zm-27.51159 5.82989v11.89938c9.09108 2.77471 18.28849 5.05388 27.39469 7.29853 29.73311 7.32909 57.81738 14.25232 80.40261 36.83773 1.14474 1.14474 2.22232 2.31354 3.28875 3.48611a10.19327 10.19327 0 0 0-3.4753-13.27711l-44.6831-29.04976c-10.84437-3.58121-21.89768-6.31656-32.81794-9.00838-10.29656-2.53799-20.37113-5.07145-30.10971-8.1865zm-24.01537-9.75969a110.99292 110.99292 0 0 1-11.31571-6.59969l-10.21821 6.6432a124.31932 124.31932 0 0 0 21.53392 12.62007zm34.19212 37.87941q-5.12528-1.26352-10.17673-2.53816v11.6946q3.73686.938 7.46168 1.85485c29.73352 7.32863 57.8178 14.25187 80.40318 36.83709.03876.03876.07277.07916.11132.11792l9.66023-6.28051c-.58945-.61726-1.14225-1.24733-1.752-1.85715-24.81978-24.81979-55.7733-32.45048-85.70768-39.82864zm-34.19212-9.50272a124.54057 124.54057 0 0 1-36.33923-18.708l-10.00473 6.50436c14.22612 11.734 30.06393 18.85268 46.344 24.1308zm0 25.84203c-22.24132-5.828-43.1555-12.94341-61.02572-28.5006l-9.8338 6.39315c20.98652 19.336 45.95407 27.41893 70.85947 33.8211zm24.01537 17.73256c23.26546 6.00539 45.14367 13.18886 63.67506 29.69312l9.78-6.35823c-21.65249-20.35225-47.63835-28.47893-73.45509-35.04068zM81.94813 247.589l-9.70729 6.311c.34117.35032.65166.71178.99967 1.05976 24.81977 24.81979 55.77329 32.45043 85.70706 39.82925 25.92266 6.38909 50.58863 12.47807 71.45288 28.898l9.94772-6.46724c-23.50506-19.79078-51.507-26.74365-78.68557-33.44222-29.43548-7.25569-57.2468-14.1377-79.71447-36.18855zM61.327 266.87321c-1.12983-1.12983-2.19209-2.28328-3.24545-3.44a10.1544 10.1544 0 0 0 1.64428 15.67957l33.5887 21.83694c14.86583 6.15509 30.41359 10.0039 45.70058 13.77166 23.8684 5.88357 46.66708 11.52464 66.4189 25.197l10.1606-6.6057c-22.46332-16.88587-48.522-23.35562-73.86447-29.60253-29.73343-7.32851-57.81791-14.25175-80.40314-36.83694zm109.30024 84.33948c2.2354 1.0861 4.44718 2.22919 6.62269 3.46036a10.16012 10.16012 0 0 0 7.64543-1.402l5.63858-3.66577c-18.11065-12.11266-38.24653-18.247-58.31095-23.36044z"/>
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
                <h1 class="text-lg sm:text-xl font-bold text-gray-900 dark:text-white">Flux Status</h1>
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
