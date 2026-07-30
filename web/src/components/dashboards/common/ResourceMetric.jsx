// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

/**
 * ResourceMetric - CPU/Memory usage display: absolute value first, with
 * the request/limit percentages as secondary text and a progress bar only
 * when a real limit is set.
 */
export function ResourceMetric({ label, value, percentLabel, barPercent }) {
  let colorClass = 'bg-green-500'
  if (barPercent >= 85) {
    colorClass = 'bg-red-500'
  } else if (barPercent >= 70) {
    colorClass = 'bg-yellow-500'
  }

  return (
    <div class="space-y-1">
      <div class="flex flex-col sm:flex-row sm:justify-between sm:items-baseline gap-1">
        <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">{label}</span>
        <span class="text-xs sm:text-sm">
          <span class="text-gray-900 dark:text-white font-medium">{value}</span>
          {percentLabel && (
            <span class="text-gray-500 dark:text-gray-400"> · {percentLabel}</span>
          )}
        </span>
      </div>
      {barPercent != null && (
        <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
          <div
            class={`${colorClass} h-2 rounded-full transition-all`}
            style={`width: ${Math.min(barPercent, 100)}%`}
          />
        </div>
      )}
    </div>
  )
}

/**
 * Sum usage, requests and limits across pod metrics entries.
 * Returns null when the list is empty.
 */
export function sumResourceUsage(metrics) {
  if (!metrics || metrics.length === 0) return null

  return metrics.reduce((acc, m) => ({
    cpu: acc.cpu + (m.cpu || 0),
    cpuRequests: acc.cpuRequests + (m.cpuRequests || 0),
    cpuLimits: acc.cpuLimits + (m.cpuLimits || 0),
    memory: acc.memory + (m.memory || 0),
    memoryRequests: acc.memoryRequests + (m.memoryRequests || 0),
    memoryLimits: acc.memoryLimits + (m.memoryLimits || 0)
  }), { cpu: 0, cpuRequests: 0, cpuLimits: 0, memory: 0, memoryRequests: 0, memoryLimits: 0 })
}
