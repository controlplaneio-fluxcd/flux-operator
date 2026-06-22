// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Shared helpers for summarizing a workload's pods. Used by the workload
// pipeline panel and the inline workload details panel so both report pod
// readiness and phase breakdowns identically.

/**
 * Check if a pod is truly ready using the Kubernetes Ready condition.
 * Falls back to checking the pod phase when podStatus is not available.
 * @param {object} pod - Pod object with status and optional podStatus
 * @returns {boolean} true if the pod is ready
 */
export function isPodReady(pod) {
  // Use the Ready condition from podStatus when available (detail endpoint)
  if (pod.podStatus?.conditions) {
    return pod.podStatus.conditions.some(c => c.type === 'Ready' && c.status === 'True')
  }
  // Fallback: treat Running/Succeeded as ready when podStatus is not available
  return pod.status === 'Running' || pod.status === 'Succeeded'
}

/**
 * Compute the aggregated pod status.
 * For CronJobs, success means all pods succeeded (no failures).
 * For other workloads, uses the Kubernetes Ready condition to determine readiness.
 * @param {Array} pods - Array of pod objects with status field
 * @param {boolean} isCronJob - Whether the parent workload is a CronJob
 * @returns {string} Aggregated status: Ready, Failed, Progressing, or Unknown
 */
export function getPodAggregateStatus(pods, isCronJob) {
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
export function getPodPhaseSummary(pods) {
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
 * Summarize a workload's pods into a status plus display strings.
 * @param {Array} pods - Array of pod objects with name and status
 * @param {string} kind - The workload kind (Deployment, CronJob, etc.)
 * @returns {{ status: string, total: number, primary: string, detail: string }}
 *   status: aggregate status for coloring; total: pod count;
 *   primary: e.g. "2/3 ready" or "1/2 completed"; detail: phase breakdown.
 */
export function summarizePods(pods, kind) {
  const isCronJob = kind === 'CronJob'
  const total = pods?.length || 0
  const status = getPodAggregateStatus(pods, isCronJob)

  if (total === 0) {
    return {
      status,
      total,
      primary: isCronJob ? 'No active pods' : 'Scaled to zero',
      detail: '0 pods'
    }
  }

  if (isCronJob) {
    const completed = pods.filter(p => p.status === 'Succeeded' || p.status === 'Failed').length
    return { status, total, primary: `${completed}/${total} completed`, detail: getPodPhaseSummary(pods) }
  }

  const ready = pods.filter(p => isPodReady(p)).length
  return { status, total, primary: `${ready}/${total} ready`, detail: getPodPhaseSummary(pods) }
}
