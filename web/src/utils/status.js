// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

/**
 * Status color utilities for consistent badge styling across the application.
 * These functions return Tailwind CSS classes for status badges.
 *
 * Canonical status values are defined in constants.js:
 * - resourceStatuses: Ready, Failed, Progressing, Suspended, Unknown
 * - eventSeverities: Normal, Warning
 */

/**
 * Get badge class for Flux resource statuses.
 * Aligns with resourceStatuses in constants.js.
 * @param {string} status - Status value (Ready, Failed, Progressing, Suspended, Unknown)
 * @returns {string} Tailwind CSS classes for the badge
 */
export function getStatusBadgeClass(status) {
  switch (status) {
  case 'Ready':
    return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
  case 'Failed':
    return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
  case 'Progressing':
    return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
  case 'Suspended':
    return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
  default:
    return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400'
  }
}

/**
 * Get badge class for Kubernetes workload statuses.
 * Accepts workload-level statuses (Current, InProgress, Ready, Progressing) and
 * Kubernetes pod phases (Pending, Running, Succeeded, Failed).
 * @param {string} status - Status value
 * @returns {string} Tailwind CSS classes for the badge
 */
export function getWorkloadStatusBadgeClass(status) {
  switch (status) {
  // Workload-level statuses
  case 'Current':
  case 'Ready':
  case 'Idle':
    return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
  case 'InProgress':
  case 'Progressing':
    return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
  case 'Terminating':
  case 'Suspended':
    return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
  // Kubernetes pod phases
  case 'Pending':
    return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
  case 'Running':
    return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
  case 'Succeeded':
    return 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/30 dark:text-cyan-400'
  case 'Failed':
    return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
  default:
    return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400'
  }
}

/**
 * Format workload status for display.
 * Transforms backend values to user-friendly display values.
 * @param {string} status - Backend status value
 * @returns {string} Display status value
 */
export function formatWorkloadStatus(status) {
  switch (status) {
  case 'Current':
    return 'Ready'
  case 'InProgress':
    return 'Progressing'
  default:
    return status
  }
}

/**
 * Get badge class for reconciliation history statuses.
 * Uses fuzzy matching to handle compound statuses like ReconciliationSucceeded,
 * HealthCheckFailed, as well as Helm release statuses like deployed, superseded.
 * @param {string} status - Status string
 * @returns {string} Tailwind CSS classes for the badge
 */
export function getHistoryStatusBadgeClass(status) {
  const statusLower = status?.toLowerCase() || ''

  if (statusLower.includes('succe') || status === 'deployed') {
    return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
  }
  if (statusLower.includes('failed')) {
    return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
  }
  // Yellow for any other status (superseded, progressing, etc.)
  return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
}

/**
 * Get dot color class for history timeline.
 * Uses same fuzzy matching logic as getHistoryStatusBadgeClass.
 * @param {string} status - Status string
 * @returns {string} Tailwind CSS classes for the dot
 */
export function getHistoryDotClass(status) {
  const statusLower = status?.toLowerCase() || ''

  if (statusLower.includes('succe') || status === 'deployed') {
    return 'bg-green-500 dark:bg-green-400'
  }
  if (statusLower.includes('failed')) {
    return 'bg-red-500 dark:bg-red-400'
  }
  // Yellow for any other status (superseded, progressing, etc.)
  return 'bg-yellow-500 dark:bg-yellow-400'
}

/**
 * Get badge class for Kubernetes event types.
 * Aligns with eventSeverities in constants.js.
 * @param {string} type - Event type (Normal, Warning)
 * @returns {string} Tailwind CSS classes for the badge
 */
export function getEventBadgeClass(type) {
  return type === 'Normal'
    ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    : 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
}

/**
 * Get badge class for Kubernetes container states.
 * Maps container state labels to appropriate badge styling.
 * @param {string} stateLabel - Container state label (Running, Waiting, Terminated, Completed)
 * @returns {string} Tailwind CSS classes for the badge
 */
export function getContainerStateBadgeClass(stateLabel) {
  switch (stateLabel) {
  case 'Running':
    return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
  case 'Waiting':
    return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
  case 'Completed':
    return 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/30 dark:text-cyan-400'
  case 'Terminated':
    return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
  default:
    return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400'
  }
}

/**
 * Get solid background color class for resource status bars/charts.
 * @param {string} status - Status value (Ready, Failed, Progressing, Suspended, Unknown)
 * @returns {string} Tailwind CSS classes for the background
 */
export function getStatusBarColor(status) {
  switch (status) {
  case 'Ready':
    return 'bg-green-500 dark:bg-green-600'
  case 'Failed':
    return 'bg-red-500 dark:bg-red-600'
  case 'Progressing':
    return 'bg-blue-500 dark:bg-blue-600'
  case 'Suspended':
    return 'bg-yellow-500 dark:bg-yellow-600'
  case 'Unknown':
    return 'bg-gray-600 dark:bg-gray-500'
  default:
    return 'bg-gray-200 dark:bg-gray-700'
  }
}

/**
 * Get solid background color class for event type bars/charts.
 * @param {string} type - Event type (Normal, Warning)
 * @returns {string} Tailwind CSS classes for the background
 */
export function getEventBarColor(type) {
  switch (type) {
  case 'Normal':
    return 'bg-green-500 dark:bg-green-600'
  case 'Warning':
    return 'bg-red-500 dark:bg-red-600'
  default:
    return 'bg-gray-200 dark:bg-gray-700'
  }
}

/**
 * Normalize a status value to the Flux vocabulary (Ready/Failed/Progressing/
 * Suspended/Unknown) used by status charts, filters, and borders. Kubernetes workload
 * kstatus values are mapped to their Flux equivalents (Current/Idle -> Ready,
 * InProgress -> Progressing, Terminating -> Suspended); Flux statuses and any
 * unrecognized value pass through unchanged. Keep this in sync with
 * getWorkloadStatusBadgeClass so badges, borders, and charts stay consistent.
 * @param {string} status - Status value
 * @returns {string} Status value in the Flux vocabulary
 */
export function normalizeToFluxStatus(status) {
  switch (status) {
  case 'Current':
  case 'Idle':
    return 'Ready'
  case 'InProgress':
    return 'Progressing'
  case 'Terminating':
    return 'Suspended'
  default:
    return status
  }
}

/**
 * Get dot color class for a Flux resource status.
 * Aligns with resourceStatuses in constants.js; unknown/empty falls back to gray.
 * @param {string} status - Status value (Ready, Failed, Progressing, Suspended, Unknown)
 * @returns {string} Tailwind background color class for a status dot
 */
export function getStatusDotClass(status) {
  switch (status) {
  case 'Ready':
    return 'bg-green-500'
  case 'Failed':
    return 'bg-red-500'
  case 'Progressing':
    return 'bg-blue-500'
  case 'Suspended':
    return 'bg-yellow-500'
  default:
    return 'bg-gray-500'
  }
}

/**
 * Get border color class for resource status.
 * Uses semantic color classes (border-success, border-danger, etc.) for consistency.
 * Accepts both Flux resource statuses and Kubernetes workload kstatus values
 * (normalized via normalizeToFluxStatus), so workload favorites render a border
 * consistent with their status badge.
 * @param {string} status - Status value
 * @returns {string} Tailwind CSS classes for the border
 */
export function getStatusBorderClass(status) {
  switch (normalizeToFluxStatus(status)) {
  case 'Ready':
    return 'border-success'
  case 'Failed':
    return 'border-danger'
  case 'Suspended':
    return 'border-warning'
  case 'Progressing':
    return 'border-info'
  default:
    return 'border-gray-300 dark:border-gray-600'
  }
}

/**
 * Clean status object for display by removing internal fields.
 * Removes fields that are displayed separately or are internal to the UI.
 * @param {object} status - The status object from a Flux resource
 * @returns {object|undefined} Cleaned status object or undefined if input is falsy
 */
export function cleanStatus(status) {
  if (!status) return undefined
  // eslint-disable-next-line no-unused-vars
  const { exportedInputs, helmValues, helmValuesError, inputProviderRefs, inventory, inventoryError, reconcilerRef, sourceRef, userActions, ...clean } = status
  return clean
}

/**
 * Derive the display status of a Flux controller component reported by the
 * cluster status page. The backend reports `ready` (kstatus Current) and a
 * `status` string prefixed with the kstatus status word, so a Deployment that
 * is still rolling out (e.g. at bootstrap or during an upgrade) reads as
 * `InProgress ...` and is Progressing rather than Failed.
 * @param {{ready?: boolean, status?: string}} component - Component status
 * @returns {'Ready'|'Progressing'|'Failed'} Display status
 */
export function getComponentStatus(component) {
  if (component?.ready) return 'Ready'
  if (component?.status?.startsWith('InProgress')) return 'Progressing'
  return 'Failed'
}

/**
 * Derive the display status of the cluster sync reported by the cluster status
 * page. A sync whose Kustomization has not reported a Ready condition yet is
 * still initializing (e.g. at bootstrap), so it is Progressing rather than
 * Failed. The check is exact: when the source is failing its error is appended
 * to the status and the sync is Failed.
 * @param {{ready?: boolean, status?: string}} sync - Cluster sync status
 * @returns {'Suspended'|'Synced'|'Progressing'|'Failed'} Display status
 */
export function getSyncStatus(sync) {
  if (sync?.status?.startsWith('Suspended')) return 'Suspended'
  if (sync?.ready) return 'Synced'
  if (sync?.status === 'not initialized') return 'Progressing'
  return 'Failed'
}
