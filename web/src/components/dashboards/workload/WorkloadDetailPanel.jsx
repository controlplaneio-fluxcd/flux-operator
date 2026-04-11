// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo, useEffect, useRef, useState } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { formatTimestamp } from '../../../utils/time'
import { getWorkloadStatusBadgeClass, formatWorkloadStatus, getEventBadgeClass, getContainerStateBadgeClass } from '../../../utils/status'
import { formatScheduleMessage } from '../../../utils/cron'
import { DashboardPanel, TabButton } from '../common/panel'
import { YamlBlock } from '../common/yaml'
import { WorkloadDeleteAction } from '../resource/WorkloadDeleteAction'
import { FluxOperatorIcon } from '../../layout/Icons'
import { useHashTab } from '../../../utils/hash'

// Valid tabs for the WorkloadDetailPanel
const WORKLOAD_TABS = ['overview', 'pods', 'events', 'spec', 'status']

// Highlight threshold for recently changed pods
const RECENT_POD_THRESHOLD = 30000  // 30 seconds

/**
 * Check if a timestamp is within the recent threshold
 */
function isRecentTimestamp(timestamp) {
  if (!timestamp) return false
  const podTime = new Date(timestamp).getTime()
  const now = Date.now()
  return (now - podTime) < RECENT_POD_THRESHOLD
}

/**
 * Get a summary of container readiness and restart counts from podStatus.
 * For completed pods (Succeeded phase), counts terminated containers with exit code 0.
 * For running pods, counts containers with ready=true.
 * @param {object} podStatus - The Kubernetes PodStatus object
 * @returns {{ readyCount: number, totalCount: number, isCompleted: boolean }}
 */
function getContainerSummary(podStatus) {
  const containers = podStatus.containerStatuses || []
  const isCompleted = podStatus.phase === 'Succeeded'
  let readyCount = 0
  for (const cs of containers) {
    if (isCompleted) {
      if (cs.state?.terminated?.exitCode === 0) readyCount++
    } else {
      if (cs.ready) readyCount++
    }
  }
  return { readyCount, totalCount: containers.length, isCompleted }
}

/**
 * Get badge label and CSS class for a container's state.
 * @param {object} state - The container state object with one of: waiting, running, terminated
 * @returns {{ label: string, detail: string, badgeClass: string }}
 */
function getContainerStateBadge(state) {
  if (!state) return { label: 'Unknown', detail: '', badgeClass: getContainerStateBadgeClass('Unknown') }

  if (state.waiting) {
    return {
      label: 'Waiting',
      detail: state.waiting.reason || '',
      badgeClass: getContainerStateBadgeClass('Waiting')
    }
  }
  if (state.terminated) {
    const isSuccess = state.terminated.exitCode === 0
    const label = isSuccess ? 'Completed' : 'Terminated'
    let detail = ''
    if (!isSuccess) {
      detail = state.terminated.reason || ''
      if (state.terminated.exitCode !== undefined) {
        detail += ` (exit ${state.terminated.exitCode})`
      }
    }
    return {
      label,
      detail: detail.trim(),
      badgeClass: getContainerStateBadgeClass(isSuccess ? 'Completed' : 'Terminated')
    }
  }
  if (state.running) {
    return {
      label: 'Running',
      detail: '',
      badgeClass: getContainerStateBadgeClass('Running')
    }
  }

  return { label: 'Unknown', detail: '', badgeClass: getContainerStateBadgeClass('Unknown') }
}

/**
 * WorkloadDetailPanel - Displays workload details including overview, pods, events, spec, and status.
 * Owns tab state, events lazy-loading, and pod sorting/highlighting.
 */
export function WorkloadDetailPanel({
  kind, namespace, workloadData, workloadInfo, workloadStatus,
  pendingDeletions, onPodDeleteStart, onPodDeleteFailed,
  onActionStart, onActionComplete
}) {
  // Tab state synced with URL hash
  const [workloadTab, setWorkloadTab] = useHashTab('workload', 'overview', WORKLOAD_TABS, 'workload-panel')

  // Expanded pods state (set of pod names)
  const [expandedPods, setExpandedPods] = useState(new Set())

  // Events data state (lazy-loaded)
  const [eventsData, setEventsData] = useState([])
  const [eventsLoading, setEventsLoading] = useState(false)
  const [eventsLoaded, setEventsLoaded] = useState(false)

  // Track initial mount to avoid refetching on first render
  const isInitialMount = useRef(true)

  // Sorted pods for display (most recent first)
  const sortedPods = useMemo(() => {
    const pods = workloadInfo?.pods || []
    return [...pods].sort((a, b) => {
      if (!a.createdAt && !b.createdAt) return 0
      if (!a.createdAt) return 1
      if (!b.createdAt) return -1
      return new Date(b.createdAt) - new Date(a.createdAt)
    })
  }, [workloadInfo])

  // Resolve the pod spec based on workload kind
  const podSpec = useMemo(() => {
    if (!workloadData?.spec) return null
    return kind === 'CronJob'
      ? workloadData.spec.jobTemplate?.spec?.template?.spec
      : workloadData.spec.template?.spec
  }, [workloadData, kind])

  // Extract and sort unique container ports from the spec
  const containerPorts = useMemo(() => {
    if (!podSpec) return []
    const containers = [...(podSpec.containers || []), ...(podSpec.initContainers || [])]
    const ports = new Set()
    for (const c of containers) {
      for (const p of c.ports || []) {
        if (p.containerPort) ports.add(p.containerPort)
      }
    }
    return [...ports].sort((a, b) => a - b)
  }, [podSpec])

  // Last action timestamp: conditions for apps/v1, lastScheduleTime for CronJob
  const lastActionTime = useMemo(() => {
    if (!workloadData?.status) return null
    // Find the most recent lastUpdateTime across all conditions
    const conditions = workloadData.status.conditions || []
    const lastUpdateTime = conditions.reduce((latest, c) => {
      if (!c.lastUpdateTime) return latest
      if (!latest) return c.lastUpdateTime
      return new Date(c.lastUpdateTime) > new Date(latest) ? c.lastUpdateTime : latest
    }, null)
    if (lastUpdateTime) return lastUpdateTime
    if (workloadData.status.lastScheduleTime) return workloadData.status.lastScheduleTime
    return workloadInfo?.createdAt || null
  }, [workloadData, workloadInfo])

  // Memoized YAML data for workload
  const workloadSpecYaml = useMemo(() => {
    if (!workloadData) return null
    return {
      apiVersion: workloadData.apiVersion,
      kind: workloadData.kind,
      metadata: workloadData.metadata,
      spec: workloadData.spec
    }
  }, [workloadData])

  const workloadStatusYaml = useMemo(() => {
    if (!workloadData?.status) return null
    return {
      apiVersion: workloadData.apiVersion,
      kind: workloadData.kind,
      metadata: { name: workloadData.metadata.name, namespace: workloadData.metadata.namespace },
      status: workloadData.status
    }
  }, [workloadData])

  // Fetch events on demand when Events tab is clicked
  useEffect(() => {
    if (workloadTab === 'events' && !eventsLoaded && !eventsLoading && workloadData?.metadata) {
      const fetchEvents = async () => {
        setEventsLoading(true)
        const params = new URLSearchParams({
          kind: workloadData.kind,
          name: workloadData.metadata.name,
          namespace: workloadData.metadata.namespace
        })

        try {
          const eventsResp = await fetchWithMock({
            endpoint: `/api/v1/events?${params.toString()}`,
            mockPath: '../mock/events',
            mockExport: 'getMockEvents'
          })
          setEventsData(eventsResp?.events || [])
          setEventsLoaded(true)
        } catch (err) {
          console.error('Failed to fetch workload events:', err)
        } finally {
          setEventsLoading(false)
        }
      }

      fetchEvents()
    }
  }, [workloadTab, eventsLoaded, eventsLoading, workloadData])

  // Refetch events when workloadData changes (auto-refresh)
  useEffect(() => {
    if (isInitialMount.current) {
      if (workloadData) isInitialMount.current = false
      return
    }

    if (workloadTab === 'events' && eventsLoaded && !eventsLoading && workloadData?.metadata) {
      const refetchEvents = async () => {
        const params = new URLSearchParams({
          kind: workloadData.kind,
          name: workloadData.metadata.name,
          namespace: workloadData.metadata.namespace
        })

        try {
          const eventsResp = await fetchWithMock({
            endpoint: `/api/v1/events?${params.toString()}`,
            mockPath: '../mock/events',
            mockExport: 'getMockEvents'
          })
          setEventsData(eventsResp?.events || [])
        } catch (err) {
          console.error('Failed to refetch workload events:', err)
        }
      }

      refetchEvents()
    }
  }, [workloadData])

  return (
    <DashboardPanel title={kind} id="workload-panel">
      {/* Tab Navigation */}
      <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
        <nav class="flex space-x-4">
          <TabButton active={workloadTab === 'overview'} onClick={() => setWorkloadTab('overview')}>
            <span class="sm:hidden">Info</span>
            <span class="hidden sm:inline">Overview</span>
          </TabButton>
          <TabButton active={workloadTab === 'pods'} onClick={() => setWorkloadTab('pods')}>
            Pods
          </TabButton>
          <TabButton active={workloadTab === 'events'} onClick={() => setWorkloadTab('events')}>
            Events
          </TabButton>
          <TabButton active={workloadTab === 'spec'} onClick={() => setWorkloadTab('spec')}>
            <span class="sm:hidden">Spec</span>
            <span class="hidden sm:inline">Specification</span>
          </TabButton>
          <TabButton active={workloadTab === 'status'} onClick={() => setWorkloadTab('status')}>
            Status
          </TabButton>
        </nav>
      </div>

      {/* Workload Overview Tab */}
      {workloadTab === 'overview' && (
        <div class="space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Left column: Status and metadata */}
            <div class="space-y-4">
              {/* Status Badge */}
              <div class="text-sm">
                <span class="text-gray-500 dark:text-gray-400">Status</span>
                <span class={`ml-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getWorkloadStatusBadgeClass(workloadStatus)}`}>
                  {formatWorkloadStatus(workloadStatus)}
                </span>
              </div>

              {/* Created At */}
              {workloadInfo?.createdAt && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Created</span>
                  <span class="ml-1 text-gray-900 dark:text-white">{new Date(workloadInfo.createdAt).toLocaleString().replace(',', '')}</span>
                </div>
              )}

              {/* CronJob concurrency policy (spec.concurrencyPolicy) */}
              {kind === 'CronJob' && workloadData?.spec?.concurrencyPolicy && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Concurrency policy</span>
                  <span class="ml-1 text-gray-900 dark:text-white">{workloadData.spec.concurrencyPolicy}</span>
                </div>
              )}

              {/* Deployment rollout strategy (spec.strategy) */}
              {kind === 'Deployment' && workloadData?.spec?.strategy?.type && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Strategy</span>
                  <span class="ml-1 text-gray-900 dark:text-white">
                    {workloadData.spec.strategy.type}
                    {workloadData.spec.strategy.rollingUpdate && (
                      <span class="text-gray-500 dark:text-gray-400">
                        {' '}(maxUnavailable: {workloadData.spec.strategy.rollingUpdate.maxUnavailable || 'N/A'}, maxSurge: {workloadData.spec.strategy.rollingUpdate.maxSurge || 'N/A'})
                      </span>
                    )}
                  </span>
                </div>
              )}

              {/* StatefulSet/DaemonSet update strategy (spec.updateStrategy) */}
              {(kind === 'StatefulSet' || kind === 'DaemonSet') && workloadData?.spec?.updateStrategy?.type && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Update strategy</span>
                  <span class="ml-1 text-gray-900 dark:text-white">
                    {workloadData.spec.updateStrategy.type}
                    {workloadData.spec.updateStrategy.rollingUpdate && kind === 'DaemonSet' && (
                      <span class="text-gray-500 dark:text-gray-400">
                        {' '}(maxUnavailable: {workloadData.spec.updateStrategy.rollingUpdate.maxUnavailable || 'N/A'}, maxSurge: {workloadData.spec.updateStrategy.rollingUpdate.maxSurge || 'N/A'})
                      </span>
                    )}
                    {workloadData.spec.updateStrategy.rollingUpdate && kind === 'StatefulSet' && (
                      <span class="text-gray-500 dark:text-gray-400">
                        {' '}(maxUnavailable: {workloadData.spec.updateStrategy.rollingUpdate.maxUnavailable || 'N/A'}, partition: {workloadData.spec.updateStrategy.rollingUpdate.partition ?? 'N/A'})
                      </span>
                    )}
                  </span>
                </div>
              )}

              {/* Service Account */}
              {podSpec?.serviceAccountName && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Service account</span>
                  <span class="ml-1 text-gray-900 dark:text-white">{podSpec.serviceAccountName}</span>
                </div>
              )}

              {/* Container Ports */}
              {containerPorts.length > 0 && (
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Ports</span>
                  <span class="ml-1 text-gray-900 dark:text-white">{containerPorts.join(', ')}</span>
                </div>
              )}
            </div>

            {/* Right column: Last action and status message */}
            {workloadInfo?.statusMessage && (
              <div class="space-y-2 border-gray-200 dark:border-gray-700 border-t pt-4 md:border-t-0 md:border-l md:pt-0 md:pl-6">
                {lastActionTime && (
                  <div class="text-sm text-gray-500 dark:text-gray-400">
                    Last action <span class="text-gray-900 dark:text-white">{new Date(lastActionTime).toLocaleString().replace(',', '')}</span>
                  </div>
                )}
                <div class="text-sm text-gray-700 dark:text-gray-300">
                  <pre class="whitespace-pre-wrap break-all font-sans">{formatScheduleMessage(workloadInfo.statusMessage)}</pre>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Workload Pods Tab */}
      {workloadTab === 'pods' && (
        <div class="space-y-4">
          {/* Pods list */}
          {sortedPods.length > 0 ? (
            <div class="space-y-2">
              {sortedPods.map((pod) => {
                const isRecent = isRecentTimestamp(pod.createdAt)
                const isPendingDeletion = pendingDeletions.has(pod.name)
                const displayStatus = isPendingDeletion ? 'Terminating' : pod.status
                const hasPodStatus = !!pod.podStatus
                const isExpanded = expandedPods.has(pod.name)
                const summary = hasPodStatus ? getContainerSummary(pod.podStatus) : null
                return (
                  <div
                    key={pod.name}
                    class={`rounded px-3 py-2 border border-gray-200 dark:border-gray-700 ${
                      isRecent ? 'ring-2 ring-blue-400 dark:ring-blue-500 ring-opacity-50' : ''
                    }`}
                    data-testid={isRecent ? 'recent-pod' : undefined}
                  >
                    <div
                      class={`flex items-center justify-between ${hasPodStatus ? 'cursor-pointer' : ''}`}
                      onClick={(e) => {
                        if (!hasPodStatus) return
                        // Don't toggle when clicking the delete button
                        if (e.target.closest('[data-testid="workload-delete-action"]')) return
                        setExpandedPods(prev => {
                          const next = new Set(prev)
                          if (next.has(pod.name)) {
                            next.delete(pod.name)
                          } else {
                            next.add(pod.name)
                          }
                          return next
                        })
                      }}
                    >
                      <div class="flex items-center flex-grow mr-2 min-w-0">
                        {hasPodStatus && (
                          <svg class={`w-3 h-3 mr-1.5 flex-shrink-0 text-gray-400 dark:text-gray-500 transition-transform ${isExpanded ? 'rotate-90' : ''}`} data-testid="pod-chevron" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
                          </svg>
                        )}
                        <span class="text-sm text-gray-900 dark:text-white truncate">
                          {pod.name}
                        </span>
                      </div>
                      <div class="flex items-center gap-1.5 flex-shrink-0">
                        <span class={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${getWorkloadStatusBadgeClass(displayStatus)}`}>
                          {formatWorkloadStatus(displayStatus)}
                        </span>
                        {workloadInfo?.userActions?.includes('deletePods') && (
                          <WorkloadDeleteAction
                            namespace={namespace}
                            name={pod.name}
                            isPendingDeletion={pendingDeletions.has(pod.name)}
                            onActionStart={onActionStart}
                            onActionComplete={onActionComplete}
                            onPodDeleteStart={onPodDeleteStart}
                            onPodDeleteFailed={onPodDeleteFailed}
                          />
                        )}
                      </div>
                    </div>
                    {pod.statusMessage && (
                      <div class="flex items-start gap-1.5 mt-1">
                        <svg class="w-3.5 h-3.5 text-gray-400 dark:text-gray-500 flex-shrink-0 mt-px" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <span class="text-xs text-gray-600 dark:text-gray-400 break-all">{pod.statusMessage}</span>
                      </div>
                    )}
                    {pod.createdBy && (
                      <div class="flex items-center gap-1.5 mt-1" data-testid="pod-created-by">
                        <svg class="w-3.5 h-3.5 text-gray-400 dark:text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                        </svg>
                        <span class="text-xs text-gray-500 dark:text-gray-400">Triggered by {pod.createdBy}</span>
                      </div>
                    )}
                    {/* Container summary line */}
                    {summary && (
                      <div class="flex items-center gap-1.5 mt-1" data-testid="pod-container-summary">
                        <svg class="w-3.5 h-3.5 text-gray-400 dark:text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
                        </svg>
                        <span class="text-xs text-gray-600 dark:text-gray-400">
                          Containers: {summary.readyCount}/{summary.totalCount} {summary.isCompleted ? 'completed' : 'ready'}
                        </span>
                      </div>
                    )}
                    {/* Expanded container details */}
                    {isExpanded && hasPodStatus && (
                      <div class="mt-2 pt-2 border-t border-gray-100 dark:border-gray-700" data-testid="pod-expanded-details">
                        {/* Container statuses */}
                        <span class="text-xs font-medium text-gray-500 dark:text-gray-400">Containers</span>
                        <div class="mt-1.5 space-y-2">
                          {[
                            ...(pod.podStatus.initContainerStatuses || []).map(cs => ({ ...cs, isInit: true })),
                            ...(pod.podStatus.containerStatuses || [])
                          ].map((cs) => {
                            const stateInfo = getContainerStateBadge(cs.state)
                            const rawImage = cs.image && !cs.image.startsWith('sha256:') ? cs.image : (cs.imageID || '').replace(/^docker-pullable:\/\//, '')
                            return (
                              <div key={cs.name} class="bg-white dark:bg-gray-900 rounded px-3 py-2 text-xs" data-testid="container-status">
                                <div class="flex items-center justify-between">
                                  <div class="flex items-center gap-1.5 min-w-0">
                                    <span class="text-gray-900 dark:text-white font-medium truncate">
                                      {cs.isInit ? `init:${cs.name}` : cs.name}
                                    </span>
                                    {(cs.restartCount || 0) > 0 && (
                                      <span class="text-gray-500 dark:text-gray-500">({cs.restartCount} restarts)</span>
                                    )}
                                  </div>
                                  <span class={`inline-flex items-center px-1.5 py-0.5 rounded-full text-xs font-medium ${stateInfo.badgeClass} ml-2 flex-shrink-0`}>
                                    {stateInfo.label}
                                  </span>
                                </div>
                                {rawImage && (
                                  <p class="text-gray-500 dark:text-gray-400 mt-1 truncate">{rawImage}</p>
                                )}
                                {stateInfo.detail && (
                                  <p class="text-gray-600 dark:text-gray-400 mt-0.5">{stateInfo.detail}</p>
                                )}
                                {cs.state?.waiting?.message && (
                                  <p class="text-gray-500 dark:text-gray-400 mt-0.5 break-all">{cs.state.waiting.message}</p>
                                )}
                                {cs.state?.terminated?.message && (
                                  <p class="text-gray-500 dark:text-gray-400 mt-0.5 break-all">{cs.state.terminated.message}</p>
                                )}
                              </div>
                            )
                          })}
                        </div>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          ) : (
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {kind === 'CronJob' ? 'No recent jobs' : 'No pods found'}
            </p>
          )}
        </div>
      )}

      {/* Workload Events Tab */}
      {workloadTab === 'events' && (
        <div>
          {eventsLoading ? (
            <div class="flex items-center justify-center p-8">
              <FluxOperatorIcon className="animate-spin h-8 w-8 text-flux-blue" />
              <span class="ml-3 text-gray-600 dark:text-gray-400">Loading events...</span>
            </div>
          ) : eventsData.length === 0 ? (
            <p class="text-sm text-gray-500 dark:text-gray-400">No events found</p>
          ) : (
            <div class="space-y-4">
              {eventsData.map((event, idx) => {
                const displayStatus = event.type === 'Normal' ? 'Info' : 'Warning'
                return (
                  <div key={idx} class="card p-4 hover:shadow-md transition-shadow">
                    <div class="flex items-center justify-between mb-3">
                      <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getEventBadgeClass(event.type)}`}>
                        {displayStatus}
                      </span>
                      <span class="text-xs text-gray-500 dark:text-gray-400">{formatTimestamp(event.lastTimestamp)}</span>
                    </div>
                    <div class="text-sm text-gray-700 dark:text-gray-300">
                      <pre class="whitespace-pre-wrap break-all font-sans">{event.message}</pre>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}

      {/* Workload Specification Tab */}
      {workloadTab === 'spec' && <YamlBlock data={workloadSpecYaml} />}

      {/* Workload Status Tab */}
      {workloadTab === 'status' && <YamlBlock data={workloadStatusYaml} />}
    </DashboardPanel>
  )
}
