// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'

/**
 * ActionBar - Action buttons for Flux resources (Reconcile, Reconcile Source, Suspend/Resume)
 *
 * This component is designed to be reusable across ResourcePage, Favorites, and ResourceList views.
 */
export function ActionBar({ kind, namespace, name, resourceData, onActionComplete }) {
  const [loading, setLoading] = useState(null) // tracks which action is loading
  const [error, setError] = useState(null)

  // Extract status information
  const status = resourceData?.status?.reconcilerRef?.status || 'Unknown'
  const actionable = resourceData?.status?.actionable === true
  const sourceRef = resourceData?.status?.sourceRef
  const sourceStatus = sourceRef?.status

  // Determine if resource is suspended
  const isSuspended = status === 'Suspended'

  // Determine if reconciliation is in progress
  const isProgressing = status === 'Progressing'

  // Check if this is a Kustomization or HelmRelease (can reconcile source)
  const canReconcileSource = (kind === 'Kustomization' || kind === 'HelmRelease') && sourceRef

  // Determine button disabled states
  const allButtonsDisabled = !actionable || isProgressing
  const reconcileDisabled = allButtonsDisabled || isSuspended
  const reconcileSourceDisabled = allButtonsDisabled || isSuspended || sourceStatus === 'Suspended'

  // Perform an action
  const performAction = async (action, targetKind, targetNamespace, targetName) => {
    setLoading(action)
    setError(null)

    try {
      await fetchWithMock({
        endpoint: '/api/v1/action',
        mockPath: '../mock/action',
        mockExport: 'mockAction',
        method: 'POST',
        body: {
          kind: targetKind,
          namespace: targetNamespace,
          name: targetName,
          action: action
        }
      })

      // Trigger refetch to get updated status
      if (onActionComplete) {
        onActionComplete()
      }
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(null)
    }
  }

  // Handle reconcile
  const handleReconcile = () => {
    performAction('reconcile', kind, namespace, name)
  }

  // Handle reconcile source
  const handleReconcileSource = () => {
    if (sourceRef) {
      performAction('reconcile', sourceRef.kind, sourceRef.namespace, sourceRef.name)
    }
  }

  // Handle suspend/resume
  const handleSuspendResume = () => {
    const action = isSuspended ? 'resume' : 'suspend'
    performAction(action, kind, namespace, name)
  }

  // Loading spinner
  const LoadingSpinner = () => (
    <svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
  )

  // Don't render if not actionable (no permissions)
  if (!actionable) {
    return null
  }

  return (
    <div class="card p-4" data-testid="action-bar">
      <div class="flex flex-wrap items-center gap-3">
        {/* Reconcile button */}
        <button
          onClick={handleReconcile}
          disabled={reconcileDisabled || loading !== null}
          class={`inline-flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md transition-colors
            ${reconcileDisabled || loading !== null
      ? 'bg-gray-100 text-gray-400 cursor-not-allowed dark:bg-gray-700 dark:text-gray-500'
      : 'bg-flux-blue text-white hover:bg-blue-700 dark:bg-blue-600 dark:hover:bg-blue-700'
    }`}
          data-testid="reconcile-button"
        >
          {loading === 'reconcile' ? <LoadingSpinner /> : (
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          )}
          Reconcile
        </button>

        {/* Reconcile Source button (only for Kustomization/HelmRelease with sourceRef) */}
        {canReconcileSource && (
          <button
            onClick={handleReconcileSource}
            disabled={reconcileSourceDisabled || loading !== null}
            class={`inline-flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md transition-colors
              ${reconcileSourceDisabled || loading !== null
            ? 'bg-gray-100 text-gray-400 cursor-not-allowed dark:bg-gray-700 dark:text-gray-500'
            : 'bg-gray-200 text-gray-700 hover:bg-gray-300 dark:bg-gray-600 dark:text-gray-200 dark:hover:bg-gray-500'
          }`}
            data-testid="reconcile-source-button"
            title={`Reconcile ${sourceRef.kind} ${sourceRef.namespace}/${sourceRef.name}`}
          >
            {loading === 'reconcile' && sourceRef ? <LoadingSpinner /> : (
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            )}
            Reconcile Source
          </button>
        )}

        {/* Suspend/Resume button */}
        <button
          onClick={handleSuspendResume}
          disabled={allButtonsDisabled || loading !== null}
          class={`inline-flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md transition-colors
            ${allButtonsDisabled || loading !== null
      ? 'bg-gray-100 text-gray-400 cursor-not-allowed dark:bg-gray-700 dark:text-gray-500'
      : isSuspended
        ? 'bg-green-600 text-white hover:bg-green-700 dark:bg-green-600 dark:hover:bg-green-700'
        : 'bg-yellow-500 text-white hover:bg-yellow-600 dark:bg-yellow-600 dark:hover:bg-yellow-700'
    }`}
          data-testid="suspend-resume-button"
        >
          {(loading === 'suspend' || loading === 'resume') ? <LoadingSpinner /> : (
            isSuspended ? (
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            ) : (
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            )
          )}
          {isSuspended ? 'Resume' : 'Suspend'}
        </button>

        {/* Progressing indicator */}
        {isProgressing && (
          <span class="inline-flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400">
            <svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Reconciling...
          </span>
        )}
      </div>

      {/* Error message */}
      {error && (
        <div class="mt-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md" data-testid="action-error">
          <p class="text-sm text-red-800 dark:text-red-200">{error}</p>
        </div>
      )}
    </div>
  )
}
