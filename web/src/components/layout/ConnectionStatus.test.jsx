// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, act } from '@testing-library/preact'
import { ConnectionStatus } from './ConnectionStatus'
import { connectionStatus } from '../../app'

describe('ConnectionStatus', () => {
  beforeEach(() => {
    // Reset signal to default state
    connectionStatus.value = 'connected'
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
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
    it('should not render loading banner before 300ms delay', () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      // Before 300ms, should not show
      expect(container.firstChild).toBeNull()
    })

    it('should render loading banner after 300ms delay', async () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      // Advance timers past the 300ms delay
      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const banner = container.querySelector('div')
      expect(banner).toBeInTheDocument()
    })

    it('should show gray bar in loading state after delay', async () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const bar = container.querySelector('.bg-gray-400')
      expect(bar).toBeInTheDocument()
    })

    it('should show h-1 height in loading state after delay', async () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const bar = container.querySelector('.h-1')
      expect(bar).toBeInTheDocument()
    })

    it('should show animated pulse gradient in loading state after delay', async () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const gradient = container.querySelector('.animate-pulse')
      expect(gradient).toBeInTheDocument()
      expect(gradient).toHaveClass('bg-gradient-to-r')
      expect(gradient).toHaveClass('from-transparent')
      expect(gradient).toHaveClass('via-white')
      expect(gradient).toHaveClass('to-transparent')
    })

    it('should be positioned at top of viewport after delay', async () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const wrapper = container.querySelector('.fixed')
      expect(wrapper).toBeInTheDocument()
      expect(wrapper).toHaveClass('top-0')
      expect(wrapper).toHaveClass('left-0')
      expect(wrapper).toHaveClass('right-0')
      expect(wrapper).toHaveClass('z-50')
    })

    it('should have transition-colors for smooth state changes after delay', async () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const bar = container.querySelector('.transition-colors')
      expect(bar).toBeInTheDocument()
    })

    it('should not show loading bar if fetch completes before 300ms', async () => {
      connectionStatus.value = 'loading'

      const { container, rerender } = render(<ConnectionStatus />)

      // Advance only 200ms (less than 300ms threshold)
      await act(async () => {
        vi.advanceTimersByTime(200)
      })

      // Fetch completes - status changes to connected
      connectionStatus.value = 'connected'
      rerender(<ConnectionStatus />)

      // Should not show any bar
      expect(container.firstChild).toBeNull()
    })
  })

  describe('Disconnected State', () => {
    it('should render disconnected banner immediately (no delay)', () => {
      connectionStatus.value = 'disconnected'

      const { container } = render(<ConnectionStatus />)

      // Should show immediately without waiting for timer
      const banner = container.querySelector('div')
      expect(banner).toBeInTheDocument()
    })

    it('should show red bar in disconnected state immediately', () => {
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

    it('should show "disconnected" label below the bar', () => {
      connectionStatus.value = 'disconnected'

      const { container } = render(<ConnectionStatus />)

      const label = container.querySelector('span')
      expect(label).toBeInTheDocument()
      expect(label.textContent).toBe('disconnected')
      expect(label).toHaveClass('bg-red-500')
      expect(label).toHaveClass('text-white')
      expect(label).toHaveClass('rounded-b')
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
    it('should update when connectionStatus changes from connected to loading after delay', async () => {
      connectionStatus.value = 'connected'

      const { container, rerender } = render(<ConnectionStatus />)
      expect(container.firstChild).toBeNull()

      connectionStatus.value = 'loading'
      rerender(<ConnectionStatus />)

      // Before delay, still null
      expect(container.firstChild).toBeNull()

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const bar = container.querySelector('.bg-gray-400')
      expect(bar).toBeInTheDocument()
    })

    it('should update when connectionStatus changes from loading to disconnected', async () => {
      connectionStatus.value = 'loading'

      const { container, rerender } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

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

    it('should show disconnected immediately even if loading timer is pending', async () => {
      connectionStatus.value = 'loading'

      const { container, rerender } = render(<ConnectionStatus />)

      // Advance only 100ms (timer still pending)
      await act(async () => {
        vi.advanceTimersByTime(100)
      })

      // Disconnected happens before loading delay completes
      connectionStatus.value = 'disconnected'
      rerender(<ConnectionStatus />)

      // Should show red bar immediately
      expect(container.querySelector('.bg-red-500')).toBeInTheDocument()
    })
  })

  describe('Visual Styling', () => {
    it('should have full width', async () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const bar = container.querySelector('.w-full')
      expect(bar).toBeInTheDocument()
    })

    it('should have z-50 to appear above other content', async () => {
      connectionStatus.value = 'loading'

      const { container } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

      const wrapper = container.querySelector('.z-50')
      expect(wrapper).toBeInTheDocument()
    })

    it('should maintain consistent structure between states', async () => {
      connectionStatus.value = 'loading'

      const { container, rerender } = render(<ConnectionStatus />)

      await act(async () => {
        vi.advanceTimersByTime(300)
      })

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

    it('should cleanup timer on unmount', async () => {
      connectionStatus.value = 'loading'

      const { unmount } = render(<ConnectionStatus />)

      // Unmount before timer fires
      unmount()

      // Advance timers - should not cause any errors
      await act(async () => {
        vi.advanceTimersByTime(500)
      })

      // No assertion needed - just verifying no errors occur
    })
  })
})
