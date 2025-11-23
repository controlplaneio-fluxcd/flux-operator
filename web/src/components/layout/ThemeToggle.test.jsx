// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { ThemeToggle } from './ThemeToggle'
import { themeMode, appliedTheme, cycleTheme, themes } from '../../utils/theme'

// Mock theme signals and cycleTheme function
vi.mock('../../utils/theme', async () => {
  const actual = await vi.importActual('../../utils/theme')
  return {
    ...actual,
    themeMode: { value: 'light' },
    appliedTheme: { value: 'light' },
    cycleTheme: vi.fn(),
    themes: { light: 'light', dark: 'dark', auto: 'auto' }
  }
})

describe('ThemeToggle', () => {
  beforeEach(() => {
    // Reset signals to default state
    themeMode.value = themes.light
    appliedTheme.value = themes.light

    // Clear mock calls
    vi.clearAllMocks()
  })

  describe('Component Rendering', () => {
    it('should render a button element', () => {
      render(<ThemeToggle />)

      const button = document.querySelector('button')
      expect(button).toBeInTheDocument()
    })

    it('should have proper styling classes', () => {
      render(<ThemeToggle />)

      const button = document.querySelector('button')
      expect(button).toHaveClass('flex')
      expect(button).toHaveClass('items-center')
      expect(button).toHaveClass('space-x-2')
      expect(button).toHaveClass('px-3')
      expect(button).toHaveClass('py-2')
      expect(button).toHaveClass('rounded-lg')
      expect(button).toHaveClass('hover:bg-gray-100')
      expect(button).toHaveClass('dark:hover:bg-gray-800')
      expect(button).toHaveClass('transition-colors')
    })

    it('should render both icon and label', () => {
      render(<ThemeToggle />)

      const svg = document.querySelector('svg')
      const label = screen.getByText('Light')

      expect(svg).toBeInTheDocument()
      expect(label).toBeInTheDocument()
    })
  })

  describe('Light Theme Mode', () => {
    it('should show "Light" label in light mode', () => {
      themeMode.value = themes.light

      render(<ThemeToggle />)

      expect(screen.getByText('Light')).toBeInTheDocument()
    })

    it('should show sun icon in light mode', () => {
      themeMode.value = themes.light
      appliedTheme.value = themes.light

      render(<ThemeToggle />)

      // Sun icon has specific path
      const sunIcon = document.querySelector('path[d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"]')
      expect(sunIcon).toBeInTheDocument()
    })

    it('should have title attribute for light mode', () => {
      themeMode.value = themes.light

      render(<ThemeToggle />)

      const button = document.querySelector('button')
      expect(button).toHaveAttribute('title', 'Theme: Light')
    })

    it('should have proper icon styling', () => {
      themeMode.value = themes.light

      render(<ThemeToggle />)

      const svg = document.querySelector('svg')
      expect(svg).toHaveClass('w-5')
      expect(svg).toHaveClass('h-5')
    })
  })

  describe('Dark Theme Mode', () => {
    it('should show "Dark" label in dark mode', () => {
      themeMode.value = themes.dark

      render(<ThemeToggle />)

      expect(screen.getByText('Dark')).toBeInTheDocument()
    })

    it('should show moon icon in dark mode', () => {
      themeMode.value = themes.dark
      appliedTheme.value = themes.dark

      render(<ThemeToggle />)

      // Moon icon has specific path
      const moonIcon = document.querySelector('path[d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"]')
      expect(moonIcon).toBeInTheDocument()
    })

    it('should have title attribute for dark mode', () => {
      themeMode.value = themes.dark

      render(<ThemeToggle />)

      const button = document.querySelector('button')
      expect(button).toHaveAttribute('title', 'Theme: Dark')
    })
  })

  describe('Auto Theme Mode', () => {
    it('should show "Auto" label in auto mode', () => {
      themeMode.value = themes.auto

      render(<ThemeToggle />)

      expect(screen.getByText('Auto')).toBeInTheDocument()
    })

    it('should show lightbulb icon in auto mode', () => {
      themeMode.value = themes.auto

      render(<ThemeToggle />)

      // Lightbulb icon has specific path
      const lightbulbIcon = document.querySelector('path[d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"]')
      expect(lightbulbIcon).toBeInTheDocument()
    })

    it('should have title attribute for auto mode', () => {
      themeMode.value = themes.auto

      render(<ThemeToggle />)

      const button = document.querySelector('button')
      expect(button).toHaveAttribute('title', 'Theme: Auto')
    })
  })

  describe('Click Handler', () => {
    it('should call cycleTheme when clicked', () => {
      render(<ThemeToggle />)

      const button = document.querySelector('button')
      fireEvent.click(button)

      expect(cycleTheme).toHaveBeenCalledTimes(1)
    })

    it('should call cycleTheme on multiple clicks', () => {
      render(<ThemeToggle />)

      const button = document.querySelector('button')

      fireEvent.click(button)
      fireEvent.click(button)
      fireEvent.click(button)

      expect(cycleTheme).toHaveBeenCalledTimes(3)
    })
  })

  describe('Theme Transitions', () => {
    it('should update label when theme changes from light to dark', () => {
      themeMode.value = themes.light

      const { rerender } = render(<ThemeToggle />)
      expect(screen.getByText('Light')).toBeInTheDocument()

      themeMode.value = themes.dark
      rerender(<ThemeToggle />)
      expect(screen.getByText('Dark')).toBeInTheDocument()
    })

    it('should update label when theme changes from dark to auto', () => {
      themeMode.value = themes.dark

      const { rerender } = render(<ThemeToggle />)
      expect(screen.getByText('Dark')).toBeInTheDocument()

      themeMode.value = themes.auto
      rerender(<ThemeToggle />)
      expect(screen.getByText('Auto')).toBeInTheDocument()
    })

    it('should update icon when theme changes', () => {
      themeMode.value = themes.light
      appliedTheme.value = themes.light

      const { rerender } = render(<ThemeToggle />)

      const sunIcon = document.querySelector('path[d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"]')
      expect(sunIcon).toBeInTheDocument()

      themeMode.value = themes.dark
      appliedTheme.value = themes.dark
      rerender(<ThemeToggle />)

      const moonIcon = document.querySelector('path[d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"]')
      expect(moonIcon).toBeInTheDocument()
    })

    it('should update title when theme changes', () => {
      themeMode.value = themes.light

      const { rerender } = render(<ThemeToggle />)
      const button = document.querySelector('button')
      expect(button).toHaveAttribute('title', 'Theme: Light')

      themeMode.value = themes.auto
      rerender(<ThemeToggle />)
      expect(button).toHaveAttribute('title', 'Theme: Auto')
    })
  })

  describe('Label Styling', () => {
    it('should have proper label text styling', () => {
      render(<ThemeToggle />)

      const label = screen.getByText('Light')
      expect(label).toHaveClass('text-sm')
      expect(label).toHaveClass('font-medium')
      expect(label).toHaveClass('text-gray-700')
      expect(label).toHaveClass('dark:text-gray-300')
    })
  })

  describe('Icon Styling', () => {
    it('should render SVG with proper attributes', () => {
      render(<ThemeToggle />)

      const svg = document.querySelector('svg')
      expect(svg).toHaveAttribute('fill', 'none')
      expect(svg).toHaveAttribute('stroke', 'currentColor')
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24')
    })

    it('should render path with proper stroke attributes', () => {
      render(<ThemeToggle />)

      const path = document.querySelector('path')
      expect(path).toHaveAttribute('stroke-linecap', 'round')
      expect(path).toHaveAttribute('stroke-linejoin', 'round')
      expect(path).toHaveAttribute('stroke-width', '2')
    })
  })

  describe('Accessibility', () => {
    it('should be keyboard accessible', () => {
      render(<ThemeToggle />)

      const button = document.querySelector('button')
      expect(button.tagName).toBe('BUTTON')
    })

    it('should have descriptive title for screen readers', () => {
      themeMode.value = themes.light

      render(<ThemeToggle />)

      const button = document.querySelector('button')
      expect(button).toHaveAttribute('title')
      expect(button.getAttribute('title')).toContain('Theme')
    })
  })
})
