// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useRef } from 'preact/hooks'

/**
 * useInfiniteScroll - Hook for implementing infinite scroll functionality
 *
 * @param {Object} options - Configuration options
 * @param {number} options.totalItems - Total number of items in the dataset
 * @param {number} options.pageSize - Number of items to show per page (default: 100)
 * @param {number} options.threshold - Distance in pixels from viewport to trigger load (default: 200)
 * @param {Array} options.deps - Dependencies that should reset the visible count (e.g., filter values)
 *
 * @returns {Object} - Returns object with:
 *   - visibleCount: Number of items currently visible
 *   - sentinelRef: Ref to attach to the sentinel element for intersection observation
 *   - hasMore: Boolean indicating if there are more items to load
 *   - loadMore: Function to manually trigger loading more items
 *
 * Usage:
 * ```jsx
 * const { visibleCount, sentinelRef, hasMore, loadMore } = useInfiniteScroll({
 *   totalItems: data.length,
 *   pageSize: 100,
 *   deps: [filter1, filter2]
 * })
 *
 * // Render only visible items
 * const visibleItems = data.slice(0, visibleCount)
 *
 * // Add sentinel element at the end
 * <div ref={sentinelRef} />
 * ```
 */
export function useInfiniteScroll({
  totalItems = 0,
  pageSize = 100,
  threshold = 200,
  deps = []
}) {
  const [visibleCount, setVisibleCount] = useState(pageSize)
  const sentinelRef = useRef(null)
  const observerRef = useRef(null)

  // Reset visible count when dependencies change (filters, data refetch, etc.)
  useEffect(() => {
    setVisibleCount(pageSize)
  }, deps)

  // Calculate if there are more items to load
  const hasMore = visibleCount < totalItems

  // Function to load more items
  const loadMore = () => {
    if (hasMore) {
      setVisibleCount(prev => Math.min(prev + pageSize, totalItems))
    }
  }

  // Set up IntersectionObserver to detect when sentinel enters viewport
  useEffect(() => {
    // Check if IntersectionObserver is supported
    if (!window.IntersectionObserver) {
      return
    }

    // Don't observe if no more items to load
    if (!hasMore) {
      return
    }

    // Clean up previous observer
    if (observerRef.current) {
      observerRef.current.disconnect()
    }

    // Create new observer
    // eslint-disable-next-line no-undef
    observerRef.current = new IntersectionObserver(
      (entries) => {
        const entry = entries[0]
        if (entry.isIntersecting) {
          loadMore()
        }
      },
      {
        // Trigger when sentinel is within threshold pixels of viewport
        rootMargin: `${threshold}px`
      }
    )

    // Start observing the sentinel element
    if (sentinelRef.current) {
      observerRef.current.observe(sentinelRef.current)
    }

    // Cleanup on unmount
    return () => {
      if (observerRef.current) {
        observerRef.current.disconnect()
      }
    }
  }, [hasMore, visibleCount, totalItems])

  return {
    visibleCount,
    sentinelRef,
    hasMore,
    loadMore
  }
}
