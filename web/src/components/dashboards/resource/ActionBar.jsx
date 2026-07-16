// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useRef } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { downloadBlob } from '../../../utils/download'
import { ActionButton } from '../../common/ActionButton'
import {
  getActionTooltip,
  isActionBlockedByAccess,
  isUserActionsEnabled
} from '../../../utils/userActions'

/**
 * ActionBar - Action buttons for Flux resources (Reconcile, Reconcile Source, Suspend/Resume)
 *
 * This component is designed to be reusable across ResourcePage, Favorites, and ResourceList views.
 */
export function ActionBar({ kind, namespace, name, resourceData, onActionComplete, onActionStart }) {
  const [loading, setLoading] = useState(null) // tracks which action is loading
  const [error, setError] = useState(null)
  const [showSuccess, setShowSuccess] = useState(null) // tracks which button shows success checkmark
  const [dropdownOpen, setDropdownOpen] = useState(false)

  // Track previous status values to detect transitions
  const prevStatusRef = useRef(null)
  const prevSourceStatusRef = useRef(null)
  const dropdownRef = useRef(null)

  // Auto-dismiss error after 5 seconds
  useEffect(() => {
    if (error) {
      const timer = window.setTimeout(() => setError(null), 5000)
      return () => window.clearTimeout(timer)
    }
  }, [error])

  // Extract status information
  const status = resourceData?.status?.reconcilerRef?.status || 'Unknown'
  const userActions = resourceData?.status?.userActions || []
  const userActionsEnabled = isUserActionsEnabled(resourceData)
  const sourceRef = resourceData?.status?.sourceRef
  const sourceStatus = sourceRef?.status

  // Check which actions are allowed based on userActions array
  const canDoReconcile = userActions.includes('reconcile')
  const canDoSuspend = userActions.includes('suspend')
  const canDoResume = userActions.includes('resume')
  const canDoDownload = userActions.includes('download')

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

  // Close dropdown when clicking outside
  useEffect(() => {
    if (!dropdownOpen) return

    const handleClickOutside = (event) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setDropdownOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [dropdownOpen])

  // Close dropdown on Escape key
  useEffect(() => {
    if (!dropdownOpen) return

    const handleEscape = (event) => {
      if (event.key === 'Escape') {
        setDropdownOpen(false)
      }
    }

    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [dropdownOpen])

  // Check if this kind supports reconciliation (Alert and Provider only support suspend/resume)
  const canReconcile = kind !== 'Alert' && kind !== 'Provider'

  // Check if this kind supports suspend/resume (ExternalArtifact doesn't)
  const canSuspendResume = kind !== 'ExternalArtifact'

  // Check if this is a Kustomization or HelmRelease with a pullable source (not ExternalArtifact)
  const canReconcileSource = (kind === 'Kustomization' || kind === 'HelmRelease') && sourceRef && sourceRef.kind !== 'ExternalArtifact'

  // Source kinds that have downloadable artifacts
  const downloadableKinds = ['Bucket', 'GitRepository', 'OCIRepository', 'HelmChart', 'ExternalArtifact']

  // Check if this is a source kind with a downloadable artifact
  const hasArtifact = resourceData?.status?.artifact?.url
  const showDownload = downloadableKinds.includes(kind) && hasArtifact

  // Check if this is an ArtifactGenerator with ExternalArtifacts in inventory
  // Inventory items have: name, namespace, digest, filename
  const isArtifactGenerator = kind === 'ArtifactGenerator'
  const inventoryArtifacts = isArtifactGenerator
    ? (resourceData?.status?.inventory || [])
    : []
  const showDownloadArtifacts = isArtifactGenerator && inventoryArtifacts.length > 0

  // Base button styles
  const baseButtonClass = 'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded border transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 dark:focus:ring-offset-gray-900'
  const disabledClass = 'border-gray-300 text-gray-400 cursor-not-allowed dark:border-gray-600 dark:text-gray-500'

  const getButtonPresentation = ({
    hasPermission,
    stateDisabled,
    stateReason,
    actionLabel,
    enabledTitle,
    activeClass
  }) => {
    const accessBlocked = isActionBlockedByAccess(userActionsEnabled, hasPermission)
    const disabled = accessBlocked || stateDisabled || loading !== null
    const title = getActionTooltip({
      userActionsEnabled,
      hasPermission,
      actionLabel,
      stateReason: !accessBlocked && stateDisabled ? stateReason : undefined,
      enabledTitle
    })

    return {
      disabled,
      title,
      class: `${baseButtonClass} ${disabled ? disabledClass : activeClass}`
    }
  }

  const reconcilePresentation = getButtonPresentation({
    hasPermission: canDoReconcile,
    stateDisabled: isProgressing || isSuspended,
    stateReason: isProgressing ? 'Reconciliation in progress' : 'Resource is suspended',
    actionLabel: 'reconcile',
    enabledTitle: 'Trigger a reconciliation',
    activeClass: 'border-blue-500 text-blue-600 hover:bg-blue-50 dark:border-blue-400 dark:text-blue-400 dark:hover:bg-blue-900/30 focus:ring-blue-500'
  })

  const reconcileSourcePresentation = getButtonPresentation({
    hasPermission: canDoReconcile,
    stateDisabled: isSuspended || sourceStatus === 'Suspended',
    stateReason: isSuspended ? 'Resource is suspended' : 'Source is suspended',
    actionLabel: 'pull changes from source',
    enabledTitle: 'Pull changes from upstream source',
    activeClass: 'border-purple-500 text-purple-600 hover:bg-purple-50 dark:border-purple-400 dark:text-purple-400 dark:hover:bg-purple-900/30 focus:ring-purple-500'
  })

  const suspendResumePresentation = getButtonPresentation({
    hasPermission: isSuspended ? canDoResume : canDoSuspend,
    stateDisabled: false,
    actionLabel: isSuspended ? 'resume reconciliation' : 'suspend reconciliation',
    enabledTitle: isSuspended ? 'Resume reconciliation' : 'Suspend reconciliation',
    activeClass: isSuspended
      ? 'border-green-500 text-green-600 hover:bg-green-50 dark:border-green-400 dark:text-green-400 dark:hover:bg-green-900/30 focus:ring-green-500'
      : 'border-amber-500 text-amber-600 hover:bg-amber-50 dark:border-amber-400 dark:text-amber-400 dark:hover:bg-amber-900/30 focus:ring-amber-500'
  })

  const downloadPresentation = getButtonPresentation({
    hasPermission: canDoDownload,
    stateDisabled: false,
    actionLabel: 'download artifacts',
    enabledTitle: 'Download artifact',
    activeClass: 'border-purple-500 text-purple-600 hover:bg-purple-50 dark:border-purple-400 dark:text-purple-400 dark:hover:bg-purple-900/30 focus:ring-purple-500'
  })

  const downloadArtifactsPresentation = getButtonPresentation({
    hasPermission: canDoDownload,
    stateDisabled: false,
    actionLabel: 'download artifacts',
    enabledTitle: 'Download artifacts',
    activeClass: dropdownOpen
      ? 'border-purple-500 text-purple-600 bg-purple-50 dark:border-purple-400 dark:text-purple-400 dark:bg-purple-900/30 ring-0'
      : 'border-purple-500 text-purple-600 hover:bg-purple-50 dark:border-purple-400 dark:text-purple-400 dark:hover:bg-purple-900/30 focus:ring-purple-500'
  })

  const hasAnyAction = canReconcile || canSuspendResume || canReconcileSource || showDownload || showDownloadArtifacts
  if (!hasAnyAction) {
    return null
  }

  // Perform an action
  const performAction = async (action, targetKind, targetNamespace, targetName, loadingId = action) => {
    // Notify parent that action is starting (for faster polling)
    if (onActionStart) {
      onActionStart()
    }
    setLoading(loadingId)
    setError(null)

    try {
      await fetchWithMock({
        endpoint: '/api/v1/resource/action',
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

  // Handle download - use fetch/blob approach for better error handling and Tailscale compatibility
  const handleDownload = async () => {
    const url = `/api/v1/artifact/download?kind=${encodeURIComponent(kind)}&namespace=${encodeURIComponent(namespace)}&name=${encodeURIComponent(name)}`

    setLoading('download')
    setError(null)

    try {
      const response = await fetch(url)

      if (!response.ok) {
        const errorText = await response.text()
        throw new Error(errorText || `Server responded with ${response.status}`)
      }

      const blob = await response.blob()
      downloadBlob(blob, `${kind}-${namespace}-${name}.tar.gz`)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(null)
    }
  }

  // Handle download for ArtifactGenerator inventory items (ExternalArtifacts)
  const handleDownloadArtifact = async (artifact) => {
    const url = `/api/v1/artifact/download?kind=ExternalArtifact&namespace=${encodeURIComponent(artifact.namespace)}&name=${encodeURIComponent(artifact.name)}`

    setLoading('download')
    setDropdownOpen(false)
    setError(null)

    try {
      const response = await fetch(url)

      if (!response.ok) {
        const errorText = await response.text()
        throw new Error(errorText || `Server responded with ${response.status}`)
      }

      const blob = await response.blob()
      downloadBlob(blob, artifact.filename || `${artifact.name}.tar.gz`)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(null)
    }
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

  return (
    <div class="flex flex-wrap items-center gap-2" data-testid="action-bar">
      {/* Reconcile button (hidden for Alert and Provider) */}
      {canReconcile && (
        <ActionButton
          onClick={handleReconcile}
          disabled={reconcilePresentation.disabled}
          class={reconcilePresentation.class}
          data-testid="reconcile-button"
          title={reconcilePresentation.title}
        >
          {(loading === 'reconcile' || isProgressing) ? <LoadingSpinner /> : showSuccess === 'reconcile' ? <SuccessCheck /> : (
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          )}
          Reconcile
        </ActionButton>
      )}

      {/* Download button (only for source kinds with artifacts) */}
      {showDownload && (
        <ActionButton
          onClick={handleDownload}
          disabled={downloadPresentation.disabled}
          class={downloadPresentation.class}
          data-testid="download-button"
          title={downloadPresentation.title}
        >
          {loading === 'download' ? <LoadingSpinner /> : (
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          )}
          Download
        </ActionButton>
      )}

      {/* Download dropdown for ArtifactGenerator */}
      {showDownloadArtifacts && (
        <div class="relative" ref={dropdownRef}>
          <ActionButton
            onClick={() => {
              if (!downloadArtifactsPresentation.disabled) {
                setDropdownOpen(!dropdownOpen)
              }
            }}
            disabled={downloadArtifactsPresentation.disabled}
            class={downloadArtifactsPresentation.class}
            data-testid="download-dropdown-button"
            title={downloadArtifactsPresentation.title}
          >
            {loading === 'download' ? <LoadingSpinner /> : (
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
            )}
            Download
            <svg class="w-3 h-3 ml-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
          </ActionButton>

          {dropdownOpen && !downloadArtifactsPresentation.disabled && (
            <div
              class="absolute left-0 mt-1 w-64 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50"
              data-testid="download-dropdown-menu"
            >
              {inventoryArtifacts.map((artifact) => (
                <button
                  key={`${artifact.namespace}/${artifact.name}`}
                  onClick={() => handleDownloadArtifact(artifact)}
                  class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                  data-testid={`download-artifact-${artifact.name}`}
                >
                  <div class="font-medium truncate text-gray-900 dark:text-gray-100">{artifact.name}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400 truncate">{artifact.namespace}</div>
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Reconcile Source button (only for Kustomization/HelmRelease with sourceRef) */}
      {canReconcileSource && (
        <ActionButton
          onClick={handleReconcileSource}
          disabled={reconcileSourcePresentation.disabled}
          class={reconcileSourcePresentation.class}
          data-testid="reconcile-source-button"
          title={reconcileSourcePresentation.title}
        >
          {(loading === 'reconcile-source' || isSourceProgressing) ? <LoadingSpinner /> : showSuccess === 'reconcile-source' ? <SuccessCheck /> : (
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          )}
          Pull
        </ActionButton>
      )}

      {/* Suspend/Resume button (hidden for ExternalArtifact) */}
      {canSuspendResume && (
        <ActionButton
          onClick={handleSuspendResume}
          disabled={suspendResumePresentation.disabled}
          class={suspendResumePresentation.class}
          data-testid="suspend-resume-button"
          title={suspendResumePresentation.title}
        >
          {(loading === 'suspend' || loading === 'resume') ? <LoadingSpinner /> : (
            isSuspended ? (
              <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
                <path d="M8 5v14l11-7z" />
              </svg>
            ) : (
              <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
                <path d="M6 4h4v16H6V4zm8 0h4v16h-4V4z" />
              </svg>
            )
          )}
          {isSuspended ? 'Resume' : 'Suspend'}
        </ActionButton>
      )}

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
