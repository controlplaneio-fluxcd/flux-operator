// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { Header } from './Header'
import { fetchFluxReport } from '../../app'
import { themeMode, themes } from '../../utils/theme'

// Mock the ThemeToggle component
vi.mock('./ThemeToggle', () => ({
  ThemeToggle: () => <div data-testid="theme-toggle">Theme Toggle</div>
}))

// Mock the QuickSearch component and signal
vi.mock('../search/QuickSearch', () => ({
  QuickSearch: () => <div data-testid="quick-search">Quick Search</div>,
  quickSearchOpen: { value: false }
}))

// Mock fetchFluxReport function
vi.mock('../../app', async () => {
  const actual = await vi.importActual('../../app')
  return {
    ...actual,
    fetchFluxReport: vi.fn()
  }
})

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: vi.fn(() => ({
    path: '/',
    query: {},
    route: mockRoute
  }))
}))

import { useLocation } from 'preact-iso'

describe('Header', () => {
  beforeEach(() => {
    // Reset theme
    themeMode.value = themes.light

    // Reset mocks
    vi.clearAllMocks()

    // Default to root path
    useLocation.mockReturnValue({
      path: '/',
      query: {},
      route: mockRoute
    })
  })

  describe('Branding', () => {
    it('should render Flux logo', () => {
      render(<Header />)

      const logo = screen.getByAltText('Flux CD')
      expect(logo).toBeInTheDocument()
      expect(logo).toHaveAttribute('src')
    })

    it('should render "Flux Status" title', () => {
      render(<Header />)

      expect(screen.getByText('Flux Status')).toBeInTheDocument()
    })

    it('should use black icon for light theme', () => {
      themeMode.value = themes.light

      render(<Header />)

      const logo = screen.getByAltText('Flux CD')
      expect(logo).toHaveAttribute('src', '/flux-icon-black.svg')
    })

    it('should use white icon for dark theme', () => {
      themeMode.value = themes.dark

      render(<Header />)

      const logo = screen.getByAltText('Flux CD')
      expect(logo).toHaveAttribute('src', '/flux-icon-white.svg')
    })
  })

  describe('Logo/Title Click Behavior', () => {
    it('should call fetchFluxReport when logo clicked in dashboard view', () => {
      useLocation.mockReturnValue({
        path: '/',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const logoButton = screen.getByAltText('Flux CD').closest('button')
      fireEvent.click(logoButton)

      expect(fetchFluxReport).toHaveBeenCalledTimes(1)
      expect(mockRoute).not.toHaveBeenCalled()
    })

    it('should return to dashboard when logo clicked in events view', () => {
      useLocation.mockReturnValue({
        path: '/events',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const logoButton = screen.getByAltText('Flux CD').closest('button')
      fireEvent.click(logoButton)

      expect(mockRoute).toHaveBeenCalledWith('/')
      expect(fetchFluxReport).not.toHaveBeenCalled()
    })

    it('should return to dashboard when logo clicked in resources view', () => {
      useLocation.mockReturnValue({
        path: '/resources',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const logoButton = screen.getByAltText('Flux CD').closest('button')
      fireEvent.click(logoButton)

      expect(mockRoute).toHaveBeenCalledWith('/')
      expect(fetchFluxReport).not.toHaveBeenCalled()
    })
  })

  describe('Navigation Button', () => {
    it('should render navigation button', () => {
      render(<Header />)

      // Look for the SVG icon
      const button = document.querySelector('button svg').closest('button')
      expect(button).toBeInTheDocument()
    })

    it('should show inbox icon when in dashboard view', () => {
      useLocation.mockReturnValue({
        path: '/',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      // Inbox icon has path starting with d="M20 13V6a2 2 0 00-2-2H6..."
      const inboxIcon = document.querySelector('path[d^="M20 13V6"]')
      expect(inboxIcon).toBeInTheDocument()
    })

    it('should show back arrow icon when in events view', () => {
      useLocation.mockReturnValue({
        path: '/events',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      // Back arrow has path d="M10 19l-7-7m0 0l7-7m-7 7h18"
      const backIcon = document.querySelector('path[d="M10 19l-7-7m0 0l7-7m-7 7h18"]')
      expect(backIcon).toBeInTheDocument()
    })

    it('should show back arrow icon when in resources view', () => {
      useLocation.mockReturnValue({
        path: '/resources',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const backIcon = document.querySelector('path[d="M10 19l-7-7m0 0l7-7m-7 7h18"]')
      expect(backIcon).toBeInTheDocument()
    })

    it('should navigate to resources view when clicked from dashboard', () => {
      useLocation.mockReturnValue({
        path: '/',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      // Find the navigation button (the one with SVG that's not the logo)
      const buttons = document.querySelectorAll('button')
      const navButton = Array.from(buttons).find(btn =>
        btn.querySelector('svg') && !btn.querySelector('img')
      )

      fireEvent.click(navButton)

      expect(mockRoute).toHaveBeenCalledWith('/resources')
    })

    it('should navigate to dashboard when clicked from events view', () => {
      useLocation.mockReturnValue({
        path: '/events',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      // Find the navigation button
      const buttons = document.querySelectorAll('button')
      const navButton = Array.from(buttons).find(btn =>
        btn.querySelector('svg') && !btn.querySelector('img')
      )

      fireEvent.click(navButton)

      expect(mockRoute).toHaveBeenCalledWith('/')
    })

    it('should navigate to dashboard when clicked from resources view', () => {
      useLocation.mockReturnValue({
        path: '/resources',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const buttons = document.querySelectorAll('button')
      const navButton = Array.from(buttons).find(btn =>
        btn.querySelector('svg') && !btn.querySelector('img')
      )

      fireEvent.click(navButton)

      expect(mockRoute).toHaveBeenCalledWith('/')
    })
  })

  describe('Theme Toggle', () => {
    it('should render ThemeToggle component', () => {
      render(<Header />)

      expect(screen.getByTestId('theme-toggle')).toBeInTheDocument()
    })
  })

  describe('Quick Search', () => {
    it('should render QuickSearch component', () => {
      render(<Header />)

      expect(screen.getByTestId('quick-search')).toBeInTheDocument()
    })
  })

  describe('Responsive Design', () => {
    it('should render header container with proper styling', () => {
      render(<Header />)

      const header = document.querySelector('header')
      expect(header).toBeInTheDocument()
      expect(header).toHaveClass('bg-white')
      expect(header).toHaveClass('dark:bg-gray-800')
    })
  })
})
