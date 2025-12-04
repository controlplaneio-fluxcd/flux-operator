// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { FavoritesSearch } from './FavoritesSearch'

describe('FavoritesSearch component', () => {
  const mockOnFilter = vi.fn()
  const mockOnClose = vi.fn()
  const mockNamespaces = ['flux-system', 'default', 'kube-system']
  const mockKinds = ['FluxInstance', 'ResourceSet', 'Kustomization']

  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('rendering', () => {
    it('should render search input', () => {
      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      expect(screen.getByPlaceholderText('Search... (filter with ns: or kind:)')).toBeInTheDocument()
    })

    it('should render close button', () => {
      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      expect(screen.getByLabelText('Close search')).toBeInTheDocument()
    })

    it('should focus input on mount', async () => {
      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      await waitFor(() => {
        const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
        expect(document.activeElement).toBe(input)
      })
    })
  })

  describe('text search', () => {
    it('should call onFilter with name when typing regular text', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'flux')

      expect(mockOnFilter).toHaveBeenLastCalledWith({
        namespace: null,
        kind: null,
        name: 'flux'
      })
    })
  })

  describe('namespace filter', () => {
    it('should show namespace suggestions when typing ns:', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'ns:')

      expect(screen.getByText('Select namespace')).toBeInTheDocument()
      expect(screen.getByText('flux-system')).toBeInTheDocument()
      expect(screen.getByText('default')).toBeInTheDocument()
      expect(screen.getByText('kube-system')).toBeInTheDocument()
    })

    it('should filter namespace suggestions based on input', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'ns:flux')

      expect(screen.getByText('flux-system')).toBeInTheDocument()
      expect(screen.queryByText('default')).not.toBeInTheDocument()
    })

    it('should add namespace badge when suggestion is clicked', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'ns:')

      const fluxSystem = screen.getByText('flux-system')
      await user.click(fluxSystem)

      expect(screen.getByText('ns:flux-system')).toBeInTheDocument()
      expect(mockOnFilter).toHaveBeenLastCalledWith({
        namespace: 'flux-system',
        kind: null,
        name: ''
      })
    })

    it('should show "No matching namespaces" when no matches', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'ns:nonexistent')

      expect(screen.getByText('No matching namespaces')).toBeInTheDocument()
    })
  })

  describe('kind filter', () => {
    it('should show kind suggestions when typing kind:', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'kind:')

      expect(screen.getByText('Select kind')).toBeInTheDocument()
      expect(screen.getByText('FluxInstance')).toBeInTheDocument()
      expect(screen.getByText('ResourceSet')).toBeInTheDocument()
      expect(screen.getByText('Kustomization')).toBeInTheDocument()
    })

    it('should filter kind suggestions based on input', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'kind:flux')

      expect(screen.getByText('FluxInstance')).toBeInTheDocument()
      expect(screen.queryByText('ResourceSet')).not.toBeInTheDocument()
    })

    it('should add kind badge when suggestion is clicked', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'kind:')

      const fluxInstance = screen.getByText('FluxInstance')
      await user.click(fluxInstance)

      expect(screen.getByText('kind:FluxInstance')).toBeInTheDocument()
      expect(mockOnFilter).toHaveBeenLastCalledWith({
        namespace: null,
        kind: 'FluxInstance',
        name: ''
      })
    })

    it('should show "No matching kinds" when no matches', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'kind:nonexistent')

      expect(screen.getByText('No matching kinds')).toBeInTheDocument()
    })
  })

  describe('multiple filters', () => {
    it('should support both namespace and kind filters simultaneously', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')

      // Add namespace filter
      await user.type(input, 'ns:')
      await user.click(screen.getByText('flux-system'))

      // Add kind filter
      await user.type(input, 'kind:')
      await user.click(screen.getByText('FluxInstance'))

      expect(screen.getByText('ns:flux-system')).toBeInTheDocument()
      expect(screen.getByText('kind:FluxInstance')).toBeInTheDocument()

      expect(mockOnFilter).toHaveBeenLastCalledWith({
        namespace: 'flux-system',
        kind: 'FluxInstance',
        name: ''
      })
    })

    it('should update placeholder when filters are active', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'ns:')
      await user.click(screen.getByText('flux-system'))

      expect(screen.getByPlaceholderText('Search...')).toBeInTheDocument()
    })
  })

  describe('keyboard navigation', () => {
    it('should close search on Escape key', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, '{Escape}')

      expect(mockOnClose).toHaveBeenCalled()
    })

    it('should navigate suggestions with arrow keys', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'ns:')

      // Navigate down
      await user.keyboard('{ArrowDown}')

      // First item should be highlighted
      const buttons = screen.getAllByRole('button')
      const firstSuggestion = buttons.find(btn => btn.textContent === 'flux-system')
      expect(firstSuggestion).toHaveClass('bg-gray-100')
    })

    it('should select suggestion on Enter key', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')
      await user.type(input, 'ns:')
      await user.keyboard('{ArrowDown}{Enter}')

      expect(screen.getByText('ns:flux-system')).toBeInTheDocument()
    })

    it('should remove last filter badge on Backspace when input is empty', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const input = screen.getByPlaceholderText('Search... (filter with ns: or kind:)')

      // Add namespace filter
      await user.type(input, 'ns:')
      await user.click(screen.getByText('flux-system'))

      // Add kind filter
      await user.type(input, 'kind:')
      await user.click(screen.getByText('FluxInstance'))

      // Both badges should be visible
      expect(screen.getByText('ns:flux-system')).toBeInTheDocument()
      expect(screen.getByText('kind:FluxInstance')).toBeInTheDocument()

      // Press backspace to remove last added filter (kind)
      await user.keyboard('{Backspace}')

      // Kind badge should be removed (LIFO)
      expect(screen.getByText('ns:flux-system')).toBeInTheDocument()
      expect(screen.queryByText('kind:FluxInstance')).not.toBeInTheDocument()

      // Press backspace again to remove namespace
      await user.keyboard('{Backspace}')
      expect(screen.queryByText('ns:flux-system')).not.toBeInTheDocument()
    })
  })

  describe('close button', () => {
    it('should call onClose when close button is clicked', async () => {
      const user = userEvent.setup()

      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={mockKinds}
        />
      )

      const closeButton = screen.getByLabelText('Close search')
      await user.click(closeButton)

      expect(mockOnClose).toHaveBeenCalled()
    })
  })

  describe('edge cases', () => {
    it('should handle empty namespaces array', () => {
      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={[]}
          kinds={mockKinds}
        />
      )

      expect(screen.getByPlaceholderText('Search... (filter with ns: or kind:)')).toBeInTheDocument()
    })

    it('should handle empty kinds array', () => {
      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
          namespaces={mockNamespaces}
          kinds={[]}
        />
      )

      expect(screen.getByPlaceholderText('Search... (filter with ns: or kind:)')).toBeInTheDocument()
    })

    it('should handle undefined namespaces and kinds', () => {
      render(
        <FavoritesSearch
          onFilter={mockOnFilter}
          onClose={mockOnClose}
        />
      )

      expect(screen.getByPlaceholderText('Search... (filter with ns: or kind:)')).toBeInTheDocument()
    })
  })
})
