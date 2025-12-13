// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import {
  getCookie,
  deleteCookie,
  parseAuthProviderCookie,
  parseAuthErrorCookie,
  isOIDCProvider
} from './cookies'

// Helper to encode JSON to base64url (no padding, URL-safe chars)
function encodeBase64Url(str) {
  const base64 = window.btoa(str)
  return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

describe('cookies utilities', () => {
  let originalCookie

  beforeEach(() => {
    // Store original cookie getter/setter
    originalCookie = Object.getOwnPropertyDescriptor(document, 'cookie')
    // Clear document.cookie for each test
    Object.defineProperty(document, 'cookie', {
      writable: true,
      value: ''
    })
  })

  afterEach(() => {
    // Restore original cookie behavior
    if (originalCookie) {
      Object.defineProperty(document, 'cookie', originalCookie)
    }
  })

  describe('getCookie', () => {
    it('should return null when cookie does not exist', () => {
      document.cookie = 'other-cookie=value'
      expect(getCookie('auth-provider')).toBeNull()
    })

    it('should return cookie value when cookie exists', () => {
      document.cookie = 'auth-provider=somevalue'
      expect(getCookie('auth-provider')).toBe('somevalue')
    })

    it('should handle multiple cookies', () => {
      document.cookie = 'first=one; auth-provider=myvalue; last=three'
      expect(getCookie('auth-provider')).toBe('myvalue')
    })

    it('should handle cookies with equals signs in value', () => {
      document.cookie = 'auth-provider=value=with=equals'
      expect(getCookie('auth-provider')).toBe('value=with=equals')
    })

    it('should handle URL encoded cookie values', () => {
      document.cookie = 'auth-provider=' + encodeURIComponent('hello world')
      expect(getCookie('auth-provider')).toBe('hello world')
    })

    it('should handle empty cookie string', () => {
      document.cookie = ''
      expect(getCookie('auth-provider')).toBeNull()
    })

    it('should trim whitespace from cookie names', () => {
      document.cookie = '  auth-provider  =value'
      expect(getCookie('auth-provider')).toBe('value')
    })
  })

  describe('deleteCookie', () => {
    it('should not throw when deleting a cookie', () => {
      // deleteCookie sets cookie with expired date - verify it doesn't throw
      expect(() => deleteCookie('auth-error')).not.toThrow()
    })
  })

  describe('parseAuthProviderCookie', () => {
    it('should return null when cookie does not exist', () => {
      document.cookie = ''
      expect(parseAuthProviderCookie()).toBeNull()
    })

    it('should parse valid base64url encoded cookie', () => {
      const payload = { provider: 'oidc', url: 'https://auth.example.com/login', authenticated: true }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      const result = parseAuthProviderCookie()

      expect(result).toEqual(payload)
    })

    it('should return null for invalid JSON', () => {
      const encoded = encodeBase64Url('not valid json')
      document.cookie = `auth-provider=${encoded}`

      expect(parseAuthProviderCookie()).toBeNull()
    })

    it('should return null for invalid base64', () => {
      document.cookie = 'auth-provider=!!!invalid-base64!!!'
      expect(parseAuthProviderCookie()).toBeNull()
    })

    it('should return null when provider field is missing', () => {
      const payload = { url: 'https://auth.example.com', authenticated: true }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(parseAuthProviderCookie()).toBeNull()
    })

    it('should return null when url field is missing', () => {
      const payload = { provider: 'oidc', authenticated: true }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(parseAuthProviderCookie()).toBeNull()
    })

    it('should return null when authenticated field is missing', () => {
      const payload = { provider: 'oidc', url: 'https://auth.example.com' }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(parseAuthProviderCookie()).toBeNull()
    })

    it('should return null when provider is not a string', () => {
      const payload = { provider: 123, url: 'https://auth.example.com', authenticated: true }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(parseAuthProviderCookie()).toBeNull()
    })

    it('should return null when authenticated is not a boolean', () => {
      const payload = { provider: 'oidc', url: 'https://auth.example.com', authenticated: 'yes' }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(parseAuthProviderCookie()).toBeNull()
    })

    it('should handle authenticated=false correctly', () => {
      const payload = { provider: 'oidc', url: 'https://auth.example.com/login', authenticated: false }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      const result = parseAuthProviderCookie()

      expect(result).toEqual(payload)
      expect(result.authenticated).toBe(false)
    })

    it('should handle base64url special characters (- and _)', () => {
      // Create a payload that produces base64 with + and / characters
      const payload = { provider: 'oidc', url: 'https://auth.example.com/login?redirect=/', authenticated: true }
      const json = JSON.stringify(payload)
      // Encode to base64url format (- instead of +, _ instead of /, no padding)
      const encoded = encodeBase64Url(json)

      document.cookie = `auth-provider=${encoded}`
      const result = parseAuthProviderCookie()

      expect(result).toEqual(payload)
    })
  })

  describe('parseAuthErrorCookie', () => {
    it('should return null when cookie does not exist', () => {
      document.cookie = ''
      expect(parseAuthErrorCookie()).toBeNull()
    })

    it('should parse valid error cookie', () => {
      const payload = { code: 401, msg: 'Unauthorized access' }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-error=${encoded}`

      const result = parseAuthErrorCookie()

      expect(result).toEqual(payload)
    })

    it('should return null for invalid JSON', () => {
      const encoded = encodeBase64Url('not json')
      document.cookie = `auth-error=${encoded}`

      expect(parseAuthErrorCookie()).toBeNull()
    })

    it('should return null when code is not a number', () => {
      const payload = { code: '401', msg: 'Error' }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-error=${encoded}`

      expect(parseAuthErrorCookie()).toBeNull()
    })

    it('should return null when msg is not a string', () => {
      const payload = { code: 401, msg: 123 }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-error=${encoded}`

      expect(parseAuthErrorCookie()).toBeNull()
    })

    it('should return null when code field is missing', () => {
      const payload = { msg: 'Error message' }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-error=${encoded}`

      expect(parseAuthErrorCookie()).toBeNull()
    })

    it('should return null when msg field is missing', () => {
      const payload = { code: 401 }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-error=${encoded}`

      expect(parseAuthErrorCookie()).toBeNull()
    })
  })

  describe('isOIDCProvider', () => {
    it('should return true for oidc provider (lowercase)', () => {
      const payload = { provider: 'oidc', url: 'https://auth.example.com', authenticated: true }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(isOIDCProvider()).toBe(true)
    })

    it('should return true for OIDC provider (uppercase)', () => {
      const payload = { provider: 'OIDC', url: 'https://auth.example.com', authenticated: true }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(isOIDCProvider()).toBe(true)
    })

    it('should return true for Oidc provider (mixed case)', () => {
      const payload = { provider: 'Oidc', url: 'https://auth.example.com', authenticated: true }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(isOIDCProvider()).toBe(true)
    })

    it('should return false for non-OIDC provider', () => {
      const payload = { provider: 'saml', url: 'https://auth.example.com', authenticated: true }
      const encoded = encodeBase64Url(JSON.stringify(payload))
      document.cookie = `auth-provider=${encoded}`

      expect(isOIDCProvider()).toBe(false)
    })

    it('should return false when cookie does not exist', () => {
      document.cookie = ''
      expect(isOIDCProvider()).toBe(false)
    })

    it('should return false for invalid cookie', () => {
      document.cookie = 'auth-provider=invalid'
      expect(isOIDCProvider()).toBe(false)
    })
  })
})
