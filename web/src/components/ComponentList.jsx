// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'

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
          <span class="font-mono text-xs sm:text-sm text-gray-700 dark:text-gray-300">{imageInfo.version}</span>
        </td>
        <td class="px-6 py-4">
          {component.ready ? (
            <span class="status-badge status-ready">Ready</span>
          ) : (
            <span class="status-badge status-not-ready">Failing</span>
          )}
        </td>
      </tr>
      {isRowExpanded && (
        <tr class="bg-gray-50 dark:bg-gray-700/50">
          <td colspan="3" class="px-6 py-4">
            <div class="space-y-2 text-xs sm:text-sm">
              {componentMetrics ? (
                <>
                  <div>
                    <span class="text-gray-600 dark:text-gray-400">CPU:</span>
                    <span class="ml-2 text-gray-900 dark:text-white">
                      {componentMetrics.cpu.toFixed(3)} / {componentMetrics.cpuLimit.toFixed(1)} cores
                      ({(componentMetrics.cpuLimit > 0 ? (componentMetrics.cpu / componentMetrics.cpuLimit) * 100 : 0).toFixed(0)}%)
                    </span>
                  </div>
                  <div>
                    <span class="text-gray-600 dark:text-gray-400">Memory:</span>
                    <span class="ml-2 text-gray-900 dark:text-white">
                      {formatMemory(componentMetrics.memory)} / {formatMemory(componentMetrics.memoryLimit)} MiB
                      ({(componentMetrics.memoryLimit > 0 ? (componentMetrics.memory / componentMetrics.memoryLimit) * 100 : 0).toFixed(0)}%)
                    </span>
                  </div>
                </>
              ) : <div><span class="text-gray-600 dark:text-gray-400">Metrics:</span><span class="ml-2 text-gray-900 dark:text-white">Not available</span></div>
              }
              <div>
                <span class="text-gray-600 dark:text-gray-400">Image:</span>
                <span class="ml-2 text-gray-900 dark:text-white break-all">{component.image}</span>
              </div>
              <div>
                <span class="text-gray-600 dark:text-gray-400">Status:</span>
                <span class="ml-2 text-gray-900 dark:text-white">{component.status}</span>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  )
}

/**
 * ComponentList component - Displays Flux controller components in a collapsible table
 *
 * @param {Object} props
 * @param {Array} props.components - Array of Flux controller components
 * @param {Array} props.metrics - Array of pod metrics (CPU/memory usage)
 *
 * Features:
 * - Shows component name, version, and readiness status
 * - Expandable rows display detailed metrics (CPU, memory usage and limits)
 * - Links to full image name and digest
 * - Collapsible table with total component count and failing count
 */
export function ComponentList({components, metrics}) {
  const expandedComponentRows = useSignal(new Set())
  const isExpanded = useSignal(true)

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
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Flux Components</h3>
            <div class="flex items-center space-x-4 mt-1">
              <p class="text-sm text-gray-600 dark:text-gray-400">{components.length} controllers deployed</p>
              {totalFailing > 0 && (
                <span class="status-badge status-not-ready">
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
              {components.map(component => (
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

// Find metrics for a component by matching pod name pattern: {component-name}-{hash}-{pod-id}
function findComponentMetrics(componentName, metrics) {
  if (!metrics || metrics.length === 0) return null

  return metrics.find(m => {
    if (!m.pod) return false

    // Pod name format: {component-name}-{replicaset-hash}-{pod-id}
    // Extract component name by removing last two dash-separated segments
    const parts = m.pod.split('-')
    if (parts.length < 3) return false

    // Join all parts except the last two (hash and pod-id)
    const extractedComponentName = parts.slice(0, -2).join('-')

    return extractedComponentName === componentName
  })
}

// Format bytes to MiB
function formatMemory(bytes) {
  if (typeof bytes !== 'number' || bytes < 0) return '0'
  return (bytes / (1024 ** 2)).toFixed(0)
}
