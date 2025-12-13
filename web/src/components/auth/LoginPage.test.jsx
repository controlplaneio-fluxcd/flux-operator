// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/preact'
import { LoginPage } from './LoginPage'
import * as cookies from '../../utils/cookies'

// Mock the cookies module
vi.mock('../../utils/cookies', () => ({
  parseAuthProviderCookie: vi.fn(),
  parseAuthErrorCookie: vi.fn(),
  deleteCookie: vi.fn()
}))

describe('LoginPage', () => {
  let originalLocation
  let sessionStorageMock

  beforeEach(() => {
    // Reset mocks
    vi.clearAllMocks()

    // Mock window.location
    originalLocation = window.location
    delete window.location
    window.location = {
      href: '',
      pathname: '/',
      search: '',
      origin: 'http://localhost:9080'
    }

    // Mock sessionStorage
    sessionStorageMock = {
      store: {},
      getItem: vi.fn((key) => sessionStorageMock.store[key] || null),
      setItem: vi.fn((key, value) => { sessionStorageMock.store[key] = value }),
      removeItem: vi.fn((key) => { delete sessionStorageMock.store[key] })
    }
    Object.defineProperty(window, 'sessionStorage', {
      value: sessionStorageMock,
      writable: true
    })

    // Default mock returns
    cookies.parseAuthProviderCookie.mockReturnValue({
      provider: 'oidc',
      url: 'http://localhost:9080/oauth2/authorize',
      authenticated: false
    })
    cookies.parseAuthErrorCookie.mockReturnValue(null)
  })

  afterEach(() => {
    window.location = originalLocation
  })

  describe('Basic Rendering', () => {
    it('should render Flux logo', () => {
      render(<LoginPage />)

      // FluxIcon renders an SVG with specific path
      const logo = document.querySelector('svg[viewBox="0 0 64 64"]')
      expect(logo).toBeInTheDocument()
    })

    it('should render page title', () => {
      render(<LoginPage />)

      expect(screen.getByRole('heading', { name: 'Flux Status' })).toBeInTheDocument()
    })

    it('should render authentication required heading', () => {
      render(<LoginPage />)

      expect(screen.getByRole('heading', { name: 'Authentication Required' })).toBeInTheDocument()
    })

    it('should render authentication message', () => {
      render(<LoginPage />)

      expect(screen.getByText(/Sign in with your organization account/)).toBeInTheDocument()
    })

    it('should render shield icon', () => {
      render(<LoginPage />)

      // Shield icon has a specific path
      const shieldIcon = document.querySelector('path[d*="M9 12l2 2 4-4m5.618"]')
      expect(shieldIcon).toBeInTheDocument()
    })

    it('should render documentation link', () => {
      render(<LoginPage />)

      const link = screen.getByRole('link', { name: /Documentation/ })
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute('href', 'https://fluxoperator.dev/docs/')
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    })
  })

  describe('Auth Provider Cookie', () => {
    it('should display login button with OIDC provider', () => {
      render(<LoginPage />)

      expect(screen.getByRole('button', { name: /Login with OIDC/ })).toBeInTheDocument()
    })

    it('should display login button with custom provider name', () => {
      cookies.parseAuthProviderCookie.mockReturnValue({
        provider: 'github',
        url: 'http://localhost:9080/oauth2/authorize',
        authenticated: false
      })

      render(<LoginPage />)

      expect(screen.getByRole('button', { name: /Login with GITHUB/ })).toBeInTheDocument()
    })

    it('should capitalize provider name', () => {
      cookies.parseAuthProviderCookie.mockReturnValue({
        provider: 'azure',
        url: 'http://localhost:9080/oauth2/authorize',
        authenticated: false
      })

      render(<LoginPage />)

      expect(screen.getByRole('button', { name: /Login with AZURE/ })).toBeInTheDocument()
    })

    it('should show error when no auth provider cookie', async () => {
      cookies.parseAuthProviderCookie.mockReturnValue(null)

      render(<LoginPage />)

      await waitFor(() => {
        expect(screen.getByText(/Authentication configuration unavailable/)).toBeInTheDocument()
      })
    })

    it('should disable button when no auth provider', async () => {
      cookies.parseAuthProviderCookie.mockReturnValue(null)

      render(<LoginPage />)

      await waitFor(() => {
        const button = screen.getByRole('button', { name: /Login with OIDC/ })
        expect(button).toBeDisabled()
      })
    })
  })

  describe('Auth Error Cookie', () => {
    it('should display error message from auth-error cookie', async () => {
      cookies.parseAuthErrorCookie.mockReturnValue({
        msg: 'Invalid credentials'
      })

      render(<LoginPage />)

      await waitFor(() => {
        expect(screen.getByText('Invalid credentials')).toBeInTheDocument()
      })
    })

    it('should delete auth-error cookie after displaying', async () => {
      cookies.parseAuthErrorCookie.mockReturnValue({
        msg: 'Session expired'
      })

      render(<LoginPage />)

      await waitFor(() => {
        expect(cookies.deleteCookie).toHaveBeenCalledWith('auth-error')
      })
    })

    it('should not show error section when no errors', () => {
      render(<LoginPage />)

      // Error section has red background
      const errorSection = document.querySelector('.bg-red-50')
      expect(errorSection).not.toBeInTheDocument()
    })

    it('should show both auth error and cookie error', async () => {
      cookies.parseAuthProviderCookie.mockReturnValue(null)
      cookies.parseAuthErrorCookie.mockReturnValue({
        msg: 'Token expired'
      })

      render(<LoginPage />)

      await waitFor(() => {
        expect(screen.getByText('Token expired')).toBeInTheDocument()
        expect(screen.getByText(/Authentication configuration unavailable/)).toBeInTheDocument()
      })
    })
  })

  describe('Login URL Building', () => {
    it('should build URL with originalPath from current location', async () => {
      window.location.pathname = '/resource/HelmRelease/flux-system/weave-gitops'
      window.location.search = '?tab=events'

      render(<LoginPage />)

      const button = screen.getByRole('button', { name: /Login with OIDC/ })
      fireEvent.click(button)

      await waitFor(() => {
        expect(window.location.href).toContain('originalPath=%2Fresource%2FHelmRelease%2Fflux-system%2Fweave-gitops%3Ftab%3Devents')
      })
    })

    it('should use sessionStorage path if available (from logout)', async () => {
      sessionStorageMock.store['flux-originalPath'] = '/favorites'
      window.location.pathname = '/'

      render(<LoginPage />)

      const button = screen.getByRole('button', { name: /Login with OIDC/ })
      fireEvent.click(button)

      await waitFor(() => {
        expect(window.location.href).toContain('originalPath=%2Ffavorites')
        expect(sessionStorageMock.removeItem).toHaveBeenCalledWith('flux-originalPath')
      })
    })

    it('should handle absolute URLs', async () => {
      cookies.parseAuthProviderCookie.mockReturnValue({
        provider: 'oidc',
        url: 'https://auth.example.com/authorize',
        authenticated: false
      })

      render(<LoginPage />)

      const button = screen.getByRole('button', { name: /Login with OIDC/ })
      fireEvent.click(button)

      await waitFor(() => {
        expect(window.location.href).toContain('https://auth.example.com/authorize')
      })
    })

    it('should handle relative URLs', async () => {
      cookies.parseAuthProviderCookie.mockReturnValue({
        provider: 'oidc',
        url: '/oauth2/authorize',
        authenticated: false
      })

      render(<LoginPage />)

      const button = screen.getByRole('button', { name: /Login with OIDC/ })
      fireEvent.click(button)

      await waitFor(() => {
        expect(window.location.href).toContain('http://localhost:9080/oauth2/authorize')
      })
    })
  })

  describe('Login Button Behavior', () => {
    it('should be enabled when loginUrl is valid', async () => {
      render(<LoginPage />)

      await waitFor(() => {
        const button = screen.getByRole('button', { name: /Login with OIDC/ })
        expect(button).not.toBeDisabled()
      })
    })

    it('should be disabled when auth provider URL is missing', async () => {
      cookies.parseAuthProviderCookie.mockReturnValue({
        provider: 'oidc',
        url: null,
        authenticated: false
      })

      render(<LoginPage />)

      await waitFor(() => {
        const button = screen.getByRole('button', { name: /Login with OIDC/ })
        expect(button).toBeDisabled()
      })
    })

    it('should show loading state when clicked', async () => {
      render(<LoginPage />)

      const button = screen.getByRole('button', { name: /Login with OIDC/ })
      fireEvent.click(button)

      await waitFor(() => {
        expect(screen.getByText('Redirecting...')).toBeInTheDocument()
      })
    })

    it('should show spinner when loading', async () => {
      render(<LoginPage />)

      const button = screen.getByRole('button', { name: /Login with OIDC/ })
      fireEvent.click(button)

      await waitFor(() => {
        const spinner = document.querySelector('.animate-spin')
        expect(spinner).toBeInTheDocument()
      })
    })

    it('should be disabled when loading', async () => {
      render(<LoginPage />)

      const button = screen.getByRole('button', { name: /Login with OIDC/ })
      fireEvent.click(button)

      await waitFor(() => {
        expect(button).toBeDisabled()
      })
    })

    it('should redirect to login URL when clicked', async () => {
      render(<LoginPage />)

      const button = screen.getByRole('button', { name: /Login with OIDC/ })
      fireEvent.click(button)

      await waitFor(() => {
        expect(window.location.href).toContain('http://localhost:9080/oauth2/authorize')
      })
    })

    it('should have correct styling when enabled', async () => {
      render(<LoginPage />)

      await waitFor(() => {
        const button = screen.getByRole('button', { name: /Login with OIDC/ })
        expect(button).toHaveClass('bg-flux-blue')
      })
    })

    it('should have disabled styling when disabled', async () => {
      cookies.parseAuthProviderCookie.mockReturnValue(null)

      render(<LoginPage />)

      await waitFor(() => {
        const button = screen.getByRole('button', { name: /Login with OIDC/ })
        expect(button).toHaveClass('bg-gray-300')
        expect(button).toHaveClass('cursor-not-allowed')
      })
    })
  })

  describe('Default Provider Name', () => {
    it('should default to OIDC when provider is missing', async () => {
      cookies.parseAuthProviderCookie.mockReturnValue({
        url: 'http://localhost:9080/oauth2/authorize',
        authenticated: false
      })

      render(<LoginPage />)

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Login with OIDC/ })).toBeInTheDocument()
      })
    })
  })

  describe('Login Icon', () => {
    it('should render login icon on button', async () => {
      render(<LoginPage />)

      await waitFor(() => {
        // Login icon has viewBox 0 0 20 20 and fill currentColor
        const loginIcon = document.querySelector('button svg[viewBox="0 0 20 20"]')
        expect(loginIcon).toBeInTheDocument()
      })
    })
  })

  describe('Responsive Design', () => {
    it('should have mobile padding classes', () => {
      render(<LoginPage />)

      const container = document.querySelector('.py-12')
      expect(container).toBeInTheDocument()
    })

    it('should have max-width container', () => {
      render(<LoginPage />)

      const card = document.querySelector('.max-w-md')
      expect(card).toBeInTheDocument()
    })
  })

  describe('Error Display', () => {
    it('should render error icon when error present', async () => {
      cookies.parseAuthErrorCookie.mockReturnValue({
        msg: 'Test error'
      })

      render(<LoginPage />)

      await waitFor(() => {
        // Error icon path
        const errorIcon = document.querySelector('path[d*="M12 8v4m0 4h.01M21 12a9 9 0"]')
        expect(errorIcon).toBeInTheDocument()
      })
    })
  })
})
