// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { formatTimestamp } from '../../../utils/time'
import { getWorkloadStatusBadgeClass, formatWorkloadStatus } from '../../../utils/status'
import { FluxIcon } from '../../common/icons'

/**
 * WorkloadsTabContent - Displays detailed Kubernetes workload information
 * Handles data fetching and state management for workload details
 */
export function WorkloadsTabContent({ workloadItems, namespace }) {
  // State
  const [workloadsData, setWorkloadsData] = useState({})
  const [loading, setLoading] = useState(true)
  const [expandedWorkloads, setExpandedWorkloads] = useState({})

  // Fetch workload data when component mounts or workloadItems change
  useEffect(() => {
    const fetchWorkloadsData = async () => {
      // Only show loading spinner on initial load (when no data exists)
      if (Object.keys(workloadsData).length === 0) {
        setLoading(true)
      }

      try {
        // Build workloads array with resolved namespaces
        const workloads = workloadItems.map(item => ({
          kind: item.kind,
          name: item.name,
          namespace: item.namespace || namespace
        }))

        // Send all workloads in a single POST request
        const response = await fetchWithMock({
          endpoint: '/api/v1/workloads',
          mockPath: '../mock/workload',
          mockExport: 'getMockWorkloads',
          method: 'POST',
          body: { workloads }
        })

        // Build workloadsData map from response
        const newWorkloadsData = {}
        const returnedWorkloads = response.workloads || []
        returnedWorkloads.forEach(workload => {
          const key = `${workload.kind}/${workload.namespace}/${workload.name}`
          newWorkloadsData[key] = workload
        })
        setWorkloadsData(newWorkloadsData)
      } catch (err) {
        console.error('Failed to fetch workloads:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchWorkloadsData()
  }, [workloadItems, namespace])

  // Toggle workload expansion
  const toggleWorkloadExpansion = (key) => {
    setExpandedWorkloads(prev => ({
      ...prev,
      [key]: !prev[key]
    }))
  }

  // Show loading state
  if (loading) {
    return (
      <div class="flex items-center justify-center p-8">
        <FluxIcon className="animate-spin h-8 w-8 text-flux-blue" />
        <span class="ml-3 text-gray-600 dark:text-gray-400">Loading workloads...</span>
      </div>
    )
  }

  // Render workload list
  return (
    <div class="space-y-4">
      {workloadItems.map((item) => {
        const key = `${item.kind}/${item.namespace || namespace}/${item.name}`
        const workload = workloadsData[key]
        const isExpanded = expandedWorkloads[key]

        // Get most recent pod timestamp
        const mostRecentTimestamp = workload?.pods?.length > 0
          ? workload.pods.reduce((latest, pod) => {
            if (!pod.timestamp) return latest
            if (!latest) return pod.timestamp
            return new Date(pod.timestamp) > new Date(latest) ? pod.timestamp : latest
          }, null)
          : null

        return (
          <div key={key} class="border border-gray-200 dark:border-gray-700 rounded-md overflow-hidden">
            {/* Workload Header */}
            <button
              onClick={() => toggleWorkloadExpansion(key)}
              class="w-full px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
            >
              <div class="flex items-center justify-between">
                <div class="flex-grow min-w-0 mr-2">
                  {/* Line 1: KIND (uppercase) with Status badge and timestamp */}
                  <div class="flex items-center justify-between mb-1">
                    <div class="flex items-center gap-3">
                      <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">
                        {item.kind}
                      </span>
                      {workload && (
                        <span class={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${getWorkloadStatusBadgeClass(workload.status)}`}>
                          {formatWorkloadStatus(workload.status)}
                        </span>
                      )}
                    </div>
                    {mostRecentTimestamp && (
                      <span class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap ml-4">
                        {formatTimestamp(mostRecentTimestamp)}
                      </span>
                    )}
                  </div>
                  {/* Line 2: Namespace/Name */}
                  <div class="text-sm mt-1">
                    <span class="text-gray-500 dark:text-gray-400">{item.namespace || namespace}/</span>
                    <span class="font-semibold text-gray-900 dark:text-gray-100">{item.name}</span>
                  </div>
                  {/* Line 3: StatusMessage */}
                  {workload && workload.statusMessage && (
                    <div class="text-sm text-gray-700 dark:text-gray-300 mt-1 break-all">
                      {workload.statusMessage}
                    </div>
                  )}
                </div>
                <svg
                  class={`w-4 h-4 text-gray-400 dark:text-gray-500 transition-transform flex-shrink-0 ml-2 ${isExpanded ? 'rotate-180' : ''}`}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                </svg>
              </div>
            </button>

            {/* Workload Details (Expandable) */}
            {isExpanded && workload && (
              <div class="px-4 py-3 bg-gray-50 dark:bg-gray-800/30 border-t border-gray-200 dark:border-gray-700">
                {/* Container Images */}
                {workload.containerImages && workload.containerImages.length > 0 && (
                  <div class="mb-3">
                    <span class="text-xs font-medium text-gray-500 dark:text-gray-400">Images</span>
                    <div class="mt-1 space-y-1">
                      {workload.containerImages.map((image, idx) => (
                        <div key={idx} class="text-xs text-gray-700 dark:text-gray-300 break-all bg-white dark:bg-gray-900 px-2 py-1 rounded">
                          {image}
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Pods */}
                {workload.pods && workload.pods.length > 0 && (
                  <div>
                    <span class="text-xs font-medium text-gray-500 dark:text-gray-400">Pods</span>
                    <div class="mt-2 space-y-2">
                      {workload.pods.map((pod, idx) => (
                        <div key={idx} class="bg-white dark:bg-gray-900 rounded px-3 py-2">
                          <div class="flex items-center justify-between">
                            <span class="text-xs text-gray-900 dark:text-white truncate flex-grow mr-2">
                              {pod.name}
                            </span>
                            <span class={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium flex-shrink-0 ${getWorkloadStatusBadgeClass(pod.status)}`}>
                              {formatWorkloadStatus(pod.status)}
                            </span>
                          </div>
                          {pod.statusMessage && (
                            <p class="text-xs text-gray-600 dark:text-gray-400 mt-1 break-all">
                              {pod.statusMessage}
                            </p>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* No Pods */}
                {(!workload.pods || workload.pods.length === 0) && (
                  <p class="text-xs text-gray-500 dark:text-gray-400">No pods found</p>
                )}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
