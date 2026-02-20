// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useRef, useCallback, useMemo } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { usePrismTheme } from '../common/yaml'
import { formatTime } from '../../../utils/time'
import { usePageMeta } from '../../../utils/meta'
import { ActionBar } from '../resource/ActionBar'
import { WorkloadActionBar } from '../resource/WorkloadActionBar'
import { WorkloadReconcilerPanel } from './WorkloadReconcilerPanel'
import { WorkloadPipelinePanel } from './WorkloadPipelinePanel'
import { WorkloadDetailPanel } from './WorkloadDetailPanel'

// Polling intervals
const POLL_INTERVAL_MS = 10000  // 10 seconds (workloads change more frequently)
const FAST_POLL_INTERVAL_MS = 5000  // 5 seconds after actions
const FAST_POLL_TIMEOUT_MS = 60000  // 60 seconds to revert

/**
 * Get loading status styling info
 */
function getLoadingStatusInfo() {
  return {
    color: 'text-blue-600 dark:text-blue-400',
    bgColor: 'bg-blue-50',
    borderColor: 'border-blue-500',
    icon: (
      <svg class="w-10 h-10 text-blue-600 dark:text-blue-400 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
      </svg>
    )
  }
}

/**
 * Get error status styling info
 */
function getErrorStatusInfo() {
  return {
    color: 'text-danger',
    bgColor: 'bg-red-50',
    borderColor: 'border-danger',
    icon: (
      <svg class="w-10 h-10 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    )
  }
}

/**
 * Get not found status styling info
 */
function getNotFoundStatusInfo() {
  return {
    color: 'text-gray-600 dark:text-gray-400',
    bgColor: 'bg-gray-50',
    borderColor: 'border-gray-400',
    icon: (
      <svg class="w-10 h-10 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    )
  }
}

/**
 * Get workload status styling info for the header card.
 * Uses kstatus values (Current, InProgress, Failed, Terminating, Unknown).
 */
function getWorkloadStatusInfo(status) {
  switch (status) {
  case 'Current':
  case 'Idle':
    return {
      color: 'text-success',
      bgColor: 'bg-green-50',
      borderColor: 'border-success',
      icon: (
        <svg class="w-10 h-10 text-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
      )
    }
  case 'InProgress':
  case 'Progressing':
    return {
      color: 'text-blue-600 dark:text-blue-400',
      bgColor: 'bg-blue-50',
      borderColor: 'border-blue-500',
      icon: (
        <svg class="w-10 h-10 text-blue-600 dark:text-blue-400 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      )
    }
  case 'Failed':
    return {
      color: 'text-danger',
      bgColor: 'bg-red-50',
      borderColor: 'border-danger',
      icon: (
        <svg class="w-10 h-10 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      )
    }
  case 'Terminating':
  case 'Suspended':
    return {
      color: 'text-yellow-600 dark:text-yellow-400',
      bgColor: 'bg-yellow-50',
      borderColor: 'border-yellow-500',
      icon: (
        <svg class="w-10 h-10 text-yellow-600 dark:text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    }
  default:
    return {
      color: 'text-gray-600 dark:text-gray-400',
      bgColor: 'bg-gray-50',
      borderColor: 'border-gray-400',
      icon: (
        <svg class="w-10 h-10 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    }
  }
}

/**
 * WorkloadPage - Full page dashboard for a single Kubernetes workload
 */
export function WorkloadPage({ kind, namespace, name }) {
  // Set page title and description
  usePageMeta(name, `${kind}/${namespace}/${name} workload dashboard`)

  // State
  const [workloadData, setWorkloadData] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [lastUpdatedAt, setLastUpdatedAt] = useState(null)
  const [pendingDeletions, setPendingDeletions] = useState(new Set())

  // Track fast polling mode (activated by user actions)
  const [fastPolling, setFastPolling] = useState(false)
  const fastPollTimeoutRef = useRef(null)

  // Use faster polling when recently activated by action
  const currentPollInterval = fastPolling ? FAST_POLL_INTERVAL_MS : POLL_INTERVAL_MS

  // Load Prism theme based on current app theme
  usePrismTheme()

  // Reset state when navigating to a different workload
  useEffect(() => {
    setWorkloadData(null)
    setLoading(true)
    setError(null)
    setPendingDeletions(new Set())
  }, [kind, namespace, name])

  // Fetch workload data
  const fetchData = useCallback(async () => {
    setError(null)
    const params = new URLSearchParams({ kind, name, namespace })

    try {
      const resp = await fetchWithMock({
        endpoint: `/api/v1/workload?${params.toString()}`,
        mockPath: '../mock/workload',
        mockExport: 'getMockWorkload'
      })

      setWorkloadData(resp)
      setLastUpdatedAt(new Date())
      setError(null)

      // Clean up pending deletions for pods that have disappeared
      const pods = resp?.workloadInfo?.pods || []
      setPendingDeletions(prev => {
        if (prev.size === 0) return prev
        const allPodNames = new Set(pods.map(p => p.name))
        const next = new Set([...prev].filter(n => allPodNames.has(n)))
        return next.size === prev.size ? prev : next
      })
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [kind, name, namespace])

  // Fetch data on mount and when route params change
  useEffect(() => {
    fetchData()
  }, [kind, namespace, name])

  // Setup polling with dynamic interval
  useEffect(() => {
    const interval = setInterval(fetchData, currentPollInterval)
    return () => clearInterval(interval)
  }, [kind, namespace, name, currentPollInterval])

  // Cleanup fast poll timeout on unmount
  useEffect(() => {
    return () => {
      if (fastPollTimeoutRef.current) {
        window.clearTimeout(fastPollTimeoutRef.current)
      }
    }
  }, [])

  // Handle action start - switch to fast polling with timeout
  const handleActionStart = useCallback(() => {
    setFastPolling(true)

    if (fastPollTimeoutRef.current) {
      window.clearTimeout(fastPollTimeoutRef.current)
    }

    fastPollTimeoutRef.current = window.setTimeout(() => {
      setFastPolling(false)
    }, FAST_POLL_TIMEOUT_MS)
  }, [])

  // Track pod deletion start
  const handlePodDeleteStart = useCallback((podName) => {
    setPendingDeletions(prev => new Set([...prev, podName]))
  }, [])

  // Remove pod from pending deletions on error
  const handlePodDeleteFailed = useCallback((podName) => {
    setPendingDeletions(prev => {
      const next = new Set(prev)
      next.delete(podName)
      return next
    })
  }, [])

  // Determine display state
  const isStaleData = workloadData?.kind && workloadData.kind !== kind
  const isInitialLoading = (loading && !workloadData) || isStaleData
  const isInitialError = error && !workloadData && !isStaleData
  const isNotFound = !isInitialLoading && !isInitialError && (!workloadData || !workloadData.metadata || !workloadData.metadata.name)
  const isSuccess = !isInitialLoading && !isInitialError && !isNotFound

  // Derived data (only valid when we have workloadData)
  const workloadInfo = workloadData?.workloadInfo
  const workloadStatus = workloadInfo?.status || 'Unknown'
  const reconciler = workloadInfo?.reconciler

  // Find the most recent triggered pod (has createdBy set) for CronJob Run Job button
  const triggeredPod = useMemo(() => {
    const pods = workloadInfo?.pods || []
    return pods.reduce((latest, pod) => {
      if (!pod.createdBy || !pod.createdAt) return latest
      if (!latest) return pod
      return new Date(pod.createdAt) > new Date(latest.createdAt) ? pod : latest
    }, null)
  }, [workloadInfo])

  // Check if either action bar has actions to show
  const hasReconcilerActions = reconciler?.status?.userActions?.length > 0
  const hasWorkloadActions = workloadInfo?.userActions?.includes('restart')

  // Compute statusInfo based on display state
  let statusInfo
  if (isInitialLoading) {
    statusInfo = getLoadingStatusInfo()
  } else if (isInitialError || isNotFound) {
    statusInfo = isNotFound ? getNotFoundStatusInfo() : getErrorStatusInfo()
  } else {
    statusInfo = getWorkloadStatusInfo(workloadStatus)
  }

  return (
    <main data-testid="workload-dashboard-view" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-grow w-full">
      <div class="space-y-6">

        {/* Header Card */}
        <div class={`card ${statusInfo.bgColor} dark:bg-opacity-20 border-2 ${statusInfo.borderColor}`}>
          <div class="flex items-center space-x-4">
            <div class="flex-shrink-0">
              <div class={`w-16 h-16 rounded-full ${statusInfo.bgColor} dark:bg-opacity-30 flex items-center justify-center`}>
                {statusInfo.icon}
              </div>
            </div>
            <div class="flex-grow min-w-0">
              <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{kind}</span>
              <h1 class="text-lg sm:text-2xl font-semibold text-gray-900 dark:text-white break-all">
                {name}
              </h1>
              <span class="text-xs sm:text-sm text-gray-500 dark:text-gray-400">Namespace: {namespace}</span>
            </div>
            {isSuccess && (
              <div class="hidden md:block text-right flex-shrink-0">
                <div class="text-sm text-gray-600 dark:text-gray-400">Last Updated</div>
                <div class="text-lg font-semibold text-gray-900 dark:text-white">{formatTime(lastUpdatedAt)}</div>
              </div>
            )}
          </div>
        </div>

        {/* Loading message */}
        {isInitialLoading && (
          <div data-testid="loading-message" class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md p-4">
            <p class="text-sm text-blue-800 dark:text-blue-200">Loading workload data...</p>
          </div>
        )}

        {/* Error message */}
        {isInitialError && (
          <div data-testid="error-message" class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
            <p class="text-sm text-red-800 dark:text-red-200">Failed to load workload: {error}</p>
          </div>
        )}

        {/* Not found message */}
        {isNotFound && (
          <div data-testid="not-found-message" class="bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 rounded-md p-4">
            <p class="text-sm text-gray-600 dark:text-gray-400">Workload not found in the cluster.</p>
          </div>
        )}

        {/* Success content */}
        {isSuccess && (
          <>
            {/* Action Bars - Flux reconciler actions + workload actions on same line */}
            {(hasReconcilerActions || hasWorkloadActions) && (
              <div class="flex flex-wrap items-center gap-2" data-testid="combined-action-bar">
                {reconciler && (
                  <ActionBar
                    kind={reconciler.kind}
                    namespace={reconciler.metadata.namespace}
                    name={reconciler.metadata.name}
                    resourceData={reconciler}
                    onActionComplete={fetchData}
                    onActionStart={handleActionStart}
                  />
                )}
                {hasReconcilerActions && hasWorkloadActions && (
                  <div class="w-px h-5 bg-gray-300 dark:bg-gray-600" data-testid="action-bar-separator" />
                )}
                {hasWorkloadActions && (
                  <WorkloadActionBar
                    kind={kind}
                    namespace={namespace}
                    name={name}
                    status={workloadStatus}
                    restartedAt={workloadInfo?.restartedAt}
                    lastTriggeredAt={triggeredPod?.createdAt}
                    lastTriggeredPodStatus={triggeredPod?.status}
                    userActions={workloadInfo?.userActions}
                    onActionStart={handleActionStart}
                    onActionComplete={fetchData}
                  />
                )}
              </div>
            )}

            {/* Pipeline Panel */}
            <WorkloadPipelinePanel
              reconciler={reconciler}
              kind={kind}
              name={name}
              workloadStatus={workloadStatus}
              pods={workloadInfo?.pods}
            />

            {/* Workload Detail Panel */}
            <WorkloadDetailPanel
              kind={kind}
              namespace={namespace}
              name={name}
              workloadData={workloadData}
              workloadInfo={workloadInfo}
              workloadStatus={workloadStatus}
              pendingDeletions={pendingDeletions}
              onPodDeleteStart={handlePodDeleteStart}
              onPodDeleteFailed={handlePodDeleteFailed}
              onActionStart={handleActionStart}
              onActionComplete={fetchData}
            />

            {/* Reconciler Panel */}
            {reconciler && (
              <WorkloadReconcilerPanel
                reconciler={reconciler}
                workloadData={workloadData}
              />
            )}
          </>
        )}

      </div>
    </main>
  )
}
