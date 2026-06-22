// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../utils/fetch'
import { getWorkloadStatusBadgeClass, formatWorkloadStatus } from '../../utils/status'
import { summarizePods } from '../../utils/pods'
import { formatScheduleMessage } from '../../utils/cron'
import { usePrismTheme, YamlBlock } from '../dashboards/common/yaml'
import { FluxOperatorIcon } from '../layout/Icons'

/**
 * WorkloadDetailsView - Read-only inline detail panel for a Kubernetes workload
 *
 * @param {Object} props
 * @param {string} props.kind - Workload kind (Deployment, StatefulSet, DaemonSet, CronJob)
 * @param {string} props.name - Workload name
 * @param {string} props.namespace - Workload namespace
 * @param {boolean} props.isExpanded - Whether the view is expanded
 *
 * Features:
 * - Lazy loads workload data from the RBAC-enforced /api/v1/workload endpoint on
 *   first expand and caches it to avoid redundant fetches.
 * - Tabbed interface with three read-only sections: Overview, Specification, Status.
 * - Mirrors the Resources list detail panel (ResourceDetailsView). Pod listing,
 *   events, and actions live on the full workload dashboard, not here.
 * - Handles loading, error, and not-found states (e.g. forbidden namespaces).
 */
export function WorkloadDetailsView({ kind, name, namespace, isExpanded }) {
  const [workloadData, setWorkloadData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [activeTab, setActiveTab] = useState('overview')
  const fetchingRef = useRef(false)

  // Load Prism theme based on current app theme
  usePrismTheme()

  // Reset state when workload identity changes
  useEffect(() => {
    setWorkloadData(null)
    setError(null)
    setActiveTab('overview')
  }, [kind, name, namespace])

  // Fetch workload details when expanded
  useEffect(() => {
    if (!isExpanded || workloadData || fetchingRef.current) return

    let cancelled = false
    fetchingRef.current = true

    const fetchWorkloadDetails = async () => {
      if (!cancelled) {
        setLoading(true)
        setError(null)
      }

      const params = new URLSearchParams({ kind, name, namespace })

      try {
        const data = await fetchWithMock({
          endpoint: `/api/v1/workload?${params.toString()}`,
          mockPath: '../mock/workload',
          mockExport: 'getMockWorkload'
        })
        if (!cancelled) setWorkloadData(data)
      } catch (err) {
        console.error('Failed to fetch workload details:', err)
        if (!cancelled) setError(err.message)
      } finally {
        fetchingRef.current = false
        if (!cancelled) setLoading(false)
      }
    }

    fetchWorkloadDetails()
    return () => { cancelled = true }
  }, [isExpanded, kind, name, namespace, workloadData])

  const workloadInfo = workloadData?.workloadInfo
  const workloadStatus = workloadInfo?.status || 'Unknown'

  // Pod readiness/phase summary shown in the overview (CronJobs report completion)
  const podSummary = useMemo(
    () => summarizePods(workloadInfo?.pods, kind),
    [workloadInfo, kind]
  )

  // True when the object carries no metadata.name, i.e. it was not found or the
  // user is not allowed to read it in this namespace.
  const isNotFound = !!workloadData && !workloadData?.metadata?.name

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

  // Memoized YAML data for the spec and status tabs
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

  if (!isExpanded) return null

  return (
    <div class="mt-3 space-y-4">
      {/* Loading State */}
      {loading && (
        <div class="flex items-center justify-center p-4">
          <FluxOperatorIcon className="animate-spin h-6 w-6 text-flux-blue" />
          <span class="ml-2 text-sm text-gray-600 dark:text-gray-400">
            Loading details...
          </span>
        </div>
      )}

      {/* Error State */}
      {error && (
        <div class="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
          <p class="text-sm text-red-800 dark:text-red-200">
            Failed to load details: {error}
          </p>
        </div>
      )}

      {/* Not Found State (deleted or not visible to the user) */}
      {!loading && !error && isNotFound && (
        <div class="p-3 bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 rounded-md">
          <p class="text-sm text-gray-600 dark:text-gray-400">
            Workload not found in the cluster.
          </p>
        </div>
      )}

      {/* Tabs + Content */}
      {!loading && !error && workloadData && !isNotFound && (
        <>
          {/* Tab Navigation */}
          <div class="border-b border-gray-200 dark:border-gray-700 mb-4">
            <nav class="flex space-x-4 overflow-x-auto" aria-label="Tabs">
              <button
                onClick={() => setActiveTab('overview')}
                class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'overview'
                    ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                }`}
              >
                <span class="inline sm:hidden">Info</span>
                <span class="hidden sm:inline">Overview</span>
              </button>
              <button
                onClick={() => setActiveTab('specification')}
                class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'specification'
                    ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                }`}
              >
                <span class="inline sm:hidden">Spec</span>
                <span class="hidden sm:inline">Specification</span>
              </button>
              <button
                onClick={() => setActiveTab('status')}
                class={`py-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'status'
                    ? 'border-flux-blue text-flux-blue dark:text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300'
                }`}
              >
                Status
              </button>
            </nav>
          </div>

          {/* Overview Tab */}
          {activeTab === 'overview' && (
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Left column: status and metadata */}
              <div class="space-y-4">
                {/* Status Badge */}
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">{kind}</span>
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

                {/* Pods readiness / phase summary */}
                <div class="text-sm">
                  <span class="text-gray-500 dark:text-gray-400">Pods</span>
                  <span class="ml-1 text-gray-900 dark:text-white">{podSummary.primary}</span>
                  {podSummary.detail && podSummary.total > 0 && (
                    <span class="text-gray-500 dark:text-gray-400"> ({podSummary.detail})</span>
                  )}
                </div>

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

              {/* Right column: last action and status message */}
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
          )}

          {/* Specification Tab */}
          {activeTab === 'specification' && (
            <YamlBlock data={workloadSpecYaml} />
          )}

          {/* Status Tab */}
          {activeTab === 'status' && (
            <YamlBlock data={workloadStatusYaml} />
          )}
        </>
      )}
    </div>
  )
}
