// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect, useState } from 'preact/hooks'
import { parseAuthProviderCookie, parseAuthErrorCookie, deleteCookie } from '../../utils/cookies'
import { FluxIcon } from '../common/icons'

/**
 * LoginPage component - Authentication required page
 *
 * Displays when the user needs to authenticate via OIDC.
 * Shows:
 * - Flux Operator logo and title
 * - Authentication required message
 * - Error messages from auth-error cookie (if any)
 * - Login button that redirects to OIDC provider
 * - Link to documentation
 */
export function LoginPage() {
  const [authProvider, setAuthProvider] = useState(null)
  const [authError, setAuthError] = useState(null)
  const [cookieError, setCookieError] = useState(null)
  const [isLoading, setIsLoading] = useState(false)
  const [originalPath, setOriginalPath] = useState(null)

  useEffect(() => {
    // Parse auth-provider cookie
    const provider = parseAuthProviderCookie()
    setAuthProvider(provider)

    if (!provider) {
      setCookieError('Authentication configuration unavailable. Please contact your administrator.')
    }

    // Parse and clear auth-error cookie
    const error = parseAuthErrorCookie()
    if (error) {
      setAuthError(error)
      deleteCookie('auth-error')
    }

    // Get original path from sessionStorage (set by logout) or current location
    let storedPath = window.sessionStorage.getItem('flux-originalPath')
    if (storedPath) {
      window.sessionStorage.removeItem('flux-originalPath')
    } else {
      storedPath = window.location.pathname + window.location.search
    }
    setOriginalPath(storedPath)
  }, [])

  // Build login URL with originalPath
  const getLoginUrl = () => {
    if (!authProvider?.url || !originalPath) return null
    try {
      // Try as absolute URL first (external IDP), fall back to relative (local proxy)
      const loginUrl = authProvider.url.startsWith('http')
        ? new window.URL(authProvider.url)
        : new window.URL(authProvider.url, window.location.origin)
      loginUrl.searchParams.set('originalPath', originalPath)
      return loginUrl.toString()
    } catch {
      return null
    }
  }

  const handleLogin = () => {
    const loginUrl = getLoginUrl()
    if (loginUrl) {
      setIsLoading(true)
      window.location.href = loginUrl
    }
  }

  const providerName = authProvider?.provider || 'OIDC'
  // Capitalize first letter of provider name
  const displayProviderName = providerName.charAt(0).toUpperCase() + providerName.slice(1).toUpperCase()

  return (
    <div class="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors flex flex-col items-center justify-center px-4 py-12 sm:py-0">
      <div class="w-full max-w-md">
        {/* Logo and Title */}
        <div class="text-center mb-8">
          <FluxIcon className="w-16 h-16 mx-auto text-gray-900 dark:text-white mb-4" />
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            Flux Status
          </h1>
        </div>

        {/* Card */}
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-md border border-gray-200 dark:border-gray-700 p-8">
          {/* Shield Icon */}
          <div class="flex justify-center mb-6">
            <div class="w-16 h-16 rounded-full bg-flux-blue/10 dark:bg-flux-blue/20 flex items-center justify-center">
              <svg class="w-8 h-8 text-flux-blue" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
            </div>
          </div>

          {/* Message */}
          <div class="text-center mb-8">
            <h2 class="text-xl font-medium text-gray-900 dark:text-white mb-3">
              Authentication Required
            </h2>
            <p class="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
              Sign in with your organization account to access the Flux Status Page and monitor your GitOps pipelines.
            </p>
          </div>

          {/* Error Messages */}
          {(authError || cookieError) && (
            <div class="mb-6 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
              <div class="flex items-start gap-3">
                <svg class="w-5 h-5 text-red-500 dark:text-red-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div class="flex-1 min-w-0">
                  {authError && (
                    <p class="text-sm text-red-700 dark:text-red-300">
                      {authError.msg}
                    </p>
                  )}
                  {cookieError && (
                    <p class="text-sm text-red-700 dark:text-red-300">
                      {cookieError}
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Login Button */}
          <button
            onClick={handleLogin}
            disabled={!authProvider?.url || !originalPath || isLoading}
            class={`w-full flex items-center justify-center gap-2 px-4 py-4 rounded-md text-base font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-flux-blue ${
              authProvider?.url && originalPath && !isLoading
                ? 'bg-flux-blue text-white hover:bg-blue-600'
                : 'bg-gray-300 dark:bg-gray-600 text-gray-500 dark:text-gray-400 cursor-not-allowed'
            }`}
          >
            {isLoading ? (
              <>
                <svg class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                Redirecting...
              </>
            ) : (
              <>
                Login with {displayProviderName}
                <svg class="w-5 h-5" viewBox="0 0 20 20" fill="currentColor">
                  <path d="M9.76 0C15.417 0 20 4.477 20 10S15.416 20 9.76 20c-3.191 0-6.142-1.437-8.07-3.846a.644.644 0 0 1 .115-.918a.68.68 0 0 1 .94.113a8.96 8.96 0 0 0 7.016 3.343c4.915 0 8.9-3.892 8.9-8.692s-3.985-8.692-8.9-8.692a8.96 8.96 0 0 0-6.944 3.255a.68.68 0 0 1-.942.101a.644.644 0 0 1-.103-.92C3.703 1.394 6.615 0 9.761 0m.545 6.862l2.707 2.707c.262.262.267.68.011.936L10.38 13.15a.66.66 0 0 1-.937-.011a.66.66 0 0 1-.01-.937l1.547-1.548l-10.31.001A.66.66 0 0 1 0 10c0-.361.3-.654.67-.654h10.268L9.38 7.787a.66.66 0 0 1-.01-.937a.66.66 0 0 1 .935.011"/></svg>
              </>
            )}
          </button>
        </div>

        {/* Documentation Link */}
        <div class="mt-6 text-center">
          <a
            href="https://fluxoperator.dev"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 dark:text-gray-400 hover:text-flux-blue dark:hover:text-blue-400 transition-colors inline-flex items-center gap-1"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            Documentation
          </a>
        </div>
      </div>
    </div>
  )
}
