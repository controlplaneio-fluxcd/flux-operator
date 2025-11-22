// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { Footer } from './Footer'
import { themeMode, appliedTheme, themes } from '../../utils/theme'

describe('Footer', () => {
  beforeEach(() => {
    // Reset theme to default
    themeMode.value = themes.light
    appliedTheme.value = themes.light
  })

  describe('Component Rendering', () => {
    it('should render a footer element', () => {
      render(<Footer />)

      const footer = document.querySelector('footer')
      expect(footer).toBeInTheDocument()
    })

    it('should have proper styling classes', () => {
      render(<Footer />)

      const footer = document.querySelector('footer')
      expect(footer).toHaveClass('bg-white')
      expect(footer).toHaveClass('dark:bg-gray-800')
      expect(footer).toHaveClass('border-t')
      expect(footer).toHaveClass('border-gray-200')
      expect(footer).toHaveClass('dark:border-gray-700')
      expect(footer).toHaveClass('transition-colors')
      expect(footer).toHaveClass('mt-8')
    })

    it('should have responsive container', () => {
      render(<Footer />)

      const container = document.querySelector('.max-w-7xl')
      expect(container).toBeInTheDocument()
      expect(container).toHaveClass('mx-auto')
      expect(container).toHaveClass('px-4')
      expect(container).toHaveClass('sm:px-6')
      expect(container).toHaveClass('lg:px-8')
      expect(container).toHaveClass('py-4')
    })

    it('should have flex layout', () => {
      render(<Footer />)

      const flexContainer = document.querySelector('.flex.flex-col')
      expect(flexContainer).toBeInTheDocument()
    })
  })

  describe('Flux Operator Link', () => {
    it('should render Flux Operator GitHub link', () => {
      render(<Footer />)

      const link = screen.getByText('Flux Operator').closest('a')
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute('href', 'https://github.com/controlplaneio-fluxcd/flux-operator')
    })

    it('should open in new tab with security attributes', () => {
      render(<Footer />)

      const link = screen.getByText('Flux Operator').closest('a')
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    })

    it('should have Flux logo icon', () => {
      render(<Footer />)

      const link = screen.getByText('Flux Operator').closest('a')
      const img = link.querySelector('img')
      expect(img).toBeInTheDocument()
      expect(img).toHaveAttribute('alt', 'Flux')
    })

    it('should use black logo in light theme', () => {
      themeMode.value = themes.light
      appliedTheme.value = themes.light

      render(<Footer />)

      const link = screen.getByText('Flux Operator').closest('a')
      const img = link.querySelector('img')
      expect(img).toHaveAttribute('src', '/flux-icon-black.svg')
    })

    it('should use white logo in dark theme', () => {
      themeMode.value = themes.dark
      appliedTheme.value = themes.dark

      render(<Footer />)

      const link = screen.getByText('Flux Operator').closest('a')
      const img = link.querySelector('img')
      expect(img).toHaveAttribute('src', '/flux-icon-white.svg')
    })

    it('should have proper logo size', () => {
      render(<Footer />)

      const link = screen.getByText('Flux Operator').closest('a')
      const img = link.querySelector('img')
      expect(img).toHaveClass('w-4')
      expect(img).toHaveClass('h-4')
    })

    it('should have hover styles', () => {
      render(<Footer />)

      const link = screen.getByText('Flux Operator').closest('a')
      expect(link).toHaveClass('text-gray-600')
      expect(link).toHaveClass('dark:text-gray-400')
      expect(link).toHaveClass('hover:text-gray-900')
      expect(link).toHaveClass('dark:hover:text-white')
      expect(link).toHaveClass('transition-colors')
    })
  })

  describe('Documentation Link', () => {
    it('should render documentation link', () => {
      render(<Footer />)

      const link = screen.getByText('Documentation').closest('a')
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute('href', 'https://fluxcd.control-plane.io')
    })

    it('should open in new tab with security attributes', () => {
      render(<Footer />)

      const link = screen.getByText('Documentation').closest('a')
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    })

    it('should have document icon', () => {
      render(<Footer />)

      const link = screen.getByText('Documentation').closest('a')
      const svg = link.querySelector('svg')
      expect(svg).toBeInTheDocument()
      expect(svg).toHaveClass('w-4')
      expect(svg).toHaveClass('h-4')
    })

    it('should have proper icon path', () => {
      render(<Footer />)

      const link = screen.getByText('Documentation').closest('a')
      const path = link.querySelector('path')
      expect(path).toHaveAttribute('d', 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z')
    })
  })

  describe('Enterprise Support Link', () => {
    it('should render enterprise support email link', () => {
      render(<Footer />)

      const link = screen.getByText('Enterprise Support').closest('a')
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute('href', 'mailto:flux-enterprise@control-plane.io')
    })

    it('should have email icon', () => {
      render(<Footer />)

      const link = screen.getByText('Enterprise Support').closest('a')
      const svg = link.querySelector('svg')
      expect(svg).toBeInTheDocument()
      expect(svg).toHaveClass('w-4')
      expect(svg).toHaveClass('h-4')
    })

    it('should have proper icon path', () => {
      render(<Footer />)

      const link = screen.getByText('Enterprise Support').closest('a')
      const path = link.querySelector('path')
      expect(path).toHaveAttribute('d', 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z')
    })

    it('should not open in new tab (email link)', () => {
      render(<Footer />)

      const link = screen.getByText('Enterprise Support').closest('a')
      expect(link).not.toHaveAttribute('target')
    })
  })

  describe('Separators', () => {
    it('should render bullet separators between links', () => {
      render(<Footer />)

      const separators = document.querySelectorAll('.text-gray-300.dark\\:text-gray-600.hidden.sm\\:inline')
      expect(separators.length).toBeGreaterThan(0)
    })

    it('should hide separators on mobile', () => {
      render(<Footer />)

      const separator = document.querySelector('.hidden.sm\\:inline')
      expect(separator).toBeInTheDocument()
      expect(separator).toHaveClass('hidden')
      expect(separator).toHaveClass('sm:inline')
    })
  })

  describe('License Information', () => {
    it('should display AGPL-3.0 license', () => {
      render(<Footer />)

      expect(screen.getByText('AGPL-3.0 Licensed')).toBeInTheDocument()
    })

    it('should have proper license text styling', () => {
      render(<Footer />)

      const licenseContainer = screen.getByText('AGPL-3.0 Licensed').closest('div')
      expect(licenseContainer).toHaveClass('text-gray-600')
      expect(licenseContainer).toHaveClass('dark:text-gray-400')
    })

    it('should be right-aligned on desktop', () => {
      render(<Footer />)

      const licenseContainer = screen.getByText('AGPL-3.0 Licensed').closest('div')
      expect(licenseContainer).toHaveClass('text-xs')
      expect(licenseContainer).toHaveClass('sm:text-sm')
      expect(licenseContainer).toHaveClass('text-center')
      expect(licenseContainer).toHaveClass('sm:text-right')
    })
  })

  describe('Responsive Layout', () => {
    it('should have mobile-first column layout', () => {
      render(<Footer />)

      const mainFlex = document.querySelector('.flex.flex-col.sm\\:flex-row')
      expect(mainFlex).toBeInTheDocument()
      expect(mainFlex).toHaveClass('flex-col')
      expect(mainFlex).toHaveClass('sm:flex-row')
    })

    it('should have proper spacing and alignment', () => {
      render(<Footer />)

      const mainFlex = document.querySelector('.flex.flex-col.sm\\:flex-row')
      expect(mainFlex).toHaveClass('items-start')
      expect(mainFlex).toHaveClass('sm:items-center')
      expect(mainFlex).toHaveClass('justify-between')
      expect(mainFlex).toHaveClass('gap-4')
    })

    it('should have responsive link container', () => {
      render(<Footer />)

      const linksContainer = screen.getByText('Flux Operator').closest('div').parentElement
      expect(linksContainer).toHaveClass('flex')
      expect(linksContainer).toHaveClass('flex-col')
      expect(linksContainer).toHaveClass('sm:flex-row')
      expect(linksContainer).toHaveClass('sm:items-center')
    })
  })

  describe('Link Styling', () => {
    it('should have consistent link styles', () => {
      render(<Footer />)

      const links = [
        screen.getByText('Flux Operator').closest('a'),
        screen.getByText('Documentation').closest('a'),
        screen.getByText('Enterprise Support').closest('a')
      ]

      links.forEach(link => {
        expect(link).toHaveClass('flex')
        expect(link).toHaveClass('items-center')
        expect(link).toHaveClass('gap-2')
        expect(link).toHaveClass('text-gray-600')
        expect(link).toHaveClass('dark:text-gray-400')
        expect(link).toHaveClass('hover:text-gray-900')
        expect(link).toHaveClass('dark:hover:text-white')
        expect(link).toHaveClass('transition-colors')
      })
    })

    it('should have text-sm for all links', () => {
      render(<Footer />)

      const linksContainer = screen.getByText('Flux Operator').closest('div')
      expect(linksContainer).toHaveClass('text-sm')
    })
  })

  describe('Icons', () => {
    it('should render all SVG icons with proper attributes', () => {
      render(<Footer />)

      const svgs = document.querySelectorAll('svg')
      // Should have 2 SVGs (document icon and email icon) + 1 image for Flux logo
      expect(svgs.length).toBeGreaterThanOrEqual(2)

      svgs.forEach(svg => {
        expect(svg).toHaveAttribute('fill', 'none')
        expect(svg).toHaveAttribute('stroke', 'currentColor')
        expect(svg).toHaveAttribute('viewBox', '0 0 24 24')
      })
    })

    it('should have flex-shrink-0 on icons', () => {
      render(<Footer />)

      const documentIcon = screen.getByText('Documentation').closest('a').querySelector('svg')
      expect(documentIcon).toHaveClass('flex-shrink-0')
    })

    it('should have proper stroke attributes on paths', () => {
      render(<Footer />)

      const paths = document.querySelectorAll('path')
      paths.forEach(path => {
        expect(path).toHaveAttribute('stroke-linecap', 'round')
        expect(path).toHaveAttribute('stroke-linejoin', 'round')
        expect(path).toHaveAttribute('stroke-width', '2')
      })
    })
  })

  describe('Theme Integration', () => {
    it('should use correct logo for light theme', () => {
      themeMode.value = themes.light
      appliedTheme.value = themes.light

      render(<Footer />)
      const link = screen.getByText('Flux Operator').closest('a')
      const img = link.querySelector('img')
      expect(img).toHaveAttribute('src', '/flux-icon-black.svg')
    })

    it('should use correct logo for dark theme', () => {
      themeMode.value = themes.dark
      appliedTheme.value = themes.dark

      render(<Footer />)
      const link = screen.getByText('Flux Operator').closest('a')
      const img = link.querySelector('img')
      expect(img).toHaveAttribute('src', '/flux-icon-white.svg')
    })
  })
})
