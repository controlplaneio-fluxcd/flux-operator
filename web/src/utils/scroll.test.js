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

  it('should provide a sentinelRef callback for the IntersectionObserver', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    // sentinelRef is a callback ref (a function), so the observer re-binds when
    // the sentinel node mounts/remounts rather than holding a stale node.
    expect(typeof result.current.sentinelRef).toBe('function')
  })

  it('should load more items when sentinel intersects viewport', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    expect(result.current.visibleCount).toBe(100)

    // Attach the sentinel via the callback ref; the effect then observes it.
    const mockSentinel = document.createElement('div')
    act(() => {
      result.current.sentinelRef(mockSentinel)
    })

    expect(observeMock).toHaveBeenCalledWith(mockSentinel)

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

  it('re-binds the observer when the sentinel node is replaced after a refetch', () => {
    // Regression: navigating away and back unmounts the list (and its sentinel)
    // while a refetch is in flight, then remounts a NEW sentinel node. The
    // observer must re-bind to the live node, otherwise paging is stuck at one
    // page. An object ref would leave the observer watching the detached node.
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    const first = document.createElement('div')
    act(() => {
      result.current.sentinelRef(first)
    })
    expect(observeMock).toHaveBeenCalledWith(first)

    // Sentinel unmounts (list hidden during refetch), then a new node remounts.
    act(() => {
      result.current.sentinelRef(null)
    })
    expect(disconnectMock).toHaveBeenCalled()

    const second = document.createElement('div')
    act(() => {
      result.current.sentinelRef(second)
    })
    expect(observeMock).toHaveBeenCalledWith(second)

    // Intersecting the fresh node still pages.
    act(() => {
      intersectionObserverCallback([
        {
          isIntersecting: true,
          target: second
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

    // Attach a sentinel: the effect must still skip observing because the
    // !hasMore guard short-circuits before creating the observer.
    const mockSentinel = document.createElement('div')
    act(() => {
      result.current.sentinelRef(mockSentinel)
    })
    expect(observeMock).not.toHaveBeenCalled()
  })

  it('should disconnect observer when hasMore becomes false', () => {
    const { result } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 150,
        pageSize: 100
      })
    )

    // Attach a sentinel via the callback ref so an observer is created.
    const mockSentinel = document.createElement('div')
    act(() => {
      result.current.sentinelRef(mockSentinel)
    })

    expect(result.current.hasMore).toBe(true)

    // Load remaining items; hasMore flips false and the observer is torn down.
    act(() => {
      result.current.loadMore()
    })

    expect(result.current.visibleCount).toBe(150)
    expect(result.current.hasMore).toBe(false)
    expect(disconnectMock).toHaveBeenCalled()
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
    const { result, unmount } = renderHook(() =>
      useInfiniteScroll({
        totalItems: 500,
        pageSize: 100
      })
    )

    // Attach a sentinel so an observer is actually created.
    const mockSentinel = document.createElement('div')
    act(() => {
      result.current.sentinelRef(mockSentinel)
    })
    expect(observeMock).toHaveBeenCalled()

    unmount()

    expect(disconnectMock).toHaveBeenCalled()
  })
})
