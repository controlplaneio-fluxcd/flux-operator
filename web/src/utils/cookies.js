// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

/**
 * Get a cookie value by name
 * @param {string} name - Cookie name
 * @returns {string|null} - Cookie value or null if not found
 */
export function getCookie(name) {
  const cookies = document.cookie.split(';')
  for (const cookie of cookies) {
    const [cookieName, ...cookieValueParts] = cookie.split('=')
    if (cookieName.trim() === name) {
      return decodeURIComponent(cookieValueParts.join('='))
    }
  }
  return null
}

/**
 * Decode a base64url encoded string (RawURLEncoding - no padding)
 * @param {string} base64url - Base64url encoded string
 * @returns {string} - Decoded string
 */
function decodeBase64Url(base64url) {
  // Convert base64url to base64 (replace - with +, _ with /)
  let base64 = base64url.replace(/-/g, '+').replace(/_/g, '/')
  // Add padding if needed
  const padding = base64.length % 4
  if (padding) {
    base64 += '='.repeat(4 - padding)
  }
  return window.atob(base64)
}

/**
 * Delete a cookie by name
 * @param {string} name - Cookie name
 */
export function deleteCookie(name) {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`
}

/**
 * Parse the auth-provider cookie
 * @returns {Object|null} - Parsed auth provider object or null if invalid
 * Expected format: { provider: string, url: string, authenticated: boolean }
 */
export function parseAuthProviderCookie() {
  const value = getCookie('auth-provider')
  if (!value) {
    return null
  }
  try {
    // Decode base64url then parse JSON
    const decoded = decodeBase64Url(value)
    const parsed = JSON.parse(decoded)
    // Validate required fields
    if (typeof parsed.provider !== 'string' || typeof parsed.url !== 'string' || typeof parsed.authenticated !== 'boolean') {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

/**
 * Parse the auth-error cookie
 * @returns {Object|null} - Parsed error object or null if invalid
 * Expected format: { msg: string }
 */
export function parseAuthErrorCookie() {
  const value = getCookie('auth-error')
  if (!value) {
    return null
  }
  try {
    // Decode base64url then parse JSON
    const decoded = decodeBase64Url(value)
    const parsed = JSON.parse(decoded)
    // Validate required fields
    if (typeof parsed.msg !== 'string') {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

/**
 * Check if the auth provider is OIDC (case-insensitive)
 * @returns {boolean}
 */
export function isOIDCProvider() {
  const authProvider = parseAuthProviderCookie()
  return authProvider?.provider?.toLowerCase() === 'oidc'
}
