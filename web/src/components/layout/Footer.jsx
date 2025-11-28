// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { appliedTheme, themes } from '../../utils/theme'

/**
 * Footer component - Application footer with links and license information
 *
 * Features:
 * - Link to Flux Operator GitHub repository
 * - Link to documentation
 * - Link to enterprise support email
 * - License information (AGPL-3.0)
 * - Theme-aware Flux logo
 * - Responsive layout
 */
export function Footer() {
  // Use appropriate Flux logo based on theme
  const fluxLogoSrc = appliedTheme.value === themes.dark ? '/flux-icon-white.svg' : '/flux-icon-black.svg'

  return (
    <footer class="bg-white dark:bg-gray-800 sm:border-t border-gray-200 dark:border-gray-700 transition-colors mt-8">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-3 sm:py-4">
        {/* Mobile: centered, minimal footer */}
        <div class="flex sm:hidden items-center justify-center gap-4 text-xs text-gray-500 dark:text-gray-400">
          <a
            href="https://github.com/controlplaneio-fluxcd/flux-operator"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center gap-1.5 hover:text-gray-900 dark:hover:text-white transition-colors"
          >
            <img src={fluxLogoSrc} alt="Flux" class="w-3.5 h-3.5" />
            <span>Flux Operator</span>
          </a>
          <span class="text-gray-300 dark:text-gray-600">•</span>
          <a
            href="mailto:flux-enterprise@control-plane.io"
            class="hover:text-gray-900 dark:hover:text-white transition-colors"
          >
            Enterprise Support
          </a>
        </div>

        {/* Desktop: full footer */}
        <div class="hidden sm:flex flex-row items-center justify-between gap-4">
          <div class="flex flex-row items-center gap-6 text-sm">
            <a
              href="https://github.com/controlplaneio-fluxcd/flux-operator"
              target="_blank"
              rel="noopener noreferrer"
              class="flex items-center gap-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
            >
              <img src={fluxLogoSrc} alt="Flux" class="w-4 h-4" />
              <span>Flux Operator</span>
            </a>
            <span class="text-gray-300 dark:text-gray-600">•</span>
            <a
              href="https://fluxcd.control-plane.io/operator/"
              target="_blank"
              rel="noopener noreferrer"
              class="flex items-center gap-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <span>Documentation</span>
            </a>
            <span class="text-gray-300 dark:text-gray-600">•</span>
            <a
              href="mailto:flux-enterprise@control-plane.io"
              class="flex items-center gap-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
            >
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
              <span>Enterprise Support</span>
            </a>
          </div>
          <div class="text-sm text-gray-600 dark:text-gray-400 text-right">
            <p>AGPL-3.0 Licensed</p>
          </div>
        </div>
      </div>
    </footer>
  )
}
