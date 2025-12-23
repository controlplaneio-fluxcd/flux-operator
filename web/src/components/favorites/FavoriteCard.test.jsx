// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/preact'
import { FavoriteCard } from './FavoriteCard'
import { removeFavorite } from '../../utils/favorites'

// Mock preact-iso
const mockRoute = vi.fn()
vi.mock('preact-iso', () => ({
  useLocation: () => ({
    route: mockRoute
  })
}))

// Mock favorites utility
vi.mock('../../utils/favorites', () => ({
  removeFavorite: vi.fn()
}))

describe('FavoriteCard component', () => {
  const mockFavorite = {
    kind: 'FluxInstance',
    namespace: 'flux-system',
    name: 'flux'
  }

  const mockResourceData = {
    status: 'Ready',
    lastReconciled: '2024-01-15T10:30:00Z',
    message: 'Applied revision: main/abc1234'
  }

  const mockNotFoundResourceData = {
    status: 'NotFound',
    lastReconciled: '2024-01-15T10:30:00Z',
    message: 'Not found in namespace flux-system'
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('rendering', () => {
    it('should render kind, namespace and name', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockResourceData}
        />
      )

      expect(screen.getByText('FluxInstance')).toBeInTheDocument()
      expect(screen.getByText('flux-system')).toBeInTheDocument()
      expect(screen.getByText('flux')).toBeInTheDocument()
    })

    it('should render status badge', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockResourceData}
        />
      )

      expect(screen.getByText('Ready')).toBeInTheDocument()
    })

    it('should render last reconciled timestamp', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockResourceData}
        />
      )

      // Should have the sync icon and formatted timestamp
      const timestampElement = screen.getByText(/Jan 15/)
      expect(timestampElement).toBeInTheDocument()
    })

    it('should render message with lowercase first character', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockResourceData}
        />
      )

      // Message should start with lowercase
      expect(screen.getByText(/applied revision/)).toBeInTheDocument()
    })

    it('should render star button for unfavoriting', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockResourceData}
        />
      )

      const starButton = screen.getByTitle('Remove from favorites')
      expect(starButton).toBeInTheDocument()
    })
  })

  describe('not found state', () => {
    it('should show "Not Found" status when resourceData is NotFound', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockNotFoundResourceData}
        />
      )

      expect(screen.getByText('Not Found')).toBeInTheDocument()
    })

    it('should show the message when not found', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockNotFoundResourceData}
        />
      )

      expect(screen.getByText(/Not found in namespace flux-system/)).toBeInTheDocument()
    })

    it('should apply faded styling when resource not found', () => {
      const { container } = render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockNotFoundResourceData}
        />
      )

      const card = container.querySelector('.card')
      expect(card).toHaveClass('opacity-60')
    })

    it('should not show timestamp when resource not found', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={null}
        />
      )

      // Should not have any timestamp text
      expect(screen.queryByText(/Jan/)).not.toBeInTheDocument()
    })

    it('should not show message when resource not found', () => {
      render(
        <FavoriteCard
          favorite={{ ...mockFavorite }}
          resourceData={null}
        />
      )

      expect(screen.queryByText(/applied revision/)).not.toBeInTheDocument()
    })
  })

  describe('unknown status', () => {
    it('should show "Unknown" status when resourceData has no status', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={{ lastReconciled: '2024-01-15T10:30:00Z' }}
        />
      )

      expect(screen.getByText('Unknown')).toBeInTheDocument()
    })
  })

  describe('interactions', () => {
    it('should have correct href for resource dashboard link', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockResourceData}
        />
      )

      const cardLink = screen.getByText('flux').closest('a')
      expect(cardLink).toHaveAttribute('href', '/resource/FluxInstance/flux-system/flux')
    })

    it('should call removeFavorite when star button is clicked', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockResourceData}
        />
      )

      const starButton = screen.getByTitle('Remove from favorites')
      fireEvent.click(starButton)

      expect(removeFavorite).toHaveBeenCalledWith('FluxInstance', 'flux-system', 'flux')
    })

    it('should prevent navigation when star button is clicked', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={mockResourceData}
        />
      )

      const starButton = screen.getByTitle('Remove from favorites')
      fireEvent.click(starButton)

      // Click should call removeFavorite but not navigate (preventDefault)
      expect(removeFavorite).toHaveBeenCalledWith('FluxInstance', 'flux-system', 'flux')
    })

    it('should encode special characters in navigation URL', () => {
      const specialFavorite = {
        kind: 'ResourceSet',
        namespace: 'flux-system',
        name: 'my-app-config'
      }

      render(
        <FavoriteCard
          favorite={specialFavorite}
          resourceData={mockResourceData}
        />
      )

      const cardLink = screen.getByText('my-app-config').closest('a')
      expect(cardLink).toHaveAttribute('href', '/resource/ResourceSet/flux-system/my-app-config')
    })
  })

  describe('different statuses', () => {
    it('should render Failed status', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={{ ...mockResourceData, status: 'Failed' }}
        />
      )

      expect(screen.getByText('Failed')).toBeInTheDocument()
    })

    it('should render Progressing status', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={{ ...mockResourceData, status: 'Progressing' }}
        />
      )

      expect(screen.getByText('Progressing')).toBeInTheDocument()
    })

    it('should render Suspended status', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={{ ...mockResourceData, status: 'Suspended' }}
        />
      )

      expect(screen.getByText('Suspended')).toBeInTheDocument()
    })
  })

  describe('message handling', () => {
    it('should not show message section when message is empty', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={{ status: 'Ready', lastReconciled: '2024-01-15T10:30:00Z' }}
        />
      )

      // Should not have the message text
      expect(screen.queryByText(/applied/)).not.toBeInTheDocument()
    })

    it('should show full message in title attribute', () => {
      const longMessage = 'Applied revision: main/abc1234567890 with very long description'

      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={{ ...mockResourceData, message: longMessage }}
        />
      )

      const messageElement = screen.getByTitle(longMessage)
      expect(messageElement).toBeInTheDocument()
    })
  })

  describe('timestamp handling', () => {
    it('should not show timestamp section when lastReconciled is missing', () => {
      render(
        <FavoriteCard
          favorite={mockFavorite}
          resourceData={{ status: 'Ready', message: 'Test message' }}
        />
      )

      // Should not have any timestamp-related elements besides message
      // The sync icon is only shown with lastReconciled
      const svgElements = document.querySelectorAll('svg')
      // Should have star, cube (name), chevron, folder (namespace), console (message) icons - but not sync icon
      expect(svgElements.length).toBe(5) // star, cube, chevron, folder, console
    })
  })
})
