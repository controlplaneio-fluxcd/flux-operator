// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'

/**
 * ActionBar - Action buttons for Flux resources (Reconcile, Reconcile Source, Suspend/Resume)
 *
 * This component is designed to be reusable across ResourcePage, Favorites, and ResourceList views.
 */
export function ActionBar({ kind, namespace, name, resourceData, onActionComplete }) {
  const [loading, setLoading] = useState(null) // tracks which action is loading
  const [error, setError] = useState(null)
  const [showSuccess, setShowSuccess] = useState(null) // tracks which button shows success checkmark

  // Track previous status values to detect transitions
  const prevStatusRef = useRef(null)
  const prevSourceStatusRef = useRef(null)

  // Auto-dismiss error after 5 seconds
  useEffect(() => {
    if (error) {
      const timer = window.setTimeout(() => setError(null), 5000)
      return () => window.clearTimeout(timer)
    }
  }, [error])

  // Extract status information
  const status = resourceData?.status?.reconcilerRef?.status || 'Unknown'
  const actionable = resourceData?.status?.actionable === true
  const sourceRef = resourceData?.status?.sourceRef
  const sourceStatus = sourceRef?.status

  // Determine if resource is suspended
  const isSuspended = status === 'Suspended'

  // Determine if reconciliation is in progress
  const isProgressing = status === 'Progressing'
  const isSourceProgressing = sourceStatus === 'Progressing'

  // Show success checkmark when reconciliation completes
  useEffect(() => {
    if (prevStatusRef.current === 'Progressing' && status !== 'Progressing') {
      setShowSuccess('reconcile')
      const timer = window.setTimeout(() => setShowSuccess(null), 2000)
      return () => window.clearTimeout(timer)
    }
    prevStatusRef.current = status
  }, [status])

  // Show success checkmark when source fetch completes
  useEffect(() => {
    if (prevSourceStatusRef.current === 'Progressing' && sourceStatus !== 'Progressing') {
      setShowSuccess('reconcile-source')
      const timer = window.setTimeout(() => setShowSuccess(null), 2000)
      return () => window.clearTimeout(timer)
    }
    prevSourceStatusRef.current = sourceStatus
  }, [sourceStatus])

  // Check if this is a Kustomization or HelmRelease (can reconcile source)
  const canReconcileSource = (kind === 'Kustomization' || kind === 'HelmRelease') && sourceRef

  // Determine button disabled states
  const allButtonsDisabled = !actionable || isProgressing
  const reconcileDisabled = allButtonsDisabled || isSuspended
  const reconcileSourceDisabled = allButtonsDisabled || isSuspended || sourceStatus === 'Suspended'

  // Perform an action
  const performAction = async (action, targetKind, targetNamespace, targetName, loadingId = action) => {
    setLoading(loadingId)
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

      // Trigger refetch to get updated status and wait for it
      if (onActionComplete) {
        await onActionComplete()
      }

      // Show success checkmark for reconcile actions
      if (loadingId === 'reconcile' || loadingId === 'reconcile-source') {
        setShowSuccess(loadingId)
        window.setTimeout(() => setShowSuccess(null), 2000)
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
      performAction('reconcile', sourceRef.kind, sourceRef.namespace, sourceRef.name, 'reconcile-source')
    }
  }

  // Handle suspend/resume
  const handleSuspendResume = () => {
    const action = isSuspended ? 'resume' : 'suspend'
    performAction(action, kind, namespace, name)
  }

  // Loading spinner
  const LoadingSpinner = () => (
    <svg class="animate-spin h-3.5 w-3.5" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
  )

  // Success checkmark (inherits button color)
  const SuccessCheck = () => (
    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
    </svg>
  )

  // Don't render if not actionable (no permissions)
  if (!actionable) {
    return null
  }

  // Base button styles
  const baseButtonClass = 'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded border transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 dark:focus:ring-offset-gray-900'
  const disabledClass = 'border-gray-300 text-gray-400 cursor-not-allowed dark:border-gray-600 dark:text-gray-500'

  return (
    <div class="flex flex-wrap items-center gap-2" data-testid="action-bar">
      {/* Reconcile button */}
      <button
        onClick={handleReconcile}
        disabled={reconcileDisabled || loading !== null}
        class={`${baseButtonClass} ${
          reconcileDisabled || loading !== null
            ? disabledClass
            : 'border-blue-500 text-blue-600 hover:bg-blue-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30 focus:ring-blue-500'
        }`}
        data-testid="reconcile-button"
        title="Trigger a reconciliation"
      >
        {(loading === 'reconcile' || isProgressing) ? <LoadingSpinner /> : showSuccess === 'reconcile' ? <SuccessCheck /> : (
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
          class={`${baseButtonClass} ${
            reconcileSourceDisabled || loading !== null
              ? disabledClass
              : 'border-purple-500 text-purple-600 hover:bg-purple-50 dark:border-purple-400 dark:text-purple-400 dark:hover:bg-purple-900/30 focus:ring-purple-500'
          }`}
          data-testid="reconcile-source-button"
          title="Fetch changes from source"
        >
          {(loading === 'reconcile-source' || isSourceProgressing) ? <LoadingSpinner /> : showSuccess === 'reconcile-source' ? <SuccessCheck /> : (
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          )}
          Fetch
        </button>
      )}

      {/* Suspend/Resume button */}
      <button
        onClick={handleSuspendResume}
        disabled={allButtonsDisabled || loading !== null}
        class={`${baseButtonClass} ${
          allButtonsDisabled || loading !== null
            ? disabledClass
            : isSuspended
              ? 'border-green-500 text-green-600 hover:bg-green-50 dark:border-green-400 dark:text-green-400 dark:hover:bg-green-900/30 focus:ring-green-500'
              : 'border-amber-500 text-amber-600 hover:bg-amber-50 dark:border-amber-400 dark:text-amber-400 dark:hover:bg-amber-900/30 focus:ring-amber-500'
        }`}
        data-testid="suspend-resume-button"
        title={isSuspended ? 'Resume reconciliation' : 'Suspend reconciliation'}
      >
        {(loading === 'suspend' || loading === 'resume') ? <LoadingSpinner /> : (
          isSuspended ? (
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          ) : (
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          )
        )}
        {isSuspended ? 'Resume' : 'Suspend'}
      </button>

      {/* Error message */}
      {error && (
        <div class="w-full mt-2 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-xs text-red-800 dark:text-red-200 flex items-center justify-between gap-2" data-testid="action-error">
          <span>{error}</span>
          <button
            onClick={() => setError(null)}
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
