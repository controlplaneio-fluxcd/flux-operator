// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useRef, useCallback } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { formatTimestamp } from '../../../utils/time'
import { getWorkloadStatusBadgeClass, formatWorkloadStatus } from '../../../utils/status'
import { formatScheduleMessage } from '../../../utils/cron'
import { FluxOperatorIcon } from '../../layout/Icons'
import { WorkloadActionBar } from './WorkloadActionBar'

// Polling intervals
const NORMAL_POLL_INTERVAL = 10000 // 10 seconds
const FAST_POLL_INTERVAL = 5000 // 5 seconds
const FAST_POLL_DURATION = 60000 // 60 seconds

// Highlight threshold for recently changed pods
const RECENT_POD_THRESHOLD = 30000 // 30 seconds

/**
 * Check if a timestamp is within the recent threshold
 * @param {string} timestamp - ISO timestamp string
 * @returns {boolean} true if the timestamp is within the last 30 seconds
 */
function isRecentTimestamp(timestamp) {
  if (!timestamp) return false
  const podTime = new Date(timestamp).getTime()
  const now = Date.now()
  return (now - podTime) < RECENT_POD_THRESHOLD
}

/**
 * WorkloadsTabContent - Displays detailed Kubernetes workload information
 * Handles data fetching and state management for workload details
 */
export function WorkloadsTabContent({ workloadItems, namespace, userActions = [] }) {
  // State
  const [workloadsData, setWorkloadsData] = useState({})
  const [loading, setLoading] = useState(true)
  const [expandedWorkloads, setExpandedWorkloads] = useState({})
  const [pollInterval, setPollInterval] = useState(NORMAL_POLL_INTERVAL)

  // Refs for polling and initial load tracking
  const fastPollTimeoutRef = useRef(null)
  const hasLoadedRef = useRef(false)

  // Fetch workloads data function
  const fetchWorkloadsData = useCallback(async () => {
    // Only show loading spinner on initial load
    if (!hasLoadedRef.current) {
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
      hasLoadedRef.current = true
    } catch (err) {
      console.error('Failed to fetch workloads:', err)
    } finally {
      setLoading(false)
    }
  }, [workloadItems, namespace])

  // Initial fetch and polling
  useEffect(() => {
    fetchWorkloadsData()

    const intervalId = window.setInterval(fetchWorkloadsData, pollInterval)
    return () => window.clearInterval(intervalId)
  }, [workloadItems, namespace, pollInterval])

  // Cleanup fast poll timeout on unmount
  useEffect(() => {
    return () => {
      if (fastPollTimeoutRef.current) {
        window.clearTimeout(fastPollTimeoutRef.current)
      }
    }
  }, [])

  // Handler for when an action starts - speed up polling
  const handleActionStart = useCallback(() => {
    // Clear any existing fast poll timeout
    if (fastPollTimeoutRef.current) {
      window.clearTimeout(fastPollTimeoutRef.current)
    }

    // Switch to fast polling
    setPollInterval(FAST_POLL_INTERVAL)

    // Set timeout to return to normal polling
    fastPollTimeoutRef.current = window.setTimeout(() => {
      setPollInterval(NORMAL_POLL_INTERVAL)
    }, FAST_POLL_DURATION)
  }, [])

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
        <FluxOperatorIcon className="animate-spin h-8 w-8 text-flux-blue" />
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
            if (!pod.createdAt) return latest
            if (!latest) return pod.createdAt
            return new Date(pod.createdAt) > new Date(latest) ? pod.createdAt : latest
          }, null)
          : null

        // Find the most recent triggered pod (has createdBy set)
        const triggeredPod = workload?.pods?.length > 0
          ? workload.pods.reduce((latest, pod) => {
            if (!pod.createdBy || !pod.createdAt) return latest
            if (!latest) return pod
            return new Date(pod.createdAt) > new Date(latest.createdAt) ? pod : latest
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
                      {formatScheduleMessage(workload.statusMessage)}
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
                {/* Action Bar - Workload actions */}
                {(item.kind === 'Deployment' || item.kind === 'StatefulSet' || item.kind === 'DaemonSet' || item.kind === 'CronJob') && userActions.includes('restart') && (
                  <div class="mb-3 pb-3 border-b border-gray-200 dark:border-gray-700" onClick={(e) => e.stopPropagation()}>
                    <WorkloadActionBar
                      kind={item.kind}
                      namespace={item.namespace || namespace}
                      name={item.name}
                      status={workload.status}
                      restartedAt={workload.restartedAt}
                      lastTriggeredAt={triggeredPod?.createdAt}
                      lastTriggeredPodStatus={triggeredPod?.status}
                      userActions={userActions}
                      onActionStart={handleActionStart}
                      onActionComplete={fetchWorkloadsData}
                    />
                  </div>
                )}

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
                      {[...workload.pods].sort((a, b) => {
                        // Sort by timestamp descending (most recent first)
                        // Pods without timestamps go to the end
                        if (!a.createdAt && !b.createdAt) return 0
                        if (!a.createdAt) return 1
                        if (!b.createdAt) return -1
                        return new Date(b.createdAt) - new Date(a.createdAt)
                      }).map((pod, idx) => {
                        const isRecent = isRecentTimestamp(pod.createdAt)
                        return (
                          <div
                            key={idx}
                            class={`bg-white dark:bg-gray-900 rounded px-3 py-2 ${
                              isRecent
                                ? 'ring-2 ring-blue-400 dark:ring-blue-500 ring-opacity-50'
                                : ''
                            }`}
                            data-testid={isRecent ? 'recent-pod' : undefined}
                          >
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
                            {pod.createdBy && (
                              <p class="text-xs text-gray-500 dark:text-gray-400 mt-1" data-testid="pod-created-by">
                                Triggered by {pod.createdBy}
                              </p>
                            )}
                          </div>
                        )})}
                    </div>
                  </div>
                )}

                {/* No Pods */}
                {(!workload.pods || workload.pods.length === 0) && (
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {item.kind === 'CronJob' ? 'No recent jobs' : 'No pods found'}
                  </p>
                )}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
