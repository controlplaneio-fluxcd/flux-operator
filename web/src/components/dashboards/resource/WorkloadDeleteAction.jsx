// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useEffect } from 'preact/hooks'
import { useAction } from '../../../utils/useAction'

/**
 * WorkloadDeleteAction - Delete button for individual pods
 *
 * @param {Object} props
 * @param {string} props.namespace - Pod namespace
 * @param {string} props.name - Pod name
 * @param {boolean} props.isPendingDeletion - Whether this pod is awaiting deletion (spinner persists until pod disappears)
 * @param {Function} props.onActionStart - Callback when action starts (for faster polling)
 * @param {Function} props.onActionComplete - Callback to refetch workload data after action
 * @param {Function} props.onPodDeleteStart - Callback to mark pod as pending deletion
 * @param {Function} props.onPodDeleteFailed - Callback to unmark pod on error
 */
export function WorkloadDeleteAction({ namespace, name, isPendingDeletion, onActionStart, onActionComplete, onPodDeleteStart, onPodDeleteFailed }) {
  const { error, performAction, clearError } = useAction({
    onActionStart,
    onActionComplete
  })

  // When an error occurs, remove the pod from pending deletions
  // so the spinner stops and the error message is visible.
  useEffect(() => {
    if (error && onPodDeleteFailed) {
      onPodDeleteFailed(name)
    }
  }, [error, name, onPodDeleteFailed])

  const isDeleting = isPendingDeletion && !error

  const handleDelete = (e) => {
    e.stopPropagation()
    if (!window.confirm(`Are you sure you want to delete the pod ${namespace}/${name}?`)) {
      return
    }
    if (onPodDeleteStart) {
      onPodDeleteStart(name)
    }
    performAction({
      endpoint: '/api/v1/workload/action',
      body: {
        kind: 'Pod',
        namespace,
        name,
        action: 'delete'
      },
      loadingId: 'delete',
      mockPath: '../mock/action',
      mockExport: 'mockWorkloadAction'
    })
  }

  return (
    <>
      <button
        onClick={handleDelete}
        disabled={isDeleting}
        class={`inline-flex items-center p-1 rounded transition-colors focus:outline-none ${
          isDeleting
            ? 'text-gray-300 cursor-not-allowed dark:text-gray-600'
            : 'text-gray-400 hover:text-red-500 dark:text-gray-500 dark:hover:text-red-400'
        }`}
        data-testid="delete-pod-button"
        title={`Delete pod ${name}`}
      >
        {isDeleting ? (
          <svg class="animate-spin h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" data-testid="delete-pod-spinner">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
        ) : (
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        )}
      </button>
      {error && (
        <div class="w-full mt-1 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-xs text-red-800 dark:text-red-200 flex items-center justify-between gap-2" data-testid="delete-pod-error">
          <span>{error}</span>
          <button
            onClick={(e) => { e.stopPropagation(); clearError() }}
            class="text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-200 p-0.5"
            aria-label="Dismiss error"
          >
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      )}
    </>
  )
}
