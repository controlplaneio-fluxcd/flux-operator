// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { Header } from './Header'
import { showSearchView, fetchFluxReport } from '../app'
import { themeMode, themes } from '../utils/theme'

// Mock the ThemeToggle component
vi.mock('./ThemeToggle', () => ({
  ThemeToggle: () => <div data-testid="theme-toggle">Theme Toggle</div>
}))

// Mock fetchFluxReport function
vi.mock('../app', async () => {
  const actual = await vi.importActual('../app')
  return {
    ...actual,
    fetchFluxReport: vi.fn()
  }
})

describe('Header', () => {
  beforeEach(() => {
    // Reset signals
    showSearchView.value = false
    themeMode.value = themes.light // This will trigger effect to update appliedTheme

    // Reset mocks
    vi.clearAllMocks()
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
      showSearchView.value = false

      render(<Header />)

      const logoButton = screen.getByAltText('Flux CD').closest('button')
      fireEvent.click(logoButton)

      expect(fetchFluxReport).toHaveBeenCalledTimes(1)
    })

    it('should return to dashboard when logo clicked in search view', () => {
      showSearchView.value = true

      render(<Header />)

      const logoButton = screen.getByAltText('Flux CD').closest('button')
      fireEvent.click(logoButton)

      expect(showSearchView.value).toBe(false)
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

    it('should show search icon when in dashboard view', () => {
      showSearchView.value = false

      render(<Header />)

      // Search icon has path d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
      const searchIcon = document.querySelector('path[d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"]')
      expect(searchIcon).toBeInTheDocument()
    })

    it('should show back arrow icon when in search view', () => {
      showSearchView.value = true

      render(<Header />)

      // Back arrow has path d="M10 19l-7-7m0 0l7-7m-7 7h18"
      const backIcon = document.querySelector('path[d="M10 19l-7-7m0 0l7-7m-7 7h18"]')
      expect(backIcon).toBeInTheDocument()
    })

    it('should navigate to search view when clicked from dashboard', () => {
      showSearchView.value = false

      render(<Header />)

      // Find the navigation button (the one with SVG that's not the logo)
      const buttons = document.querySelectorAll('button')
      const navButton = Array.from(buttons).find(btn =>
        btn.querySelector('svg') && !btn.querySelector('img')
      )

      fireEvent.click(navButton)

      expect(showSearchView.value).toBe(true)
    })

    it('should navigate to dashboard when clicked from search view', () => {
      showSearchView.value = true

      render(<Header />)

      // Find the navigation button
      const buttons = document.querySelectorAll('button')
      const navButton = Array.from(buttons).find(btn =>
        btn.querySelector('svg') && !btn.querySelector('img')
      )

      fireEvent.click(navButton)

      expect(showSearchView.value).toBe(false)
    })

    it('should toggle between views when clicked multiple times', () => {
      showSearchView.value = false

      render(<Header />)

      const buttons = document.querySelectorAll('button')
      const navButton = Array.from(buttons).find(btn =>
        btn.querySelector('svg') && !btn.querySelector('img')
      )

      // Click to search
      fireEvent.click(navButton)
      expect(showSearchView.value).toBe(true)

      // Click back to dashboard
      fireEvent.click(navButton)
      expect(showSearchView.value).toBe(false)
    })
  })

  describe('Theme Toggle', () => {
    it('should render ThemeToggle component', () => {
      render(<Header />)

      expect(screen.getByTestId('theme-toggle')).toBeInTheDocument()
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
