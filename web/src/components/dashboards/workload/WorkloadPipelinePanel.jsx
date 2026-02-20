// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { formatWorkloadStatus } from '../../../utils/status'

/**
 * Get border color class for pipeline node status.
 * Uses the same color scheme as GraphTabContent's getStatusBorderClass.
 * @param {string} status - Status value
 * @returns {string} Tailwind CSS border classes
 */
function getStatusBorderClass(status) {
  switch (status) {
  case 'Ready':
  case 'Current':
  case 'Idle':
    return 'border border-green-500 dark:border-green-400'
  case 'Failed':
    return 'border border-red-500 dark:border-red-400'
  case 'Progressing':
  case 'InProgress':
    return 'border border-blue-500 dark:border-blue-400'
  case 'Suspended':
    return 'border border-yellow-500 dark:border-yellow-400'
  default:
    return 'border border-gray-400 dark:border-gray-500'
  }
}

/**
 * Check if a pod is truly ready using the Kubernetes Ready condition.
 * Falls back to checking the pod phase when podStatus is not available.
 * @param {object} pod - Pod object with status and optional podStatus
 * @returns {boolean} true if the pod is ready
 */
function isPodReady(pod) {
  // Use the Ready condition from podStatus when available (detail endpoint)
  if (pod.podStatus?.conditions) {
    return pod.podStatus.conditions.some(c => c.type === 'Ready' && c.status === 'True')
  }
  // Fallback: treat Running as ready when podStatus is not available
  return pod.status === 'Running' || pod.status === 'Succeeded'
}

/**
 * Compute the aggregated pod status for the Pods pipeline node.
 * For CronJobs, success means all pods succeeded (no failures).
 * For other workloads, uses the Kubernetes Ready condition to determine readiness.
 * @param {Array} pods - Array of pod objects with status field
 * @param {boolean} isCronJob - Whether the parent workload is a CronJob
 * @returns {string} Aggregated status: Ready, Failed, Progressing, or Unknown
 */
function getPodAggregateStatus(pods, isCronJob) {
  if (!pods || pods.length === 0) return 'Unknown'
  const hasAnyFailed = pods.some(p => p.status === 'Failed')
  if (hasAnyFailed) return 'Failed'
  if (isCronJob) {
    const allCompleted = pods.every(p => p.status === 'Succeeded')
    if (allCompleted) return 'Ready'
  } else {
    if (pods.every(p => isPodReady(p))) return 'Ready'
  }
  return 'Progressing'
}

/**
 * Build a human-readable summary of pod phases (e.g. "2 running", "1 running, 1 pending").
 * Groups pods by lowercase phase and joins counts with commas.
 * @param {Array} pods - Array of pod objects with status field
 * @returns {string} Phase summary string
 */
function getPodPhaseSummary(pods) {
  if (!pods || pods.length === 0) return ''
  const counts = {}
  for (const pod of pods) {
    const phase = (pod.status || 'Unknown').toLowerCase()
    counts[phase] = (counts[phase] || 0) + 1
  }
  return Object.entries(counts)
    .map(([phase, count]) => `${count} ${phase}`)
    .join(', ')
}

/**
 * Compact card for a single pipeline node.
 * Matches the NodeCard pattern from GraphTabContent.
 */
function PipelineNode({ kind, name, subtext, status, href }) {
  const borderClass = getStatusBorderClass(status)
  const isClickable = !!href

  const card = (
    <div
      class={`bg-white dark:bg-gray-800 rounded-lg p-3 shadow-sm ${borderClass} transition-all duration-300 ${
        isClickable ? 'cursor-pointer hover:shadow-md' : ''
      }`}
      data-testid="pipeline-node"
    >
      <div class="flex items-center gap-2 mb-1">
        <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">{kind}</span>
      </div>
      <div class="text-sm font-medium text-gray-900 dark:text-white truncate" title={name}>
        {name}
      </div>
      {subtext && (
        <div class="text-xs text-gray-500 dark:text-gray-400 truncate mt-1" title={subtext}>
          {subtext}
        </div>
      )}
    </div>
  )

  if (isClickable) {
    return <a href={href} class="block min-w-0" data-testid="pipeline-link">{card}</a>
  }

  return <div class="min-w-0">{card}</div>
}

/**
 * Get connector color classes for the line and arrow based on the parent node status.
 * @param {string} status - Status of the parent (source) node
 * @returns {{ bg: string, hArrow: string, vArrow: string }}
 */
function getConnectorClasses(status) {
  switch (status) {
  case 'Ready':
  case 'Current':
  case 'Idle':
    return {
      bg: 'bg-green-500 dark:bg-green-400',
      hArrow: 'border-l-green-500 dark:border-l-green-400',
      vArrow: 'border-t-green-500 dark:border-t-green-400'
    }
  case 'Failed':
    return {
      bg: 'bg-red-500 dark:bg-red-400',
      hArrow: 'border-l-red-500 dark:border-l-red-400',
      vArrow: 'border-t-red-500 dark:border-t-red-400'
    }
  case 'Progressing':
  case 'InProgress':
    return {
      bg: 'bg-blue-500 dark:bg-blue-400',
      hArrow: 'border-l-blue-500 dark:border-l-blue-400',
      vArrow: 'border-t-blue-500 dark:border-t-blue-400'
    }
  case 'Suspended':
    return {
      bg: 'bg-yellow-500 dark:bg-yellow-400',
      hArrow: 'border-l-yellow-500 dark:border-l-yellow-400',
      vArrow: 'border-t-yellow-500 dark:border-t-yellow-400'
    }
  default:
    return {
      bg: 'bg-gray-400 dark:bg-gray-500',
      hArrow: 'border-l-gray-400 dark:border-l-gray-500',
      vArrow: 'border-t-gray-400 dark:border-t-gray-500'
    }
  }
}

/**
 * Horizontal connector arrow (desktop): line + right-pointing triangle
 */
function HorizontalConnector({ status }) {
  const c = getConnectorClasses(status)
  return (
    <div class="flex items-center flex-shrink-0" data-testid="pipeline-connector">
      <div class={`h-px w-4 ${c.bg}`} />
      <div class={`w-0 h-0 border-t-[4px] border-b-[4px] border-l-[5px] border-t-transparent border-b-transparent ${c.hArrow}`} />
    </div>
  )
}

/**
 * Vertical connector arrow (mobile): line + down-pointing triangle
 */
function VerticalConnector({ status }) {
  const c = getConnectorClasses(status)
  return (
    <div class="flex flex-col items-center" data-testid="pipeline-connector">
      <div class={`w-px h-6 ${c.bg}`} />
      <div class={`w-0 h-0 border-l-[4px] border-r-[4px] border-t-[5px] border-l-transparent border-r-transparent ${c.vArrow}`} />
    </div>
  )
}

/**
 * WorkloadPipelinePanel - Renders a compact horizontal pipeline:
 * [Source] -> [Reconciler] -> [Workload] -> [Pods]
 *
 * @param {object} reconciler - The enriched parent Flux resource from GetResource API
 * @param {string} kind - The workload kind (Deployment, StatefulSet, etc.)
 * @param {string} name - The workload name
 * @param {string} workloadStatus - The workload status (Current, InProgress, Failed, etc.)
 * @param {Array} pods - Array of pod objects with name and status
 */
export function WorkloadPipelinePanel({ reconciler, kind, name, workloadStatus, pods }) {
  if (!reconciler) return null

  const sourceRef = reconciler.status?.sourceRef
  const reconcilerStatus = reconciler.status?.reconcilerRef?.status || 'Unknown'
  const revision = reconciler.status?.lastAttemptedRevision || reconciler.status?.lastAppliedRevision

  // Build node data and track statuses for connector coloring
  const nodes = []
  const nodeStatuses = []

  // Source node (optional)
  if (sourceRef) {
    const sourceStatus = sourceRef.status || 'Unknown'
    const sourceNs = sourceRef.namespace || reconciler.metadata?.namespace
    nodeStatuses.push(sourceStatus)
    nodes.push(
      <PipelineNode
        kind={sourceRef.kind}
        name={sourceRef.name}
        subtext={sourceRef.url}
        status={sourceStatus}
        href={`/resource/${encodeURIComponent(sourceRef.kind)}/${encodeURIComponent(sourceNs)}/${encodeURIComponent(sourceRef.name)}`}
      />
    )
  }

  // Reconciler node
  nodeStatuses.push(reconcilerStatus)
  nodes.push(
    <PipelineNode
      kind={reconciler.kind}
      name={reconciler.metadata?.name}
      subtext={revision}
      status={reconcilerStatus}
      href={`/resource/${encodeURIComponent(reconciler.kind)}/${encodeURIComponent(reconciler.metadata?.namespace)}/${encodeURIComponent(reconciler.metadata?.name)}`}
    />
  )

  // Workload node (current page, not clickable)
  nodeStatuses.push(workloadStatus)
  nodes.push(
    <PipelineNode
      kind={kind}
      name={name}
      subtext={formatWorkloadStatus(workloadStatus)}
      status={workloadStatus}
    />
  )

  // Pods node — CronJobs care about completion, other workloads care about readiness
  const isCronJob = kind === 'CronJob'
  const podCount = pods?.length || 0
  const podStatus = getPodAggregateStatus(pods, isCronJob)
  let podName, podSubtext
  if (podCount === 0) {
    podName = isCronJob ? 'No active pods' : 'Scaled to zero'
    podSubtext = '0 pods'
  } else if (isCronJob) {
    const completedCount = pods.filter(p => p.status === 'Succeeded' || p.status === 'Failed').length
    podName = `${completedCount}/${podCount} completed`
    podSubtext = getPodPhaseSummary(pods)
  } else {
    const readyCount = pods.filter(p => isPodReady(p)).length
    podName = `${readyCount}/${podCount} ready`
    podSubtext = getPodPhaseSummary(pods)
  }
  nodeStatuses.push(podStatus)
  nodes.push(
    <PipelineNode
      kind="Pods"
      name={podName}
      subtext={podSubtext}
      status={podStatus}
    />
  )

  return (
    <div data-testid="workload-pipeline-panel">
      {/* Desktop: horizontal pipeline */}
      <div class="hidden sm:flex items-center" data-testid="pipeline-horizontal">
        {nodes.map((node, idx) => (
          <div key={idx} class="contents">
            {idx > 0 && <HorizontalConnector status={nodeStatuses[idx - 1]} />}
            <div class="flex-1 min-w-0">{node}</div>
          </div>
        ))}
      </div>

      {/* Mobile: vertical pipeline */}
      <div class="sm:hidden flex flex-col items-center" data-testid="pipeline-vertical">
        {nodes.map((node, idx) => (
          <div key={idx} class="w-full">
            {idx > 0 && <VerticalConnector status={nodeStatuses[idx - 1]} />}
            {node}
          </div>
        ))}
      </div>
    </div>
  )
}
