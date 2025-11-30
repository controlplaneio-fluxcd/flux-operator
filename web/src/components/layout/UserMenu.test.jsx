// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { UserMenu, userMenuOpen } from './UserMenu'
import { themeMode, appliedTheme, themes } from '../../utils/theme'
import { clearFavorites } from '../../utils/favorites'

// Mock the favorites module
vi.mock('../../utils/favorites', () => ({
  clearFavorites: vi.fn()
}))

describe('UserMenu', () => {
  beforeEach(() => {
    // Reset signals
    userMenuOpen.value = false
    themeMode.value = themes.light
    appliedTheme.value = themes.light

    // Reset mocks
    vi.clearAllMocks()
  })

  describe('User Button', () => {
    it('should render user button', () => {
      render(<UserMenu />)

      const button = screen.getByRole('button', { name: 'User menu' })
      expect(button).toBeInTheDocument()
    })

    it('should have proper accessibility attributes', () => {
      render(<UserMenu />)

      const button = screen.getByRole('button', { name: 'User menu' })
      expect(button).toHaveAttribute('aria-haspopup', 'true')
      expect(button).toHaveAttribute('aria-expanded', 'false')
    })

    it('should update aria-expanded when menu opens', () => {
      render(<UserMenu />)

      const button = screen.getByRole('button', { name: 'User menu' })
      fireEvent.click(button)

      expect(button).toHaveAttribute('aria-expanded', 'true')
    })

    it('should toggle menu open state on click', () => {
      render(<UserMenu />)

      const button = screen.getByRole('button', { name: 'User menu' })

      expect(userMenuOpen.value).toBe(false)

      fireEvent.click(button)
      expect(userMenuOpen.value).toBe(true)

      fireEvent.click(button)
      expect(userMenuOpen.value).toBe(false)
    })
  })

  describe('Dropdown Menu', () => {
    it('should not render dropdown when closed', () => {
      render(<UserMenu />)

      expect(screen.queryByText('flux-operator')).not.toBeInTheDocument()
    })

    it('should render dropdown when open', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      expect(screen.getByText('flux-operator')).toBeInTheDocument()
    })

    it('should display user info', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      expect(screen.getByText('flux-operator')).toBeInTheDocument()
      expect(screen.getByText('cluster:view')).toBeInTheDocument()
    })

    it('should render mobile close button', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      const closeButton = screen.getByRole('button', { name: 'Close menu' })
      expect(closeButton).toBeInTheDocument()
    })

    it('should close menu when mobile close button clicked', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      const closeButton = screen.getByRole('button', { name: 'Close menu' })
      fireEvent.click(closeButton)

      expect(userMenuOpen.value).toBe(false)
    })
  })

  describe('Theme Toggle', () => {
    it('should render theme button', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      expect(screen.getByText(/Theme:/)).toBeInTheDocument()
    })

    it('should display Light when theme is light', () => {
      themeMode.value = themes.light
      userMenuOpen.value = true
      render(<UserMenu />)

      expect(screen.getByText('Theme: Light')).toBeInTheDocument()
    })

    it('should display Dark when theme is dark', () => {
      themeMode.value = themes.dark
      userMenuOpen.value = true
      render(<UserMenu />)

      expect(screen.getByText('Theme: Dark')).toBeInTheDocument()
    })

    it('should display Auto when theme is auto', () => {
      themeMode.value = themes.auto
      userMenuOpen.value = true
      render(<UserMenu />)

      expect(screen.getByText('Theme: Auto')).toBeInTheDocument()
    })

    it('should cycle theme when clicked', () => {
      themeMode.value = themes.auto
      userMenuOpen.value = true
      render(<UserMenu />)

      const themeButton = screen.getByText('Theme: Auto').closest('button')
      fireEvent.click(themeButton)

      // auto -> dark
      expect(themeMode.value).toBe(themes.dark)
    })
  })

  describe('Feedback Link', () => {
    it('should render feedback link', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      const link = screen.getByText('Provide feedback')
      expect(link).toBeInTheDocument()
    })

    it('should have correct href', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      const link = screen.getByText('Provide feedback').closest('a')
      expect(link.getAttribute('href')).toMatch(/^https:\/\/github\.com\/controlplaneio-fluxcd\/flux-operator\/issues\/new/)
    })

    it('should open in new tab', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      const link = screen.getByText('Provide feedback').closest('a')
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    })

    it('should close menu when clicked', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      const link = screen.getByText('Provide feedback').closest('a')
      fireEvent.click(link)

      expect(userMenuOpen.value).toBe(false)
    })
  })

  describe('Clear Local Storage', () => {
    it('should render clear local storage button', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      expect(screen.getByText('Clear local storage')).toBeInTheDocument()
    })

    it('should show confirmation and call clearFavorites when confirmed', () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
      userMenuOpen.value = true
      render(<UserMenu />)

      const button = screen.getByText('Clear local storage').closest('button')
      fireEvent.click(button)

      expect(confirmSpy).toHaveBeenCalledWith('This will delete your favorites and search history from local storage. Continue?')
      expect(clearFavorites).toHaveBeenCalledTimes(1)
      expect(userMenuOpen.value).toBe(false)

      confirmSpy.mockRestore()
    })

    it('should not call clearFavorites when confirmation is cancelled', () => {
      const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
      userMenuOpen.value = true
      render(<UserMenu />)

      const button = screen.getByText('Clear local storage').closest('button')
      fireEvent.click(button)

      expect(confirmSpy).toHaveBeenCalled()
      expect(clearFavorites).not.toHaveBeenCalled()
      expect(userMenuOpen.value).toBe(true)

      confirmSpy.mockRestore()
    })
  })

  describe('Keyboard Navigation', () => {
    it('should close menu on Escape key', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      fireEvent.keyDown(document, { key: 'Escape' })

      expect(userMenuOpen.value).toBe(false)
    })

    it('should not close on other keys', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      fireEvent.keyDown(document, { key: 'Enter' })

      expect(userMenuOpen.value).toBe(true)
    })
  })

  describe('Click Outside', () => {
    it('should close menu when clicking outside', () => {
      userMenuOpen.value = true
      render(
        <div>
          <div data-testid="outside">Outside</div>
          <UserMenu />
        </div>
      )

      const outside = screen.getByTestId('outside')
      fireEvent.mouseDown(outside)

      expect(userMenuOpen.value).toBe(false)
    })

    it('should not close menu when clicking inside', () => {
      userMenuOpen.value = true
      render(<UserMenu />)

      const username = screen.getByText('flux-operator')
      fireEvent.mouseDown(username)

      expect(userMenuOpen.value).toBe(true)
    })
  })

  describe('Theme Icons', () => {
    it('should render sun icon for light theme', () => {
      themeMode.value = themes.light
      appliedTheme.value = themes.light
      userMenuOpen.value = true
      render(<UserMenu />)

      // Sun icon path starts with "M12 3v1m0 16v1m9-9h-1M4 12H3..."
      const sunIcon = document.querySelector('path[d^="M12 3v1m0 16v1"]')
      expect(sunIcon).toBeInTheDocument()
    })

    it('should render moon icon for dark theme', () => {
      themeMode.value = themes.dark
      appliedTheme.value = themes.dark
      userMenuOpen.value = true
      render(<UserMenu />)

      // Moon icon path starts with "M20.354 15.354..."
      const moonIcon = document.querySelector('path[d^="M20.354 15.354"]')
      expect(moonIcon).toBeInTheDocument()
    })

    it('should render auto icon for auto theme', () => {
      themeMode.value = themes.auto
      userMenuOpen.value = true
      render(<UserMenu />)

      // Auto icon path starts with "M9.663 17h4.673..."
      const autoIcon = document.querySelector('path[d^="M9.663 17h4.673"]')
      expect(autoIcon).toBeInTheDocument()
    })
  })

  describe('Signal Export', () => {
    it('should export userMenuOpen signal', () => {
      expect(userMenuOpen).toBeDefined()
      expect(userMenuOpen.value).toBe(false)
    })

    it('should allow external control of menu state', () => {
      // Setting signal before render should work
      userMenuOpen.value = true
      render(<UserMenu />)

      expect(screen.getByText('flux-operator')).toBeInTheDocument()
    })
  })
})
