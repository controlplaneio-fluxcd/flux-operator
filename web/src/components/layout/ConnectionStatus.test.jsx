// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach } from 'vitest'
import { render } from '@testing-library/preact'
import { ConnectionStatus } from './ConnectionStatus'
import { connectionStatus } from '../../app'

describe('ConnectionStatus', () => {
  beforeEach(() => {
    // Reset signal to default state
    connectionStatus.value = 'connected'
  })

  describe('Connected State', () => {
    it('should not render when connected', () => {
      connectionStatus.value = 'connected'

      const { container } = render(<ConnectionStatus />)

      expect(container.firstChild).toBeNull()
    })

    it('should return null when connected', () => {
      connectionStatus.value = 'connected'

      const { container } = render(<ConnectionStatus />)

      expect(container.innerHTML).toBe('')
    })
  })

  describe('Loading State', () => {
    it('should render loading banner', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      const banner = container.querySelector('div')
      expect(banner).toBeInTheDocument()
    })

    it('should show gray/yellow bar in loading state', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      const bar = container.querySelector('.bg-gray-400')
      expect(bar).toBeInTheDocument()
    })

    it('should show h-1 height in loading state', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      const bar = container.querySelector('.h-1')
      expect(bar).toBeInTheDocument()
    })

    it('should show animated pulse gradient in loading state', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      const gradient = container.querySelector('.animate-pulse')
      expect(gradient).toBeInTheDocument()
      expect(gradient).toHaveClass('bg-gradient-to-r')
      expect(gradient).toHaveClass('from-transparent')
      expect(gradient).toHaveClass('via-white')
      expect(gradient).toHaveClass('to-transparent')
    })

    it('should be positioned at top of viewport', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      const wrapper = container.querySelector('.fixed')
      expect(wrapper).toBeInTheDocument()
      expect(wrapper).toHaveClass('top-0')
      expect(wrapper).toHaveClass('left-0')
      expect(wrapper).toHaveClass('right-0')
      expect(wrapper).toHaveClass('z-50')
    })

    it('should have transition-colors for smooth state changes', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      const bar = container.querySelector('.transition-colors')
      expect(bar).toBeInTheDocument()
    })
  })

  describe('Disconnected State', () => {
    it('should render disconnected banner', () => {
      connectionStatus.value = 'disconnected'

      const { container } = render(<ConnectionStatus />)

      const banner = container.querySelector('div')
      expect(banner).toBeInTheDocument()
    })

    it('should show red bar in disconnected state', () => {
      connectionStatus.value = 'disconnected'

      const { container } = render(<ConnectionStatus />)

      const bar = container.querySelector('.bg-red-500')
      expect(bar).toBeInTheDocument()
    })

    it('should show h-1.5 height in disconnected state (thicker than loading)', () => {
      connectionStatus.value = 'disconnected'

      const { container } = render(<ConnectionStatus />)

      const bar = container.querySelector('.h-1\\.5')
      expect(bar).toBeInTheDocument()
    })

    it('should not show pulse animation in disconnected state', () => {
      connectionStatus.value = 'disconnected'

      const { container } = render(<ConnectionStatus />)

      const gradient = container.querySelector('.animate-pulse')
      expect(gradient).not.toBeInTheDocument()
    })

    it('should be positioned at top of viewport', () => {
      connectionStatus.value = 'disconnected'

      const { container } = render(<ConnectionStatus />)

      const wrapper = container.querySelector('.fixed')
      expect(wrapper).toBeInTheDocument()
      expect(wrapper).toHaveClass('top-0')
      expect(wrapper).toHaveClass('left-0')
      expect(wrapper).toHaveClass('right-0')
    })
  })

  describe('State Transitions', () => {
    it('should update when connectionStatus changes from connected to loading', () => {
      connectionStatus.value = 'connected'

      const { container, rerender } = render(<ConnectionStatus />)
      expect(container.firstChild).toBeNull()

      connectionStatus.value = 'loading'
      rerender(<ConnectionStatus />)

      const bar = container.querySelector('.bg-gray-400')
      expect(bar).toBeInTheDocument()
    })

    it('should update when connectionStatus changes from loading to disconnected', () => {
      connectionStatus.value = 'loading'

      const { container, rerender } = render(<ConnectionStatus />)
      expect(container.querySelector('.bg-gray-400')).toBeInTheDocument()

      connectionStatus.value = 'disconnected'
      rerender(<ConnectionStatus />)

      expect(container.querySelector('.bg-red-500')).toBeInTheDocument()
      expect(container.querySelector('.bg-gray-400')).not.toBeInTheDocument()
    })

    it('should hide when connectionStatus changes from disconnected to connected', () => {
      connectionStatus.value = 'disconnected'

      const { container, rerender } = render(<ConnectionStatus />)
      expect(container.querySelector('.bg-red-500')).toBeInTheDocument()

      connectionStatus.value = 'connected'
      rerender(<ConnectionStatus />)

      expect(container.firstChild).toBeNull()
    })
  })

  describe('Visual Styling', () => {
    it('should have full width', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      const bar = container.querySelector('.w-full')
      expect(bar).toBeInTheDocument()
    })

    it('should have z-50 to appear above other content', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      const wrapper = container.querySelector('.z-50')
      expect(wrapper).toBeInTheDocument()
    })

    it('should maintain consistent structure between states', () => {
      connectionStatus.value = 'loading'

      const { container, rerender } = render(<ConnectionStatus />)
      const loadingStructure = container.querySelector('.fixed > div')
      expect(loadingStructure).toBeInTheDocument()

      connectionStatus.value = 'disconnected'
      rerender(<ConnectionStatus />)

      const disconnectedStructure = container.querySelector('.fixed > div')
      expect(disconnectedStructure).toBeInTheDocument()
    })
  })

  describe('Edge Cases', () => {
    it('should handle undefined connectionStatus gracefully', () => {
      connectionStatus.value = undefined

      const { container } = render(<ConnectionStatus />)

      // When undefined, isDisconnected and isLoading are both false, so should return null
      expect(container.firstChild).toBeNull()
    })

    it('should handle invalid connectionStatus value', () => {
      connectionStatus.value = 'invalid-state'

      const { container } = render(<ConnectionStatus />)

      // Should not crash, should treat as neither loading nor disconnected
      expect(container.firstChild).toBeNull()
    })
  })
})
