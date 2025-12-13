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
      expect(footer).toHaveClass('sm:border-t')
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
      expect(container).toHaveClass('py-3')
      expect(container).toHaveClass('sm:py-4')
    })

    it('should have mobile and desktop layouts', () => {
      render(<Footer />)

      // Mobile layout
      const mobileContainer = document.querySelector('.flex.sm\\:hidden')
      expect(mobileContainer).toBeInTheDocument()

      // Desktop layout
      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      expect(desktopContainer).toBeInTheDocument()
    })
  })

  describe('Source Code Link', () => {
    it('should render source code GitHub link in both layouts', () => {
      render(<Footer />)

      const links = screen.getAllByText('Source code')
      expect(links.length).toBe(2) // One for mobile, one for desktop
      links.forEach(linkText => {
        const link = linkText.closest('a')
        expect(link).toHaveAttribute('href', 'https://github.com/controlplaneio-fluxcd/flux-operator')
      })
    })

    it('should open in new tab with security attributes', () => {
      render(<Footer />)

      const links = screen.getAllByText('Source code')
      links.forEach(linkText => {
        const link = linkText.closest('a')
        expect(link).toHaveAttribute('target', '_blank')
        expect(link).toHaveAttribute('rel', 'noopener noreferrer')
      })
    })

    it('should have hover styles', () => {
      render(<Footer />)

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      const link = desktopContainer.querySelector('a[href*="github"]')
      expect(link).toHaveClass('text-gray-600')
      expect(link).toHaveClass('dark:text-gray-400')
      expect(link).toHaveClass('hover:text-gray-900')
      expect(link).toHaveClass('dark:hover:text-white')
      expect(link).toHaveClass('transition-colors')
    })
  })

  describe('Documentation Link', () => {
    it('should render documentation link in desktop only', () => {
      render(<Footer />)

      const link = screen.getByText('Documentation').closest('a')
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute('href', 'https://fluxoperator.dev')
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
    it('should render enterprise support email link in both layouts', () => {
      render(<Footer />)

      const links = screen.getAllByText('Enterprise Support')
      expect(links.length).toBe(2) // One for mobile, one for desktop
      links.forEach(linkText => {
        const link = linkText.closest('a')
        expect(link).toHaveAttribute('href', 'mailto:flux-enterprise@control-plane.io')
      })
    })

    it('should have proper icon path in desktop layout', () => {
      render(<Footer />)

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      const link = desktopContainer.querySelector('a[href*="mailto"]')
      const path = link.querySelector('path')
      expect(path).toHaveAttribute('d', 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z')
    })

    it('should not open in new tab (email link)', () => {
      render(<Footer />)

      const links = screen.getAllByText('Enterprise Support')
      links.forEach(linkText => {
        const link = linkText.closest('a')
        expect(link).not.toHaveAttribute('target')
      })
    })
  })

  describe('Separators', () => {
    it('should render bullet separators in desktop layout', () => {
      render(<Footer />)

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      const separators = desktopContainer.querySelectorAll('.text-gray-300.dark\\:text-gray-600')
      expect(separators.length).toBeGreaterThan(0)
    })

    it('should render bullet separator in mobile layout', () => {
      render(<Footer />)

      const mobileContainer = document.querySelector('.flex.sm\\:hidden')
      const separator = mobileContainer.querySelector('.text-gray-300.dark\\:text-gray-600')
      expect(separator).toBeInTheDocument()
    })
  })

  describe('License Information', () => {
    it('should display AGPL-3.0 license in desktop only', () => {
      render(<Footer />)

      expect(screen.getByText('AGPL-3.0 Licensed')).toBeInTheDocument()
    })

    it('should have proper license text styling', () => {
      render(<Footer />)

      const licenseContainer = screen.getByText('AGPL-3.0 Licensed').closest('div')
      expect(licenseContainer).toHaveClass('text-gray-600')
      expect(licenseContainer).toHaveClass('dark:text-gray-400')
    })

    it('should be right-aligned in desktop layout', () => {
      render(<Footer />)

      const licenseContainer = screen.getByText('AGPL-3.0 Licensed').closest('div')
      expect(licenseContainer).toHaveClass('text-sm')
      expect(licenseContainer).toHaveClass('text-right')
    })
  })

  describe('Responsive Layout', () => {
    it('should have centered mobile layout', () => {
      render(<Footer />)

      const mobileContainer = document.querySelector('.flex.sm\\:hidden')
      expect(mobileContainer).toHaveClass('items-center')
      expect(mobileContainer).toHaveClass('justify-center')
      expect(mobileContainer).toHaveClass('gap-4')
      expect(mobileContainer).toHaveClass('text-xs')
    })

    it('should have proper desktop layout', () => {
      render(<Footer />)

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      expect(desktopContainer).toHaveClass('flex-row')
      expect(desktopContainer).toHaveClass('items-center')
      expect(desktopContainer).toHaveClass('justify-between')
      expect(desktopContainer).toHaveClass('gap-4')
    })

    it('should have responsive link container in desktop', () => {
      render(<Footer />)

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      const linksContainer = desktopContainer.querySelector('.flex.flex-row.items-center.gap-6')
      expect(linksContainer).toBeInTheDocument()
      expect(linksContainer).toHaveClass('text-sm')
    })
  })

  describe('Link Styling', () => {
    it('should have consistent link styles in desktop layout', () => {
      render(<Footer />)

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      const links = desktopContainer.querySelectorAll('a')

      links.forEach(link => {
        expect(link).toHaveClass('flex')
        expect(link).toHaveClass('items-center')
        expect(link).toHaveClass('hover:text-gray-900')
        expect(link).toHaveClass('dark:hover:text-white')
        expect(link).toHaveClass('transition-colors')
      })
    })

    it('should have text-sm for desktop links', () => {
      render(<Footer />)

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      const linksContainer = desktopContainer.querySelector('.flex.flex-row.items-center.gap-6')
      expect(linksContainer).toHaveClass('text-sm')
    })

    it('should have text-xs for mobile links', () => {
      render(<Footer />)

      const mobileContainer = document.querySelector('.flex.sm\\:hidden')
      expect(mobileContainer).toHaveClass('text-xs')
    })
  })

  describe('Icons', () => {
    it('should render SVG icons in desktop layout with proper attributes', () => {
      render(<Footer />)

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      const svgs = desktopContainer.querySelectorAll('svg')
      // Should have 3 SVGs (external link icon, document icon and email icon)
      expect(svgs.length).toBe(3)

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

      const desktopContainer = document.querySelector('.hidden.sm\\:flex')
      const paths = desktopContainer.querySelectorAll('path')
      paths.forEach(path => {
        expect(path).toHaveAttribute('stroke-linecap', 'round')
        expect(path).toHaveAttribute('stroke-linejoin', 'round')
        expect(path).toHaveAttribute('stroke-width', '2')
      })
    })
  })

})
