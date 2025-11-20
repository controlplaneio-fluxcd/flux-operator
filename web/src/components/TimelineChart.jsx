// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect } from 'preact/hooks'

/**
 * Groups items into time buckets for timeline visualization
 * @param {Array} items - Array of event or resource objects
 * @param {number} bucketCount - Number of time buckets to create
 * @param {string} mode - 'events' or 'resources'
 * @returns {Array} Array of bucket objects with timestamp, count, and status breakdown
 */
function groupItemsIntoBuckets(items, bucketCount = 20, mode = 'events') {
  if (!items || items.length === 0) {
    return []
  }

  // Determine timestamp field based on mode
  const timestampField = mode === 'events' ? 'lastTimestamp' : 'lastReconciled'

  // Find time range
  const timestamps = items.map(e => new Date(e[timestampField]).getTime())
  const minTime = Math.min(...timestamps)
  const maxTime = Math.max(...timestamps)

  // If all items are at the same time, spread them across the timeline
  const timeRange = maxTime - minTime || 3600000 // Use 1 hour if all at same time

  // Create buckets
  const bucketSize = timeRange / bucketCount
  const buckets = Array.from({ length: bucketCount }, (_, i) => ({
    startTime: minTime + (i * bucketSize),
    endTime: minTime + ((i + 1) * bucketSize),
    items: []
  }))

  // Assign items to buckets
  items.forEach(item => {
    const itemTime = new Date(item[timestampField]).getTime()
    const bucketIndex = Math.min(
      Math.max(0, Math.floor((itemTime - minTime) / bucketSize)),
      bucketCount - 1
    )
    buckets[bucketIndex].items.push(item)
  })

  // Calculate stats for each bucket based on mode
  return buckets.map(bucket => {
    let stats
    if (mode === 'events') {
      const warnings = bucket.items.filter(e => e.type === 'Warning').length
      const normals = bucket.items.filter(e => e.type === 'Normal').length
      stats = { warnings, normals }
    } else {
      // resources mode
      const ready = bucket.items.filter(r => r.status === 'Ready').length
      const failed = bucket.items.filter(r => r.status === 'Failed').length
      const progressing = bucket.items.filter(r => r.status === 'Progressing').length
      const suspended = bucket.items.filter(r => r.status === 'Suspended').length
      const unknown = bucket.items.filter(r => r.status === 'Unknown').length
      stats = { ready, failed, progressing, suspended, unknown }
    }

    // Get the most recent item timestamp in this bucket for tooltip display
    const mostRecentItemTime = bucket.items.length > 0
      ? Math.max(...bucket.items.map(e => new Date(e[timestampField]).getTime()))
      : null

    return {
      startTime: bucket.startTime,
      endTime: bucket.endTime,
      mostRecentItemTime,
      count: bucket.items.length,
      ...stats
    }
  })
}

/**
 * Determines bar color based on event types in bucket
 * @param {number} warnings - Count of warning events
 * @param {number} normals - Count of normal events
 * @returns {string} Tailwind color classes
 */
function getEventBarColor(warnings, normals) {
  if (warnings === 0 && normals > 0) {
    // All normal: green
    return 'bg-green-500 dark:bg-green-600'
  } else if (warnings > 0 && normals === 0) {
    // All warnings: red
    return 'bg-red-500 dark:bg-red-600'
  } else if (warnings > 0 && normals > 0) {
    // Mixed: yellow
    return 'bg-yellow-500 dark:bg-yellow-600'
  }
  // No events: transparent/gray
  return 'bg-gray-200 dark:bg-gray-700'
}

/**
 * Determines bar color based on resource statuses in bucket
 * @param {Object} bucket - Bucket with resource status counts
 * @returns {string} Tailwind color classes
 */
function getResourceBarColor(bucket) {
  const totalCount = bucket.count
  const { failed = 0 } = bucket

  if (totalCount === 0) {
    // No resources: gray
    return 'bg-gray-200 dark:bg-gray-700'
  }

  if (failed === totalCount && failed > 0) {
    // All failed: red
    return 'bg-red-500 dark:bg-red-600'
  } else if (failed > 0) {
    // At least one failed (but not all): yellow
    return 'bg-yellow-500 dark:bg-yellow-600'
  } else {
    // Ready/Progressing/Suspended/Unknown: green
    return 'bg-green-500 dark:bg-green-600'
  }
}

/**
 * Formats a timestamp for display in tooltip
 * @param {number} startTime - Timestamp in milliseconds
 * @returns {string} Formatted time string
 */
function formatTimeRange(startTime) {
  const start = new Date(startTime)
  const now = new Date()

  // Calculate minutes ago for start time
  const minutesAgo = Math.floor((now - start) / 60000)

  if (minutesAgo < 60) {
    return `${minutesAgo}m ago`
  } else if (minutesAgo < 1440) {
    const hoursAgo = Math.floor(minutesAgo / 60)
    const remainingMinutes = minutesAgo % 60
    if (remainingMinutes > 0) {
      return `${hoursAgo}h ${remainingMinutes}m ago`
    }
    return `${hoursAgo}h ago`
  } else {
    return start.toLocaleDateString()
  }
}

/**
 * TimelineChart - Timeline visualization of events or resources with color-coded bars
 *
 * @param {Object} props
 * @param {Array} props.items - Array of event or resource objects
 * @param {boolean} props.loading - Whether data is currently loading
 * @param {string} props.mode - 'events' or 'resources'
 *
 * Features:
 * - Groups items into time buckets (40 on desktop, 20 on mobile)
 * - Color codes bars based on status/type
 * - Shows placeholder bars during loading
 * - Tooltip on hover with breakdown
 */
export function TimelineChart({ items, loading, mode = 'events' }) {
  const [hoveredBar, setHoveredBar] = useState(null)
  const [isDesktop, setIsDesktop] = useState(typeof window !== 'undefined' && window.innerWidth >= 1024)

  // Detect desktop vs mobile screen size
  useEffect(() => {
    const handleResize = () => {
      setIsDesktop(window.innerWidth >= 1024)
    }
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  const bucketCount = isDesktop ? 40 : 20

  // Create placeholder buckets for loading state
  const placeholderStats = mode === 'events'
    ? { warnings: 0, normals: 0 }
    : { ready: 0, failed: 0, progressing: 0, suspended: 0, unknown: 0 }

  const placeholderBuckets = Array.from({ length: bucketCount }, (_, i) => ({
    startTime: Date.now() - (bucketCount - i) * 60000,
    endTime: Date.now() - (bucketCount - 1 - i) * 60000,
    mostRecentItemTime: null,
    count: 1, // Set to 1 so bars show during loading
    ...placeholderStats
  }))

  // Use placeholder buckets if loading or if no items
  const buckets = loading || !items || items.length === 0
    ? placeholderBuckets
    : groupItemsIntoBuckets(items, bucketCount, mode)

  return (
    <div class="card p-4">
      <style>{`
        @keyframes fillUp {
          from {
            clip-path: inset(100% 0 0 0);
          }
          to {
            clip-path: inset(0 0 0 0);
          }
        }
      `}</style>
      {/* Bar chart */}
      <div class="relative flex items-end gap-1" style={{ height: '64px' }}>
        {buckets.map((bucket, index) => {
          // All bars are full height
          const heightPx = 64

          const isLoading = loading || !items || items.length === 0
          const hasItems = !isLoading && bucket.count > 0
          const grayClass = 'bg-gray-200 dark:bg-gray-700'

          // Determine color based on mode
          let colorClass
          if (isLoading) {
            colorClass = 'bg-gray-200 dark:bg-gray-700 opacity-50'
          } else if (bucket.count === 0) {
            colorClass = grayClass
          } else if (mode === 'events') {
            colorClass = getEventBarColor(bucket.warnings, bucket.normals)
          } else {
            colorClass = getResourceBarColor(bucket)
          }

          return (
            <div
              key={index}
              class="relative flex-1 group"
              onMouseEnter={() => !loading && setHoveredBar(index)}
              onMouseLeave={() => setHoveredBar(null)}
            >
              {/* Bar - always gray background */}
              <div
                class={`w-full ${grayClass}`}
                style={{ height: `${heightPx}px` }}
              >
                {/* Colored fill overlay - animates from bottom to top */}
                {hasItems && (
                  <div
                    class={`w-full h-full transition-opacity duration-200 ${colorClass} ${
                      bucket.count > 0 ? 'hover:opacity-80 cursor-pointer' : ''
                    }`}
                    style={{
                      animation: `fillUp 0.8s ease-out both`
                    }}
                  />
                )}
              </div>

              {/* Tooltip */}
              {hoveredBar === index && !loading && items && items.length > 0 && (
                <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 z-10 pointer-events-none">
                  <div class="bg-gray-900 dark:bg-gray-800 text-white text-xs rounded-lg py-2 px-3 shadow-lg whitespace-nowrap">
                    <div class="font-semibold mb-1">
                      {bucket.mostRecentItemTime
                        ? formatTimeRange(bucket.mostRecentItemTime)
                        : formatTimeRange(bucket.startTime)}
                    </div>
                    {bucket.count > 0 ? (
                      <div class="space-y-0.5">
                        {mode === 'events' ? (
                          <>
                            {bucket.normals > 0 && (
                              <div class="text-green-400">Info: {bucket.normals}</div>
                            )}
                            {bucket.warnings > 0 && (
                              <div class="text-red-400">Warning: {bucket.warnings}</div>
                            )}
                          </>
                        ) : (
                          <>
                            {bucket.ready > 0 && (
                              <div class="text-green-400">Ready: {bucket.ready}</div>
                            )}
                            {bucket.progressing > 0 && (
                              <div class="text-yellow-400">Progressing: {bucket.progressing}</div>
                            )}
                            {bucket.suspended > 0 && (
                              <div class="text-gray-400">Suspended: {bucket.suspended}</div>
                            )}
                            {bucket.unknown > 0 && (
                              <div class="text-gray-400">Unknown: {bucket.unknown}</div>
                            )}
                            {bucket.failed > 0 && (
                              <div class="text-red-400">Failed: {bucket.failed}</div>
                            )}
                          </>
                        )}
                      </div>
                    ) : (
                      <div class="text-gray-400">No {mode === 'events' ? 'events' : 'resources'}</div>
                    )}
                    {/* Tooltip arrow */}
                    <div class="absolute top-full left-1/2 -translate-x-1/2 -mt-px">
                      <div class="border-4 border-transparent border-t-gray-900 dark:border-t-gray-800"></div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
