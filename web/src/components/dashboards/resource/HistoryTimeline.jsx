// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { formatTimestamp } from '../../../utils/time'
import { getHistoryStatusBadgeClass, getHistoryDotClass } from '../../../utils/status'

/**
 * Format status for display
 * @param {string} status - The raw status string
 * @returns {string} Formatted status string
 */
const formatStatus = (status) => {
  // Uppercase first letter
  return status.charAt(0).toUpperCase() + status.slice(1)
}

/**
 * Truncate digest to show only first 12 characters
 * @param {string} digest - The digest string
 * @returns {string} Truncated digest
 */
const truncateDigest = (digest) => {
  if (!digest) return ''
  const hash = digest.split(':')[1] || digest
  return hash.substring(0, 12)
}

/**
 * Get kind-specific metadata to display
 * @param {string} kind - Resource kind
 * @param {object} entry - History entry
 * @returns {object} Object with label and value
 */
const getKindSpecificMetadata = (kind, entry) => {
  // HelmRelease uses different field names
  if (kind === 'HelmRelease') {
    return {
      label: 'Chart Version',
      value: entry.chartVersion || '-'
    }
  }

  // For other kinds, check metadata
  const metadata = entry.metadata || {}

  if (kind === 'FluxInstance' && metadata.flux) {
    return {
      label: 'Flux',
      value: metadata.flux
    }
  }

  if (kind === 'ResourceSet' && metadata.resources) {
    return {
      label: 'Resources',
      value: metadata.resources
    }
  }

  if (kind === 'Kustomization' && metadata.revision) {
    return {
      label: 'Revision',
      value: metadata.revision
    }
  }

  return null
}

/**
 * HistoryTimeline component displays reconciliation history as a vertical timeline
 * @param {object} props - Component props
 * @param {Array} props.history - History entries from status.history
 * @param {string} props.kind - Resource kind (FluxInstance, ResourceSet, Kustomization, HelmRelease)
 */
export function HistoryTimeline({ history, kind }) {
  if (!history || history.length === 0) {
    return (
      <div class="text-sm text-gray-500 dark:text-gray-400">
        No history available
      </div>
    )
  }

  // Determine if this is HelmRelease history (uses different field names)
  const isHelmRelease = kind === 'HelmRelease'

  return (
    <div class="space-y-0">
      {history.map((entry, idx) => {
        // Extract fields based on resource type
        const firstTime = isHelmRelease ? entry.firstDeployed : entry.firstReconciled
        const lastTime = isHelmRelease ? entry.lastDeployed : entry.lastReconciled
        const status = isHelmRelease ? entry.status : entry.lastReconciledStatus
        const duration = entry.lastReconciledDuration
        const count = isHelmRelease ? entry.version : entry.totalReconciliations
        const digest = entry.digest

        const kindMetadata = getKindSpecificMetadata(kind, entry)
        const isLast = idx === history.length - 1

        return (
          <div key={idx} class="relative">
            {/* First row: dot + status badge aligned */}
            <div class="flex items-center gap-4 mb-2">
              <div class="relative flex flex-col items-center">
                {/* Dot */}
                <div class={`w-3 h-3 rounded-full flex-shrink-0 ${getHistoryDotClass(status)} ring-4 ring-white dark:ring-gray-800`} />
              </div>
              {/* Status badge */}
              <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-sm font-medium ${getHistoryStatusBadgeClass(status)}`}>
                {formatStatus(status)}
              </span>
            </div>

            {/* Content with timeline line */}
            <div class="flex">
              {/* Timeline line column */}
              <div class="relative flex flex-col items-center mr-4" style="width: 12px;">
                <div class="w-0.5 flex-1 bg-gray-200 dark:bg-gray-700" style={!isLast ? 'min-height: 2rem;' : ''} />
              </div>

              {/* Content column */}
              <div class={`flex-1 ${!isLast ? 'pb-6' : 'pb-0'}`}>
                {/* Time range */}
                <div class="text-sm text-gray-900 dark:text-white mb-1">
                  {isHelmRelease || firstTime === lastTime ? (
                    <span>{formatTimestamp(lastTime)}</span>
                  ) : (
                    <span>
                      {formatTimestamp(firstTime)} → {formatTimestamp(lastTime)}
                    </span>
                  )}
                </div>

                {/* Details */}
                <div class="text-sm text-gray-600 dark:text-gray-400 space-y-1">
                  {/* Duration and count */}
                  <div class="flex flex-wrap gap-x-4 gap-y-1">
                    {duration && (
                      <span>Duration: <span class="text-gray-900 dark:text-white">{duration}</span></span>
                    )}
                    {count !== undefined && (
                      <span>
                        {isHelmRelease ? 'Version' : 'Reconciliations'}: <span class="text-gray-900 dark:text-white">{count}</span>
                      </span>
                    )}
                  </div>

                  {/* Kind-specific metadata */}
                  {kindMetadata && (
                    <div class="break-all">
                      {kindMetadata.label}: <span class="text-gray-900 dark:text-white">{kindMetadata.value}</span>
                    </div>
                  )}

                  {/* App Version for HelmRelease */}
                  {isHelmRelease && entry.appVersion && (
                    <div>
                    App Version: <span class="text-gray-900 dark:text-white">{entry.appVersion}</span>
                    </div>
                  )}

                  {/* Digest (not for HelmRelease) */}
                  {!isHelmRelease && digest && (
                    <div>
                    Digest: <span class="text-gray-900 dark:text-white">{truncateDigest(digest)}</span>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        )
      })}

      {/* Timeline start marker */}
      <div class="relative flex">
        <div class="relative flex flex-col items-center mr-4 pt-0.5">
          <div class="w-3 h-3 rounded-full flex-shrink-0 bg-gray-300 dark:bg-gray-600 ring-4 ring-white dark:ring-gray-800" />
        </div>
        <div class="flex-1" />
      </div>
    </div>
  )
}
