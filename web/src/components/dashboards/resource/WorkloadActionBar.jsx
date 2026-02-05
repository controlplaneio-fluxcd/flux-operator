// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useAction } from '../../../utils/useAction'

// Threshold for considering a restart as "recent" (30 seconds)
const RECENT_RESTART_THRESHOLD = 30000

/**
 * Check if a restart timestamp is within the recent threshold
 * @param {string} restartedAt - ISO timestamp string
 * @returns {boolean} true if the timestamp is within the last 30 seconds
 */
function isRecentRestart(restartedAt) {
  if (!restartedAt) return false
  const restartTime = new Date(restartedAt).getTime()
  if (isNaN(restartTime)) return false
  const now = Date.now()
  return (now - restartTime) < RECENT_RESTART_THRESHOLD
}

/**
 * WorkloadActionBar - Action buttons for Kubernetes workloads (Deployment, StatefulSet, DaemonSet)
 *
 * Supports:
 * - Rollout Restart for Deployment, StatefulSet, DaemonSet
 *
 * @param {Object} props
 * @param {string} props.kind - Workload kind (Deployment, StatefulSet, DaemonSet, CronJob)
 * @param {string} props.namespace - Workload namespace
 * @param {string} props.name - Workload name
 * @param {string} props.status - Workload status (e.g., "Current", "InProgress", "Failed")
 * @param {string} props.restartedAt - Timestamp of last restart (RFC3339 format)
 * @param {Array} props.userActions - Array of allowed user actions
 * @param {Function} props.onActionStart - Callback when action starts (for faster polling)
 * @param {Function} props.onActionComplete - Callback to refetch workload data after action
 */
export function WorkloadActionBar({ kind, namespace, name, status, restartedAt, userActions = [], onActionStart, onActionComplete }) {
  const { loading, error, showSuccess, performAction, clearError } = useAction({
    onActionStart,
    onActionComplete
  })

  // Check if a recent restart is still in progress or just completed
  const recentRestart = isRecentRestart(restartedAt)
  const isRestartInProgress = recentRestart && status === 'InProgress'
  const isRestartCompleted = recentRestart && status === 'Current'

  // Check if restart action is allowed
  const canRestart = userActions.includes('restart')

  // Only Deployment, StatefulSet, DaemonSet support restart
  const supportsRestart = kind === 'Deployment' || kind === 'StatefulSet' || kind === 'DaemonSet'

  // Handle restart action
  const handleRestart = () => {
    performAction({
      endpoint: '/api/v1/workload/action',
      body: {
        kind,
        namespace,
        name,
        action: 'restart'
      },
      loadingId: 'restart',
      mockPath: '../mock/action',
      mockExport: 'mockWorkloadAction',
      showSuccessCheck: true
    })
  }

  // Loading spinner
  const LoadingSpinner = () => (
    <svg class="animate-spin h-3.5 w-3.5" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
  )

  // Success checkmark
  const SuccessCheck = () => (
    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
    </svg>
  )

  // If no actions available, don't render
  if (!supportsRestart || !canRestart) {
    return null
  }

  // Base button styles
  const baseButtonClass = 'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded border transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 dark:focus:ring-offset-gray-900'
  const disabledClass = 'border-gray-300 text-gray-400 cursor-not-allowed dark:border-gray-600 dark:text-gray-500'

  return (
    <div class="flex flex-wrap items-center gap-2" data-testid="workload-action-bar">
      {/* Restart button */}
      <button
        onClick={handleRestart}
        disabled={loading !== null || isRestartInProgress}
        class={`${baseButtonClass} ${
          loading !== null || isRestartInProgress
            ? disabledClass
            : 'border-blue-500 text-blue-600 hover:bg-blue-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30 focus:ring-blue-500'
        }`}
        data-testid="restart-button"
        title="Restart workload by triggering a rollout"
      >
        {loading === 'restart' || isRestartInProgress ? <LoadingSpinner /> : showSuccess === 'restart' || isRestartCompleted ? <SuccessCheck /> : (
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        )}
        Rollout Restart
      </button>

      {/* Error message */}
      {error && (
        <div class="w-full mt-2 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-xs text-red-800 dark:text-red-200 flex items-center justify-between gap-2" data-testid="workload-action-error">
          <span>{error}</span>
          <button
            onClick={clearError}
            class="text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-200 p-0.5"
            aria-label="Dismiss error"
            data-testid="dismiss-error-button"
          >
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      )}
    </div>
  )
}
