// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { Header } from './Header'
import { fetchFluxReport } from '../../app'

// Mock the UserMenu component
vi.mock('./UserMenu', () => ({
  UserMenu: () => <div data-testid="user-menu">User Menu</div>,
  userMenuOpen: { value: false }
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
    it('should render Flux logo button', () => {
      render(<Header />)

      const logoButton = screen.getByRole('button', { name: 'Flux CD' })
      expect(logoButton).toBeInTheDocument()
      // Should contain an SVG (the FluxIcon)
      expect(logoButton.querySelector('svg')).toBeInTheDocument()
    })

    it('should render "Flux Status" title', () => {
      render(<Header />)

      expect(screen.getByText('Flux Status')).toBeInTheDocument()
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

      const logoButton = screen.getByRole('button', { name: 'Flux CD' })
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

      const logoButton = screen.getByRole('button', { name: 'Flux CD' })
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

      const logoButton = screen.getByRole('button', { name: 'Flux CD' })
      fireEvent.click(logoButton)

      expect(mockRoute).toHaveBeenCalledWith('/')
      expect(fetchFluxReport).not.toHaveBeenCalled()
    })
  })

  describe('Browse Resources Button', () => {
    it('should render browse resources button', () => {
      render(<Header />)

      const navButton = screen.getByTitle('Browse Resources')
      expect(navButton).toBeInTheDocument()
    })

    it('should always show folder icon regardless of view', () => {
      render(<Header />)

      // Folder icon has path starting with d="M3.75 9.776..."
      const folderIcon = document.querySelector('path[d^="M3.75 9.776"]')
      expect(folderIcon).toBeInTheDocument()
    })

    it('should show folder icon when in events view', () => {
      useLocation.mockReturnValue({
        path: '/events',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const folderIcon = document.querySelector('path[d^="M3.75 9.776"]')
      expect(folderIcon).toBeInTheDocument()
    })

    it('should show folder icon when in resources view', () => {
      useLocation.mockReturnValue({
        path: '/resources',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const folderIcon = document.querySelector('path[d^="M3.75 9.776"]')
      expect(folderIcon).toBeInTheDocument()
    })

    it('should navigate to favorites view when clicked from dashboard', () => {
      useLocation.mockReturnValue({
        path: '/',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const navButton = screen.getByTitle('Browse Resources')
      fireEvent.click(navButton)

      expect(mockRoute).toHaveBeenCalledWith('/favorites')
    })

    it('should navigate to favorites view when clicked from events view', () => {
      useLocation.mockReturnValue({
        path: '/events',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const navButton = screen.getByTitle('Browse Resources')
      fireEvent.click(navButton)

      expect(mockRoute).toHaveBeenCalledWith('/favorites')
    })

    it('should navigate to favorites view when clicked from resources view', () => {
      useLocation.mockReturnValue({
        path: '/resources',
        query: {},
        route: mockRoute
      })

      render(<Header />)

      const navButton = screen.getByTitle('Browse Resources')
      fireEvent.click(navButton)

      expect(mockRoute).toHaveBeenCalledWith('/favorites')
    })
  })

  describe('User Menu', () => {
    it('should render UserMenu component', () => {
      render(<Header />)

      expect(screen.getByTestId('user-menu')).toBeInTheDocument()
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
