// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useCallback } from 'preact/hooks'

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
  // The sentinel DOM node, tracked as state (via a callback ref) rather than a
  // ref object: the lists unmount their sentinel while a refetch is in flight
  // and remount a *new* node afterwards. A callback ref makes that swap a state
  // change, so the observer effect below re-binds to the live node. With a plain
  // ref object the effect would not re-run (its other deps are unchanged) and
  // would keep observing the detached node — leaving the list stuck at page one
  // after navigating away and back.
  const [sentinelNode, setSentinelNode] = useState(null)
  const sentinelRef = useCallback((node) => setSentinelNode(node), [])

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

  // Set up IntersectionObserver to detect when the sentinel enters the viewport.
  // Re-binds whenever the sentinel node changes (remount after a refetch) or when
  // more items load, so re-observing a still-visible sentinel keeps paging.
  useEffect(() => {
    if (!window.IntersectionObserver || !hasMore || !sentinelNode) {
      return
    }

    // eslint-disable-next-line no-undef
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          loadMore()
        }
      },
      {
        // Trigger when sentinel is within threshold pixels of viewport
        rootMargin: `${threshold}px`
      }
    )

    observer.observe(sentinelNode)

    return () => observer.disconnect()
  }, [hasMore, visibleCount, totalItems, sentinelNode, threshold])

  return {
    visibleCount,
    sentinelRef,
    hasMore,
    loadMore
  }
}
