// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'
import { formatCores, formatBytes, percentOf, percentText } from '../../../utils/metrics'

/**
 * ResourceMetric - CPU/Memory usage display: absolute value first, with
 * the request/limit percentages as secondary text and a progress bar only
 * when a real limit is set. Percentages are omitted entirely when the
 * container has no requests/limits configured.
 */
function ResourceMetric({ label, value, percentLabel, barPercent }) {
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
 * ComponentRow - Table row displaying a Flux controller component
 *
 * @param {Object} props
 * @param {Object} props.component - Component object with name, image, ready status
 * @param {Array} props.metrics - Array of pod metrics for resource usage
 * @param {Boolean} props.isRowExpanded - Whether the row is expanded
 * @param {Function} props.toggleComponent - Function to toggle the row's expanded state
 */
function ComponentRow({component, metrics, isRowExpanded, toggleComponent}) {
  const componentMetrics = findComponentMetrics(component.name, metrics)

  // Extract image name and version from full image string
  const getImageInfo = (imageStr) => {
    if (!imageStr) return { name: '', version: 'unknown' };
    const parts = imageStr.split(':')
    const name = parts[0].split('/').pop()
    const version = parts[1]?.split('@')[0] || 'latest'
    return {name, version}
  }

  const imageInfo = getImageInfo(component.image)

  return (
    <>
      <tr class="hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors border-b border-gray-200 dark:border-gray-700">
        <td class="px-6 py-4">
          <button
            onClick={() => toggleComponent(component.name)}
            class="flex items-center space-x-2 text-left w-full group"
          >
            <svg
              class={`w-4 h-4 text-gray-400 dark:text-gray-500 transition-transform ${isRowExpanded ? 'rotate-90' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
            </svg>
            <span
              class="font-medium text-sm sm:text-base text-gray-900 dark:text-gray-100 group-hover:text-flux-blue dark:group-hover:text-blue-400">{component.name}</span>
          </button>
        </td>
        <td class="px-6 py-4">
          <span class="text-xs sm:text-sm text-gray-700 dark:text-gray-300">{imageInfo.version}</span>
        </td>
        <td class="px-6 py-4">
          {component.ready ? (
            <span class="status-badge status-ready text-xs sm:text-sm">Ready</span>
          ) : (
            <span class="status-badge status-not-ready text-xs sm:text-sm">Failing</span>
          )}
        </td>
      </tr>
      {isRowExpanded && (
        <tr class="bg-gray-50 dark:bg-gray-700/50">
          <td colspan="3" class="px-6 py-4">
            <div class="flex flex-col md:flex-row md:gap-8">
              {/* Left: Image, Digest, Status */}
              <div class="flex-1 space-y-3">
                {(() => {
                  const [imagePart, digestPart] = (component.image || '').split('@')
                  return (
                    <>
                      <div class="text-sm">
                        <span class="text-gray-500 dark:text-gray-400">Image</span>
                        <p class="mt-1 text-gray-900 dark:text-white break-all text-xs">{imagePart}</p>
                      </div>
                      {digestPart && (
                        <div class="text-sm">
                          <span class="text-gray-500 dark:text-gray-400">Digest</span>
                          <p class="mt-1 text-gray-900 dark:text-white break-all text-xs">{digestPart}</p>
                        </div>
                      )}
                    </>
                  )
                })()}
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Status</span>
                  <p class="mt-1 text-gray-900 dark:text-white break-all text-xs">{component.status}</p>
                </div>
              </div>

              {/* Right: CPU/Memory metrics with progress bars (hidden when no metrics) */}
              {componentMetrics && (
                <div class="flex-1 space-y-3 mt-4 md:mt-0 pt-4 md:pt-0 border-t md:border-t-0 md:border-l border-gray-200 dark:border-gray-600 md:pl-8">
                  <ResourceMetric
                    label="CPU"
                    value={formatCores(componentMetrics.cpu)}
                    percentLabel={percentText(componentMetrics.cpu, componentMetrics.cpuRequests, componentMetrics.cpuLimits)}
                    barPercent={percentOf(componentMetrics.cpu, componentMetrics.cpuLimits)}
                  />
                  <ResourceMetric
                    label="Memory"
                    value={formatBytes(componentMetrics.memory)}
                    percentLabel={percentText(componentMetrics.memory, componentMetrics.memoryRequests, componentMetrics.memoryLimits)}
                    barPercent={percentOf(componentMetrics.memory, componentMetrics.memoryLimits)}
                  />
                </div>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  )
}

/**
 * ControllersPanel component - Displays list of Flux controller components and their status
 *
 * @param {Object} props
 * @param {Array} props.components - Array of controller component objects
 * @param {Array} props.metrics - Array of controller metrics objects (optional)
 *
 * Features:
 * - Displays component name and readiness status badge
 * - Shows resource usage (CPU/Memory) if metrics are available
 * - Shows status message if not ready
 * - Sorts components alphabetically by name
 */
export function ControllersPanel({ components, metrics }) {
  const expandedComponentRows = useSignal(new Set())
  const isExpanded = useSignal(true)

  if (!components) {
    return null
  }

  function toggleComponent(name) {
    const newSet = new Set(expandedComponentRows.value)
    if (newSet.has(name)) {
      newSet.delete(name)
    } else {
      newSet.add(name)
    }
    expandedComponentRows.value = newSet
  }

  const totalFailing = components.filter(c => !c.ready).length

  return (
    <div class="card p-0">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class="w-full px-6 py-4 border-b border-gray-200 dark:border-gray-700 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white">Flux Components</h3>
            <div class="flex items-center space-x-4 mt-1">
              <p class="text-sm text-gray-600 dark:text-gray-400">{components.length} controllers deployed</p>
              {totalFailing > 0 && (
                <span class="status-badge status-not-ready text-xs sm:text-sm">
                  {totalFailing} failing
                </span>
              )}
            </div>
          </div>
          <svg
            class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isExpanded.value ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
        </div>
      </button>
      {isExpanded.value && components.length > 0 && (
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead class="bg-gray-50 dark:bg-gray-700/50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                Component
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                Version
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                                Status
                </th>
              </tr>
            </thead>
            <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
              {[...components].sort((a, b) => a.name.localeCompare(b.name)).map(component => (
                <ComponentRow
                  key={component.name}
                  component={component}
                  metrics={metrics}
                  isRowExpanded={expandedComponentRows.value.has(component.name)}
                  toggleComponent={toggleComponent}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// Find metrics for a component by matching the pod name pattern,
// summing across all its pods so scaled controllers report full usage.
function findComponentMetrics(componentName, metrics) {
  if (!metrics || metrics.length === 0) return null

  const podMetrics = metrics.filter(m => m.pod && m.pod.startsWith(`${componentName}-`))
  if (podMetrics.length === 0) return null

  return podMetrics.reduce((acc, m) => ({
    cpu: acc.cpu + (m.cpu || 0),
    cpuRequests: acc.cpuRequests + (m.cpuRequests || 0),
    cpuLimits: acc.cpuLimits + (m.cpuLimits || 0),
    memory: acc.memory + (m.memory || 0),
    memoryRequests: acc.memoryRequests + (m.memoryRequests || 0),
    memoryLimits: acc.memoryLimits + (m.memoryLimits || 0)
  }), { cpu: 0, cpuRequests: 0, cpuLimits: 0, memory: 0, memoryRequests: 0, memoryLimits: 0 })
}
