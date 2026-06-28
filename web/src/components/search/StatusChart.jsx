// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useMemo } from 'preact/hooks'
import { getStatusBarColor, getEventBarColor } from '../../utils/status'

/**
 * Calculate status percentages across all items
 * @param {Array} items - Array of event or resource objects
 * @param {string} mode - 'events' or 'resources'
 * @returns {Array} Array of status bar objects sorted by percentage (largest first)
 */
function calculateStatusPercentages(items, mode = 'events') {
  if (!items || items.length === 0) {
    return []
  }

  const totalCount = items.length
  const statusCounts = {}

  // Count each status type
  if (mode === 'events') {
    items.forEach(item => {
      const type = item.type === 'Warning' ? 'Warning' : 'Normal'
      statusCounts[type] = (statusCounts[type] || 0) + 1
    })
  } else {
    // resources mode
    items.forEach(item => {
      const status = item.status || 'Unknown'
      statusCounts[status] = (statusCounts[status] || 0) + 1
    })
  }

  // Convert to array of status bar objects with percentage
  const statusBars = Object.entries(statusCounts).map(([status, count]) => ({
    status,
    count,
    percentage: (count / totalCount) * 100
  }))

  // Sort by percentage descending (largest first)
  return statusBars.sort((a, b) => b.percentage - a.percentage)
}

/**
 * Get bar color based on status and mode using shared utilities
 * @param {string} status - Status name ('Normal', 'Warning', 'Ready', 'Failed', etc.)
 * @param {string} mode - 'events' or 'resources'
 * @returns {string} Tailwind color classes
 */
function getBarColor(status, mode) {
  if (mode === 'events') {
    return getEventBarColor(status)
  }
  return getStatusBarColor(status)
}

/**
 * StatusChart - Status distribution visualization with color-coded bars
 *
 * @param {Object} props
 * @param {Array} props.items - Array of event or resource objects
 * @param {boolean} props.loading - Whether data is currently loading
 * @param {string} props.mode - 'events' or 'resources'
 * @param {boolean} [props.compact] - When true, renders a thin rounded bar with no
 *   card wrapper and no count/stats header (the count lives in the toolbar). The
 *   hover tooltip and click-to-filter behavior are preserved.
 *
 * Features:
 * - Displays one bar per status type (Ready, Failed, Progressing, etc.)
 * - Bar width proportional to percentage
 * - Sorted by percentage (largest to smallest, left to right)
 * - Color codes bars based on status type
 * - Shows placeholder bar during loading
 * - Tooltip on hover with count and percentage
 */
export function StatusChart({ items, loading, mode = 'events', onBarClick, compact = false }) {
  const [hoveredBar, setHoveredBar] = useState(null)
  const [animationComplete, setAnimationComplete] = useState(false)

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

  // Memoize status bar calculation to avoid re-computing on every render (expensive with 1K+ items)
  const statusBars = useMemo(() => {
    if (loading || !items || items.length === 0) {
      return []
    }
    return calculateStatusPercentages(items, mode)
  }, [loading, items, mode, itemsKey])

  return (
    <div class={compact ? '' : 'card p-4'} data-testid="status-chart">
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

      {/* Header with total count and stats (omitted in compact mode, where the
          count is shown by the toolbar instead) */}
      {!compact && (
        <div class="flex items-center justify-between mb-2">
          {/* Left: Total count */}
          <div class="text-sm text-gray-600 dark:text-gray-400">
            {loading ? (
              <span>Loading...</span>
            ) : (
              <span>
                {mode === 'events' ? 'Reconcile events: ' : 'Resources: '}
                <span class="font-semibold text-gray-900 dark:text-gray-100">
                  {items && items.length > 0 ? items.length : 0}
                </span>
              </span>
            )}
          </div>

          {/* Right: Status/Severity stats (hidden on mobile, only shown when multiple statuses) */}
          <div class="hidden md:block text-sm text-gray-600 dark:text-gray-400">
            {!loading && items && items.length > 0 && statusBars.length > 1 && (
              <span class="space-x-3">
                {statusBars.map((bar) => {
                // Get text color class based on status type
                  let textColorClass
                  if (mode === 'events') {
                    textColorClass = bar.status === 'Normal'
                      ? 'text-green-600 dark:text-green-400'
                      : 'text-red-600 dark:text-red-400'
                  } else {
                  // resources mode
                    if (bar.status === 'Ready') {
                      textColorClass = 'text-green-600 dark:text-green-400'
                    } else if (bar.status === 'Failed') {
                      textColorClass = 'text-red-600 dark:text-red-400'
                    } else if (bar.status === 'Progressing') {
                      textColorClass = 'text-blue-600 dark:text-blue-400'
                    } else if (bar.status === 'Suspended') {
                      textColorClass = 'text-yellow-600 dark:text-yellow-400'
                    } else if (bar.status === 'Unknown') {
                      textColorClass = 'text-gray-600 dark:text-gray-400'
                    }
                  }

                  // Map Normal to Info for display
                  const displayName = mode === 'events' && bar.status === 'Normal' ? 'Info' : bar.status

                  return (
                    <span key={bar.status}>
                      {displayName}: <span class={textColorClass}>{bar.count}</span>
                    </span>
                  )
                })}
              </span>
            )}
          </div>
        </div>
      )}

      {/* Horizontal bar chart - status distribution. The bar lives in a
          non-clipped relative wrapper so the hover tooltip can overflow above
          it even in compact mode, where the bar itself uses overflow-hidden to
          get its rounded ends. */}
      <div class="relative">
        <div
          class={`relative flex gap-0 ${compact ? 'h-2.5 rounded-full overflow-hidden' : ''}`}
          style={compact ? undefined : { height: '32px' }}
        >
          {loading ? (
            /* Single loading bar with shimmer */
            <div class="w-full h-full bg-gray-200 dark:bg-gray-700 loading-shimmer" />
          ) : statusBars.length === 0 ? (
            /* No data - gray bar */
            <div class="w-full h-full bg-gray-200 dark:bg-gray-700" />
          ) : (
            statusBars.map((bar, index) => {
              const colorClass = getBarColor(bar.status, mode)
              const grayClass = 'bg-gray-200 dark:bg-gray-700'

              return (
                <div
                  key={bar.status}
                  class="relative group"
                  style={{ flex: `0 0 ${bar.percentage}%` }}
                  onMouseEnter={() => setHoveredBar(index)}
                  onMouseLeave={() => setHoveredBar(null)}
                  onClick={() => onBarClick?.(bar.status)}
                  role={onBarClick ? 'button' : undefined}
                >
                  {/* Gray background */}
                  <div class={`h-full ${grayClass}`}>
                    {/* Colored fill overlay - animates from left to right on initial load */}
                    <div
                      class={`h-full transition-opacity duration-200 ${colorClass}${onBarClick ? ' hover:opacity-80 cursor-pointer' : ''}`}
                      style={{
                        width: '100%',
                        animation: !animationComplete ? 'fillRight 0.8s ease-out both' : 'none',
                        clipPath: animationComplete ? 'none' : undefined
                      }}
                    />
                  </div>
                </div>
              )
            })
          )}
        </div>

        {/* Tooltip overlay - rendered outside the (possibly clipped) bar and
            positioned over the centre of the hovered segment. */}
        {hoveredBar !== null && statusBars[hoveredBar] && (
          <div
            class="absolute bottom-full mb-2 z-10 -translate-x-1/2 pointer-events-none"
            style={{ left: `${statusBars.slice(0, hoveredBar).reduce((sum, b) => sum + b.percentage, 0) + statusBars[hoveredBar].percentage / 2}%` }}
          >
            <div class="bg-gray-900 dark:bg-gray-800 text-white text-xs rounded-lg py-2 px-3 shadow-lg whitespace-nowrap">
              <div class="font-semibold">
                {statusBars[hoveredBar].status}
              </div>
              <div class="text-gray-300 mt-1">
                Count: {statusBars[hoveredBar].count}
              </div>
              <div class="text-gray-300">
                Percentage: {statusBars[hoveredBar].percentage.toFixed(1)}%
              </div>
              {/* Tooltip arrow */}
              <div class="absolute top-full left-1/2 -translate-x-1/2 -mt-px">
                <div class="border-4 border-transparent border-t-gray-900 dark:border-t-gray-800"></div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
