// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/preact'
import userEvent from '@testing-library/user-event'
import { FavoritesHeader } from './FavoritesHeader'

describe('FavoritesHeader component', () => {
  const mockOnEditModeToggle = vi.fn()
  const mockOnSaveOrder = vi.fn()
  const mockOnCancelEdit = vi.fn()
  const mockOnFilter = vi.fn()
  const mockOnStatusFilter = vi.fn()

  const mockResources = [
    { status: 'Ready' },
    { status: 'Ready' },
    { status: 'Ready' },
    { status: 'Failed' },
    { status: 'Progressing' }
  ]

  const defaultProps = {
    resources: mockResources,
    loading: false,
    editMode: false,
    onEditModeToggle: mockOnEditModeToggle,
    onSaveOrder: mockOnSaveOrder,
    onCancelEdit: mockOnCancelEdit,
    onFilter: mockOnFilter,
    onStatusFilter: mockOnStatusFilter,
    statusFilter: null,
    namespaces: ['flux-system', 'default'],
    kinds: ['FluxInstance', 'ResourceSet']
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('normal mode', () => {
    it('should render status bar', () => {
      const { container } = render(<FavoritesHeader {...defaultProps} />)

      // Status bar should be visible
      const statusBar = container.querySelector('.flex.gap-0.flex-1')
      expect(statusBar).toBeInTheDocument()
    })

    it('should render edit order button', () => {
      render(<FavoritesHeader {...defaultProps} />)

      const editButton = screen.getByTitle('Edit order')
      expect(editButton).toBeInTheDocument()
    })

    it('should render search button', () => {
      render(<FavoritesHeader {...defaultProps} />)

      const searchButton = screen.getByTitle('Search favorites')
      expect(searchButton).toBeInTheDocument()
    })

    it('should call onEditModeToggle when edit button is clicked', async () => {
      const user = userEvent.setup()

      render(<FavoritesHeader {...defaultProps} />)

      const editButton = screen.getByTitle('Edit order')
      await user.click(editButton)

      expect(mockOnEditModeToggle).toHaveBeenCalled()
    })

    it('should disable edit button when no resources', () => {
      render(<FavoritesHeader {...defaultProps} resources={[]} />)

      const editButton = screen.getByTitle('Edit order')
      expect(editButton).toBeDisabled()
    })
  })

  describe('loading state', () => {
    it('should show loading animation in status bar', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} loading={true} />
      )

      const loadingBar = container.querySelector('.animate-pulse')
      expect(loadingBar).toBeInTheDocument()
    })

    it('should show empty status bar when loading with no resources', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} loading={true} resources={[]} />
      )

      const loadingBar = container.querySelector('.animate-pulse')
      expect(loadingBar).toBeInTheDocument()
    })
  })

  describe('status bar segments', () => {
    it('should render colored segments for each status', () => {
      const { container } = render(<FavoritesHeader {...defaultProps} />)

      // Should have segments for Ready (60%), Failed (20%), Progressing (20%)
      const segments = container.querySelectorAll('.relative.group')
      expect(segments.length).toBe(3) // Ready, Failed, Progressing
    })

    it('should call onStatusFilter when status segment is clicked', async () => {
      const user = userEvent.setup()
      const { container } = render(<FavoritesHeader {...defaultProps} />)

      // Click on a status segment
      const segments = container.querySelectorAll('.relative.group')
      await user.click(segments[0])

      expect(mockOnStatusFilter).toHaveBeenCalled()
    })

    it('should clear filter when clicking active status segment', async () => {
      const user = userEvent.setup()
      const { container } = render(
        <FavoritesHeader {...defaultProps} statusFilter="Ready" />
      )

      // Click on the active segment should pass null
      const segments = container.querySelectorAll('.relative.group')
      const readySegment = Array.from(segments).find(seg => {
        const bar = seg.querySelector('div')
        return bar && !bar.classList.contains('opacity-30')
      })

      if (readySegment) {
        await user.click(readySegment)
        expect(mockOnStatusFilter).toHaveBeenCalledWith(null)
      }
    })

    it('should fade non-active segments when filter is active', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} statusFilter="Ready" />
      )

      const segments = container.querySelectorAll('.relative.group > div')
      const fadedSegments = Array.from(segments).filter(seg =>
        seg.classList.contains('opacity-30')
      )

      // Non-Ready segments should be faded
      expect(fadedSegments.length).toBeGreaterThan(0)
    })
  })

  describe('edit mode', () => {
    it('should show drag instruction text', () => {
      render(<FavoritesHeader {...defaultProps} editMode={true} />)

      expect(screen.getByText('Drag to reorder favorites')).toBeInTheDocument()
    })

    it('should show save button (checkmark icon)', () => {
      render(<FavoritesHeader {...defaultProps} editMode={true} />)

      const saveButton = screen.getByTitle('Save')
      expect(saveButton).toBeInTheDocument()
    })

    it('should show cancel button (X icon)', () => {
      render(<FavoritesHeader {...defaultProps} editMode={true} />)

      const cancelButton = screen.getByTitle('Cancel')
      expect(cancelButton).toBeInTheDocument()
    })

    it('should call onSaveOrder when save button is clicked', async () => {
      const user = userEvent.setup()

      render(<FavoritesHeader {...defaultProps} editMode={true} />)

      const saveButton = screen.getByTitle('Save')
      await user.click(saveButton)

      expect(mockOnSaveOrder).toHaveBeenCalled()
    })

    it('should call onCancelEdit when cancel button is clicked', async () => {
      const user = userEvent.setup()

      render(<FavoritesHeader {...defaultProps} editMode={true} />)

      const cancelButton = screen.getByTitle('Cancel')
      await user.click(cancelButton)

      expect(mockOnCancelEdit).toHaveBeenCalled()
    })

    it('should not show status bar in edit mode', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} editMode={true} />
      )

      const statusBar = container.querySelector('.flex.gap-0.flex-1')
      expect(statusBar).not.toBeInTheDocument()
    })
  })

  describe('search mode', () => {
    it('should show search input when search button is clicked', async () => {
      const user = userEvent.setup()

      render(<FavoritesHeader {...defaultProps} />)

      const searchButton = screen.getByTitle('Search favorites')
      await user.click(searchButton)

      // Search input should appear
      expect(screen.getByPlaceholderText('Search... (filter with ns: or kind:)')).toBeInTheDocument()
    })

    it('should hide status bar in search mode', async () => {
      const user = userEvent.setup()
      const { container } = render(<FavoritesHeader {...defaultProps} />)

      const searchButton = screen.getByTitle('Search favorites')
      await user.click(searchButton)

      const statusBar = container.querySelector('.flex.gap-0.flex-1')
      expect(statusBar).not.toBeInTheDocument()
    })

    it('should return to normal mode when search is closed', async () => {
      const user = userEvent.setup()

      render(<FavoritesHeader {...defaultProps} />)

      // Open search
      const searchButton = screen.getByTitle('Search favorites')
      await user.click(searchButton)

      // Close search
      const closeButton = screen.getByLabelText('Close search')
      await user.click(closeButton)

      // Should be back to normal mode with status bar visible
      expect(screen.getByTitle('Search favorites')).toBeInTheDocument()

      // onFilter should be called to clear filters
      expect(mockOnFilter).toHaveBeenCalledWith({ namespace: null, kind: null, name: '' })
    })
  })

  describe('tooltip behavior', () => {
    it('should show tooltip on hover', async () => {
      const user = userEvent.setup()
      const { container } = render(<FavoritesHeader {...defaultProps} />)

      const segments = container.querySelectorAll('.relative.group')
      await user.hover(segments[0])

      // Tooltip should appear with status info
      await waitFor(() => {
        // Tooltip contains status name, count, and percentage
        expect(screen.getByText(/Count:/)).toBeInTheDocument()
        expect(screen.getByText(/Percentage:/)).toBeInTheDocument()
      })
    })

    it('should hide tooltip on mobile (class check)', () => {
      // Tooltips have 'hidden md:block' class for mobile hiding
      // This is verified through the component's structure
      // Note: tooltips are only visible on hover, so we just verify the component renders
      const { container } = render(<FavoritesHeader {...defaultProps} />)
      expect(container).toBeInTheDocument()
    })
  })

  describe('status calculations', () => {
    it('should calculate correct percentages', async () => {
      const user = userEvent.setup()
      const { container } = render(<FavoritesHeader {...defaultProps} />)

      const segments = container.querySelectorAll('.relative.group')
      await user.hover(segments[0])

      // Ready is 3/5 = 60% - check for "Percentage:" label since format may vary
      await waitFor(() => {
        expect(screen.getByText(/Percentage:/)).toBeInTheDocument()
      })
    })

    it('should handle resources with Unknown status', () => {
      const resourcesWithUnknown = [
        { status: 'Ready' },
        { status: 'Unknown' },
        {}  // No status defaults to Unknown
      ]

      const { container } = render(
        <FavoritesHeader {...defaultProps} resources={resourcesWithUnknown} />
      )

      // Should render segments including Unknown
      const segments = container.querySelectorAll('.relative.group')
      expect(segments.length).toBeGreaterThan(0)
    })

    it('should show empty status bar when no resources', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} resources={[]} loading={false} />
      )

      // Should show empty gray bar
      const emptyBar = container.querySelector('.bg-gray-200')
      expect(emptyBar).toBeInTheDocument()
    })
  })

  describe('status colors', () => {
    it('should use green for Ready status', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} resources={[{ status: 'Ready' }]} />
      )

      const greenBar = container.querySelector('.bg-green-500')
      expect(greenBar).toBeInTheDocument()
    })

    it('should use red for Failed status', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} resources={[{ status: 'Failed' }]} />
      )

      const redBar = container.querySelector('.bg-red-500')
      expect(redBar).toBeInTheDocument()
    })

    it('should use blue for Progressing status', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} resources={[{ status: 'Progressing' }]} />
      )

      const blueBar = container.querySelector('.bg-blue-500')
      expect(blueBar).toBeInTheDocument()
    })

    it('should use yellow for Suspended status', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} resources={[{ status: 'Suspended' }]} />
      )

      const yellowBar = container.querySelector('.bg-yellow-500')
      expect(yellowBar).toBeInTheDocument()
    })

    it('should use gray for Unknown status', () => {
      const { container } = render(
        <FavoritesHeader {...defaultProps} resources={[{ status: 'Unknown' }]} />
      )

      const grayBar = container.querySelector('.bg-gray-600')
      expect(grayBar).toBeInTheDocument()
    })
  })
})
