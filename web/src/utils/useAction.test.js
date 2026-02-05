// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/preact'
import { useAction } from './useAction'

// Mock the fetchWithMock function
vi.mock('../utils/fetch', () => ({
  fetchWithMock: vi.fn()
}))

import { fetchWithMock } from '../utils/fetch'

describe('useAction hook', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    fetchWithMock.mockResolvedValue({ success: true, message: 'Action completed' })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('Initial state', () => {
    it('should initialize with null loading state', () => {
      const { result } = renderHook(() => useAction())
      expect(result.current.loading).toBeNull()
    })

    it('should initialize with null error state', () => {
      const { result } = renderHook(() => useAction())
      expect(result.current.error).toBeNull()
    })

    it('should initialize with null showSuccess state', () => {
      const { result } = renderHook(() => useAction())
      expect(result.current.showSuccess).toBeNull()
    })
  })

  describe('performAction', () => {
    it('should set loading state during action', async () => {
      const { result } = renderHook(() => useAction())

      // Start the action
      let actionPromise
      act(() => {
        actionPromise = result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test', kind: 'Test', namespace: 'default', name: 'test' },
          loadingId: 'test-action'
        })
      })

      // Check loading state is set immediately
      expect(result.current.loading).toBe('test-action')

      // Wait for action to complete
      await act(async () => {
        await actionPromise
      })

      expect(result.current.loading).toBeNull()
    })

    it('should call fetchWithMock with correct parameters', async () => {
      const { result } = renderHook(() => useAction())

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/test',
          body: { action: 'restart', kind: 'Deployment', namespace: 'default', name: 'my-app' },
          mockPath: '../mock/test',
          mockExport: 'mockTest'
        })
      })

      expect(fetchWithMock).toHaveBeenCalledWith({
        endpoint: '/api/v1/test',
        mockPath: '../mock/test',
        mockExport: 'mockTest',
        method: 'POST',
        body: { action: 'restart', kind: 'Deployment', namespace: 'default', name: 'my-app' }
      })
    })

    it('should call onActionStart callback when provided', async () => {
      const onActionStart = vi.fn()
      const { result } = renderHook(() => useAction({ onActionStart }))

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test' }
        })
      })

      expect(onActionStart).toHaveBeenCalled()
    })

    it('should call onActionComplete callback after successful action', async () => {
      const onActionComplete = vi.fn()
      const { result } = renderHook(() => useAction({ onActionComplete }))

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test' }
        })
      })

      expect(onActionComplete).toHaveBeenCalled()
    })

    it('should set showSuccess when showSuccessCheck is true', async () => {
      const { result } = renderHook(() => useAction())

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test' },
          loadingId: 'test-action',
          showSuccessCheck: true
        })
      })

      expect(result.current.showSuccess).toBe('test-action')

      // Success should clear after 2 seconds
      act(() => {
        vi.advanceTimersByTime(2000)
      })

      expect(result.current.showSuccess).toBeNull()
    })

    it('should not set showSuccess when showSuccessCheck is false', async () => {
      const { result } = renderHook(() => useAction())

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test' },
          loadingId: 'test-action',
          showSuccessCheck: false
        })
      })

      expect(result.current.showSuccess).toBeNull()
    })

    it('should use action from body as loadingId when loadingId is not provided', async () => {
      const { result } = renderHook(() => useAction())

      let actionPromise
      act(() => {
        actionPromise = result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'restart' }
        })
      })

      expect(result.current.loading).toBe('restart')

      await act(async () => {
        await actionPromise
      })
    })
  })

  describe('Error handling', () => {
    it('should set error state when action fails', async () => {
      fetchWithMock.mockRejectedValue(new Error('Permission denied'))

      const { result } = renderHook(() => useAction())

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test' }
        })
      })

      expect(result.current.error).toBe('Permission denied')
    })

    it('should auto-dismiss error after 5 seconds', async () => {
      fetchWithMock.mockRejectedValue(new Error('Test error'))

      const { result } = renderHook(() => useAction())

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test' }
        })
      })

      expect(result.current.error).toBe('Test error')

      act(() => {
        vi.advanceTimersByTime(5000)
      })

      expect(result.current.error).toBeNull()
    })

    it('should clear loading state even when action fails', async () => {
      fetchWithMock.mockRejectedValue(new Error('Test error'))

      const { result } = renderHook(() => useAction())

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test' },
          loadingId: 'test-action'
        })
      })

      expect(result.current.loading).toBeNull()
    })
  })

  describe('clearError', () => {
    it('should clear error state when called', async () => {
      fetchWithMock.mockRejectedValue(new Error('Test error'))

      const { result } = renderHook(() => useAction())

      await act(async () => {
        await result.current.performAction({
          endpoint: '/api/v1/resource/action',
          body: { action: 'test' }
        })
      })

      expect(result.current.error).toBe('Test error')

      act(() => {
        result.current.clearError()
      })

      expect(result.current.error).toBeNull()
    })
  })
})
