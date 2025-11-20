// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { renderHook, act } from '@testing-library/preact'
import { useInfiniteScroll } from './scroll'

describe('useInfiniteScroll', () => {
  let intersectionObserverCallback
  let observeMock
  let disconnectMock

  beforeEach(() => {
    // Reset mocks
    observeMock = vi.fn()
    disconnectMock = vi.fn()

    // Mock IntersectionObserver as a constructor
    global.IntersectionObserver = class IntersectionObserver {
      constructor(callback) {
        intersectionObserverCallback = callback
        this.observe = observeMock
        this.disconnect = disconnectMock
        this.unobserve = vi.fn()
      }
    }
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('should initialize with pageSize items visible', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    expect(result.current.visibleCount).toBe(100)
    expect(result.current.hasMore).toBe(true)
  })

  it('should calculate hasMore correctly', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 150,
        pageSize: 100
      })
    )

    expect(result.current.hasMore).toBe(true)

    // Load more to reach total
    act(() => {
      result.current.loadMore()
    })

    expect(result.current.visibleCount).toBe(150)
    expect(result.current.hasMore).toBe(false)
  })

  it('should load more items when loadMore is called', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    expect(result.current.visibleCount).toBe(100)

    act(() => {
      result.current.loadMore()
    })

    expect(result.current.visibleCount).toBe(200)

    act(() => {
      result.current.loadMore()
    })

    expect(result.current.visibleCount).toBe(300)
  })

  it('should not exceed totalItems when loading more', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 250,
        pageSize: 100
      })
    )

    expect(result.current.visibleCount).toBe(100)

    act(() => {
      result.current.loadMore()
    })
    expect(result.current.visibleCount).toBe(200)

    act(() => {
      result.current.loadMore()
    })
    // Should cap at 250, not go to 300
    expect(result.current.visibleCount).toBe(250)
  })

  it('should reset visibleCount when deps change', () => {
    let filterValue = 'initial'

    const { result, rerender } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100,
        deps: [filterValue]
      })
    )

    // Load more items
    act(() => {
      result.current.loadMore()
      result.current.loadMore()
    })
    expect(result.current.visibleCount).toBe(300)

    // Change filter - should reset
    filterValue = 'changed'
    rerender()

    expect(result.current.visibleCount).toBe(100)
  })

  it('should provide sentinelRef for IntersectionObserver', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    expect(result.current.sentinelRef).toBeDefined()
    expect(result.current.sentinelRef.current).toBeNull() // Ref is initially null until attached
  })

  it('should load more items when sentinel intersects viewport', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    expect(result.current.visibleCount).toBe(100)

    // Create a mock sentinel element
    const mockSentinel = document.createElement('div')
    result.current.sentinelRef.current = mockSentinel

    // Manually call observe to simulate the effect
    act(() => {
      observeMock(mockSentinel)
    })

    // Simulate intersection
    act(() => {
      intersectionObserverCallback([
        {
          isIntersecting: true,
          target: mockSentinel
        }
      ])
    })

    expect(result.current.visibleCount).toBe(200)
  })

  it('should not observe when there are no more items', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 100,
        pageSize: 100
      })
    )

    // Already showing all items
    expect(result.current.hasMore).toBe(false)
    expect(observeMock).not.toHaveBeenCalled()
  })

  it('should disconnect observer when hasMore becomes false', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 150,
        pageSize: 100
      })
    )

    // Create a mock sentinel element
    const mockSentinel = document.createElement('div')
    result.current.sentinelRef.current = mockSentinel

    expect(result.current.hasMore).toBe(true)

    // Load remaining items
    act(() => {
      result.current.loadMore()
    })

    expect(result.current.visibleCount).toBe(150)
    expect(result.current.hasMore).toBe(false)
  })

  it('should not create IntersectionObserver if not supported', () => {
    // Remove IntersectionObserver support
    const originalIO = global.IntersectionObserver
    global.IntersectionObserver = undefined

    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    // Hook should still work, just without observer
    expect(result.current.visibleCount).toBe(100)
    expect(result.current.hasMore).toBe(true)

    // loadMore should still work manually
    act(() => {
      result.current.loadMore()
    })
    expect(result.current.visibleCount).toBe(200)

    // Restore IntersectionObserver
    global.IntersectionObserver = originalIO
  })

  it('should use custom pageSize', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 1000,
        pageSize: 50
      })
    )

    expect(result.current.visibleCount).toBe(50)
  })

  it('should handle empty or zero totalItems', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 0,
        pageSize: 100
      })
    )

    expect(result.current.visibleCount).toBe(100)
    expect(result.current.hasMore).toBe(false)

    // loadMore should do nothing
    act(() => {
      result.current.loadMore()
    })
    expect(result.current.visibleCount).toBe(100)
  })

  it('should cleanup observer on unmount', () => {
    const { unmount } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    unmount()

    expect(disconnectMock).toHaveBeenCalled()
  })
})
