// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useMemo, useEffect, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../utils/fetch'
import { formatTimestamp } from '../../utils/time'
import { getWorkloadStatusBadgeClass, formatWorkloadStatus } from '../../utils/status'
import { summarizePods } from '../../utils/pods'
import { formatScheduleMessage } from '../../utils/cron'
import { usePrismTheme, YamlBlock } from '../dashboards/common/yaml'
import { TabbedPanel, Field, ResourceLink, StatusBadge } from './detailPanel'

/**
 * WorkloadDetailsView - Read-only inline detail panel for a Kubernetes workload
 *
 * @param {Object} props
 * @param {string} props.kind - Workload kind (Deployment, StatefulSet, DaemonSet, CronJob)
 * @param {string} props.name - Workload name
 * @param {string} props.namespace - Workload namespace
 * @param {boolean} props.isExpanded - Whether the view is expanded
 * @param {Function} [props.onReady] - Called once the fetch settles (success or
 *   error), so the parent row can swap its spinner for the revealed panel
 * @param {Function} [props.onData] - Called with the fetched workload on success, so
 *   the parent row can refresh its owning-reconciler summary (status + last
 *   reconciled) from the detail's reconciler reference
 *
 * Features:
 * - Lazy loads workload data from the RBAC-enforced /api/v1/workload endpoint on
 *   first expand and caches it to avoid redundant fetches.
 * - Tabbed interface with three read-only sections: Overview, Specification, Status.
 *   Tabs render through the shared TabbedPanel (mobile segmented control / desktop
 *   vertical rail merging into a cohesive dark content panel).
 * - Mirrors the Resources list detail panel (ResourceDetailsView). Pod listing,
 *   events, and actions live on the full workload dashboard, not here.
 * - Handles loading, error, and not-found states (e.g. forbidden namespaces).
 */
export function WorkloadDetailsView({ kind, name, namespace, reconcilerKind, reconcilerNamespace, reconcilerName, isExpanded, onReady, onData }) {
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
        if (!cancelled) {
          setWorkloadData(data)
          // Hand the fresh payload back so the row can refresh its reconciler
          // summary from the detail's enriched reconciler reference.
          onData && onData(data)
        }
      } catch (err) {
        console.error('Failed to fetch workload details:', err)
        if (!cancelled) setError(err.message)
      } finally {
        fetchingRef.current = false
        if (!cancelled) {
          setLoading(false)
          // Signal the parent row once the fetch settles (success or error) so it
          // can swap its spinner for the revealed panel. Skipped when cancelled
          // (the row was collapsed mid-fetch and unmounted us), otherwise the
          // disclosure would be marked loaded+open with no panel mounted.
          if (onReady) onReady()
        }
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

  // Tab definitions for the shared TabbedPanel (short labels for the compact rail).
  const tabs = [
    { id: 'overview', label: 'Overview' },
    { id: 'specification', label: 'Spec' },
    { id: 'status', label: 'Status' }
  ]

  return (
    <div class="mt-3 space-y-4">
      {/* Loading State */}
      {loading && (
        <div class="py-3 text-xs text-gray-500 dark:text-gray-400">
          Loading details…
        </div>
      )}

      {/* Error State */}
      {error && (
        <div class="py-3 text-xs text-red-600 dark:text-red-400">
          Failed to load details: {error}
        </div>
      )}

      {/* Not Found State (deleted or not visible to the user) */}
      {!loading && !error && isNotFound && (
        <div class="py-3 text-xs text-gray-500 dark:text-gray-400">
          Workload not found in the cluster.
        </div>
      )}

      {/* Tabs + Content */}
      {!loading && !error && workloadData && !isNotFound && (
        <TabbedPanel tabs={tabs} active={activeTab} onSelect={setActiveTab}>
          {/* Overview — two columns: metadata fields left, last action + message right */}
          {activeTab === 'overview' && (
            <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-4 text-xs">
              {/* Left: metadata fields, evenly stacked. Leads with the same
                  "Resource: ns/name link" + "<kind>: status" pair as the
                  resource Overview/Source tabs. */}
              <div class="space-y-2.5 self-start">
                <Field label="Workload"><ResourceLink kind={kind} namespace={namespace} name={name} /></Field>
                <Field label={kind}>
                  <StatusBadge status={formatWorkloadStatus(workloadStatus)} colorClass={getWorkloadStatusBadgeClass(workloadStatus)} />
                </Field>
                <Field label="Pods">
                  {podSummary
                    ? `${podSummary.primary}${podSummary.detail && podSummary.total > 0 ? ` (${podSummary.detail})` : ''}`
                    : null}
                </Field>
                <Field label="Service account">{podSpec?.serviceAccountName || 'default'}</Field>
                {reconcilerName ? (
                  <Field label="Managed by">
                    <ResourceLink kind={reconcilerKind} namespace={reconcilerNamespace} name={reconcilerName} />
                  </Field>
                ) : null}
              </div>

              {/* Right: created + last action + message */}
              {(workloadInfo?.createdAt || lastActionTime || workloadInfo?.statusMessage) && (
                <div class="space-y-2.5 self-start md:border-l md:border-gray-200 md:dark:border-gray-700 md:pl-8">
                  <Field label="Created">{workloadInfo?.createdAt ? formatTimestamp(workloadInfo.createdAt) : null}</Field>
                  <Field label="Last action">{lastActionTime ? formatTimestamp(lastActionTime) : null}</Field>
                  {workloadInfo?.statusMessage && (
                    <pre class="whitespace-pre-wrap break-words font-sans leading-relaxed text-gray-700 dark:text-gray-300">{formatScheduleMessage(workloadInfo.statusMessage)}</pre>
                  )}
                </div>
              )}
            </div>
          )}

          {/* Specification Tab — height is capped/scrolled by the TabbedPanel. */}
          {activeTab === 'specification' && workloadSpecYaml && (
            <YamlBlock data={workloadSpecYaml} nested />
          )}

          {/* Status Tab — height is capped/scrolled by the TabbedPanel. */}
          {activeTab === 'status' && workloadStatusYaml && (
            <YamlBlock data={workloadStatusYaml} nested />
          )}
        </TabbedPanel>
      )}
    </div>
  )
}
