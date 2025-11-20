// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useMemo } from 'preact/hooks'

/**
 * Calculate stats for a bucket based on mode
 */
function calculateBucketStats(bucket, mode, timestampField) {
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
}

/**
 * Groups items into time buckets for timeline visualization
 * Timeline always goes backward in time, ending at the most recent item.
 * For 5 or fewer items with the same timestamp, returns one bucket.
 * For 5 or fewer items with different timestamps, returns one bucket per unique timestamp.
 * For more than 5 items, uses standard bucketing (10 on mobile, 20 on desktop).
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

  // If all items are at the same time, return a single bucket
  if (maxTime === minTime) {
    const bucket = {
      startTime: minTime,
      endTime: minTime,
      items: [...items]
    }
    return [calculateBucketStats(bucket, mode, timestampField)]
  }

  // For 5 or fewer items, create one bucket per unique timestamp
  const uniqueTimestamps = [...new Set(timestamps)].sort((a, b) => a - b)
  if (uniqueTimestamps.length <= 5) {
    return uniqueTimestamps.map(timestamp => {
      const bucketItems = items.filter(item => {
        return new Date(item[timestampField]).getTime() === timestamp
      })
      const bucket = {
        startTime: timestamp,
        endTime: timestamp,
        items: bucketItems
      }
      return calculateBucketStats(bucket, mode, timestampField)
    })
  }

  // Create buckets going backward in time (oldest on left, newest on right)
  const timeRange = maxTime - minTime
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
  return buckets.map(bucket => calculateBucketStats(bucket, mode, timestampField))
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
    // Mixed: orange
    return 'bg-orange-500 dark:bg-orange-600'
  }
  // No events: transparent/gray
  return 'bg-gray-200 dark:bg-gray-700'
}

/**
 * Determines bar color based on resource statuses in bucket
 * Color schema:
 * - All Ready: Green
 * - All Failed: Red
 * - All Progressing: Blue
 * - All Suspended: Yellow
 * - All Unknown: Red
 * - Mixed with any Failed or Unknown: Orange
 * - Mixed without Failed/Unknown: Green
 * @param {Object} bucket - Bucket with resource status counts
 * @returns {string} Tailwind color classes
 */
function getResourceBarColor(bucket) {
  const totalCount = bucket.count
  const { ready = 0, failed = 0, progressing = 0, suspended = 0, unknown = 0 } = bucket

  if (totalCount === 0) {
    // No resources: gray
    return 'bg-gray-200 dark:bg-gray-700'
  }

  // Check if all resources share the same status
  if (ready === totalCount) {
    return 'bg-green-500 dark:bg-green-600'
  }
  if (failed === totalCount) {
    return 'bg-red-500 dark:bg-red-600'
  }
  if (progressing === totalCount) {
    return 'bg-blue-500 dark:bg-blue-600'
  }
  if (suspended === totalCount) {
    return 'bg-yellow-500 dark:bg-yellow-600'
  }
  if (unknown === totalCount) {
    return 'bg-red-500 dark:bg-red-600'
  }

  // Mixed statuses
  if (failed > 0 || unknown > 0) {
    // Mixed with any Failed or Unknown: Orange
    return 'bg-orange-500 dark:bg-orange-600'
  }

  // Mixed without Failed/Unknown: Green
  return 'bg-green-500 dark:bg-green-600'
}

/**
 * Formats time as HH:MM:SS
 * @param {Date} date - Date object
 * @returns {string} Formatted time string e.g., "14:00:01"
 */
function formatTime(date) {
  return date.toLocaleTimeString('en-GB', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  })
}

/**
 * Formats a time range for display in tooltip
 * @param {number} startTime - Start timestamp in milliseconds
 * @param {number} endTime - End timestamp in milliseconds
 * @returns {Object} Object with dateRange and timeRange strings
 */
function formatTimeRange(startTime, endTime) {
  const start = new Date(startTime)
  const end = new Date(endTime)

  const startDate = start.toLocaleDateString()
  const endDate = end.toLocaleDateString()

  const startTimeStr = formatTime(start)
  const endTimeStr = formatTime(end)

  // If same day, show single date; otherwise show range
  const dateRange = startDate === endDate ? startDate : `${startDate} - ${endDate}`
  const timeRange = `${startTimeStr} - ${endTimeStr}`

  return { dateRange, timeRange }
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
 * - Groups items into time buckets (10 on desktop, 7 on mobile)
 * - Color codes bars based on status/type
 * - Shows placeholder bars during loading
 * - Tooltip on hover with breakdown
 */
export function TimelineChart({ items, loading, mode = 'events' }) {
  const [hoveredBar, setHoveredBar] = useState(null)
  const [isDesktop, setIsDesktop] = useState(typeof window !== 'undefined' && window.innerWidth >= 1024)
  const [animationComplete, setAnimationComplete] = useState(false)

  // Detect desktop vs mobile screen size
  useEffect(() => {
    const handleResize = () => {
      setIsDesktop(window.innerWidth >= 1024)
    }
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  // Create a stable key from items to detect actual data changes
  const itemsKey = items && items.length > 0
    ? `${items.length}-${items[0]?.lastTimestamp || items[0]?.lastReconciled}-${items[items.length - 1]?.lastTimestamp || items[items.length - 1]?.lastReconciled}`
    : 'empty'

  // Reset and run animation when data actually changes
  useEffect(() => {
    if (!loading && items && items.length > 0) {
      // Reset animation
      setAnimationComplete(false)

      // Mark as complete after animation duration
      const timer = setTimeout(() => {
        setAnimationComplete(true)
      }, 800) // Match animation duration
      return () => window.clearTimeout(timer)
    }
  }, [loading, itemsKey])

  const bucketCount = isDesktop ? 10 : 7

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

  // Memoize bucket calculation to avoid re-computing on every render (expensive with 1K+ items)
  const buckets = useMemo(() => {
    if (loading || !items || items.length === 0) {
      return placeholderBuckets
    }
    return groupItemsIntoBuckets(items, bucketCount, mode)
  }, [loading, items, bucketCount, mode, itemsKey])

  return (
    <div class="card p-4">
      <style>{`
        @keyframes fillRight {
          from {
            clip-path: inset(0 100% 0 0);
          }
          to {
            clip-path: inset(0 0 0 0);
          }
        }
        @keyframes shimmer {
          0% {
            transform: translateX(-100%);
          }
          100% {
            transform: translateX(100%);
          }
        }
        .loading-shimmer {
          position: relative;
          overflow: hidden;
        }
        .loading-shimmer::after {
          content: '';
          position: absolute;
          top: 0;
          left: 0;
          width: 100%;
          height: 100%;
          background: linear-gradient(
            90deg,
            transparent 0%,
            rgba(255, 255, 255, 0.3) 50%,
            transparent 100%
          );
          animation: shimmer 2s ease-in-out infinite;
        }
        .dark .loading-shimmer::after {
          background: linear-gradient(
            90deg,
            transparent 0%,
            rgba(255, 255, 255, 0.1) 50%,
            transparent 100%
          );
        }
      `}</style>

      {/* Header with total count and stats */}
      <div class="flex items-center justify-between mb-2">
        {/* Left: Total count */}
        <div class="text-sm text-gray-600 dark:text-gray-400">
          {loading ? (
            <span>Loading...</span>
          ) : (
            <span>
              {mode === 'events' ? 'Reconcile events: ' : 'Reconcilers: '}
              <span class="font-semibold text-gray-900 dark:text-gray-100">
                {items && items.length > 0 ? items.length : 0}
              </span>
            </span>
          )}
        </div>

        {/* Right: Status/Severity stats (hidden on mobile, only shown when multiple statuses) */}
        <div class="hidden md:block text-sm text-gray-600 dark:text-gray-400">
          {!loading && items && items.length > 0 && (
            <span class="space-x-3">
              {mode === 'events' ? (
                <>
                  {(() => {
                    const infoCount = items.filter(e => e.type === 'Normal').length
                    const warningCount = items.filter(e => e.type === 'Warning').length
                    // Only show stats if there are multiple different types
                    const hasMultipleTypes = infoCount > 0 && warningCount > 0
                    if (!hasMultipleTypes) return null
                    return (
                      <>
                        {infoCount > 0 && <span>Info: <span class="text-green-600 dark:text-green-400">{infoCount}</span></span>}
                        {warningCount > 0 && <span>Warning: <span class="text-red-600 dark:text-red-400">{warningCount}</span></span>}
                      </>
                    )
                  })()}
                </>
              ) : (
                <>
                  {(() => {
                    const readyCount = items.filter(r => r.status === 'Ready').length
                    const progressingCount = items.filter(r => r.status === 'Progressing').length
                    const suspendedCount = items.filter(r => r.status === 'Suspended').length
                    const unknownCount = items.filter(r => r.status === 'Unknown').length
                    const failedCount = items.filter(r => r.status === 'Failed').length
                    // Only show stats if there are multiple different statuses
                    const statusCounts = [readyCount, progressingCount, suspendedCount, unknownCount, failedCount]
                    const nonZeroStatuses = statusCounts.filter(c => c > 0).length
                    if (nonZeroStatuses <= 1) return null
                    return (
                      <>
                        {readyCount > 0 && <span>Ready: <span class="text-green-600 dark:text-green-400">{readyCount}</span></span>}
                        {progressingCount > 0 && <span>Progressing: <span class="text-blue-600 dark:text-blue-400">{progressingCount}</span></span>}
                        {suspendedCount > 0 && <span>Suspended: <span class="text-yellow-600 dark:text-yellow-400">{suspendedCount}</span></span>}
                        {unknownCount > 0 && <span>Unknown: <span class="text-red-600 dark:text-red-400">{unknownCount}</span></span>}
                        {failedCount > 0 && <span>Failed: <span class="text-red-600 dark:text-red-400">{failedCount}</span></span>}
                      </>
                    )
                  })()}
                </>
              )}
            </span>
          )}
        </div>
      </div>

      {/* Horizontal bar chart */}
      <div class="relative flex gap-0" style={{ height: '32px' }}>
        {loading ? (
          /* Single loading bar with shimmer */
          <div class="w-full h-full bg-gray-200 dark:bg-gray-700 loading-shimmer" />
        ) : (
          buckets.map((bucket, index) => {
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
                {/* Horizontal segment - gray background */}
                <div
                  class={`h-full ${grayClass}`}
                  style={{ width: animationComplete ? 'calc(100% + 1px)' : '100%' }}
                >
                  {/* Colored fill overlay - animates from left to right on initial load */}
                  {hasItems && (
                    <div
                      class={`h-full transition-opacity duration-200 ${colorClass} ${
                        bucket.count > 0 ? 'hover:opacity-80 cursor-pointer' : ''
                      }`}
                      style={{
                        width: '100%',
                        animation: !animationComplete ? 'fillRight 0.8s ease-out both' : 'none',
                        clipPath: animationComplete ? 'none' : undefined
                      }}
                    />
                  )}
                </div>

                {/* Tooltip */}
                {hoveredBar === index && !loading && items && items.length > 0 && (
                  <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 z-10 pointer-events-none">
                    <div class="bg-gray-900 dark:bg-gray-800 text-white text-xs rounded-lg py-2 px-3 shadow-lg whitespace-nowrap">
                      {(() => {
                        const { dateRange, timeRange } = formatTimeRange(bucket.startTime, bucket.endTime)
                        return (
                          <div class="font-semibold mb-1">
                            <div>{dateRange}</div>
                            <div>{timeRange}</div>
                          </div>
                        )
                      })()}
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
                                <div class="text-blue-400">Progressing: {bucket.progressing}</div>
                              )}
                              {bucket.suspended > 0 && (
                                <div class="text-yellow-400">Suspended: {bucket.suspended}</div>
                              )}
                              {bucket.unknown > 0 && (
                                <div class="text-red-400">Unknown: {bucket.unknown}</div>
                              )}
                              {bucket.failed > 0 && (
                                <div class="text-red-400">Failed: {bucket.failed}</div>
                              )}
                            </>
                          )}
                        </div>
                      ) : (
                        <div class="text-gray-400">No {mode === 'events' ? 'events' : 'reconciliation'}</div>
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
          })
        )}
      </div>
    </div>
  )
}
