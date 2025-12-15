// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useLocation } from 'preact-iso'
import { getStatusBadgeClass, getStatusBorderClass } from '../../utils/status'
import { formatTimestamp } from '../../utils/time'
import { removeFavorite } from '../../utils/favorites'

/**
 * FavoriteCard - Card displaying a favorite Flux resource
 *
 * @param {Object} props
 * @param {Object} props.favorite - Favorite object with kind, namespace, name
 * @param {Object} props.resourceData - Resource data with status, lastReconciled (may be null if not found)
 */
export function FavoriteCard({ favorite, resourceData }) {
  const location = useLocation()
  const { kind, namespace, name } = favorite

  // Handle resource name click - navigate to dashboard
  const handleResourceClick = () => {
    location.route(`/resource/${encodeURIComponent(kind)}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`)
  }

  // Handle unfavorite
  const handleUnfavorite = (e) => {
    e.stopPropagation()
    removeFavorite(kind, namespace, name)
  }

  // Resource may not be found (deleted from cluster)
  const notFound = resourceData?.status === 'NotFound'
  const status = notFound ? 'Not Found' : (resourceData?.status || 'Unknown')
  const lastReconciled = resourceData?.lastReconciled
  const message = resourceData?.message

  return (
    <button
      onClick={handleResourceClick}
      class={`card border-l-4 p-4 hover:shadow-md transition-shadow dark:shadow-none cursor-pointer text-left w-full ${getStatusBorderClass(status)} ${notFound ? 'opacity-60' : ''}`}
    >
      {/* Header row: star + kind + status badge */}
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-1.5">
          {/* Favorite star button (unfavorite) */}
          <span
            onClick={handleUnfavorite}
            class="w-4 h-4 rounded transition-colors text-yellow-500 hover:text-yellow-600 flex-shrink-0 cursor-pointer"
            title="Remove from favorites"
          >
            <svg class="w-4 h-4" fill="currentColor" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
            </svg>
          </span>
          <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">
            {kind}
          </span>
          <span class={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${notFound ? 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400' : getStatusBadgeClass(status)}`}>
            {status}
          </span>
        </div>
      </div>

      {/* Resource name */}
      <div class="mb-2">
        <div
          class="text-sm text-left flex items-center gap-1.5 max-w-full"
          title={name}
        >
          <svg class="w-4 h-4 text-gray-400 dark:text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
          </svg>
          <span class="font-semibold text-gray-900 dark:text-gray-100 truncate">{name}</span>
          <svg class="w-3.5 h-3.5 text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </div>
      </div>

      {/* Not found message */}
      {notFound && (
        <div class="flex gap-1.5 text-sm text-yellow-600 dark:text-yellow-500">
          <svg class="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <span class="break-words">{message}</span>
        </div>
      )}

      {/* Info rows: namespace, timestamp, message */}
      {!notFound && (
        <div class="space-y-1">
          {/* Namespace */}
          <div class="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-300" title={`Namespace: ${namespace}`}>
            <svg class="w-4 h-4 text-gray-400 dark:text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
            </svg>
            <span class="truncate">{namespace}</span>
          </div>

          {/* Last reconciled timestamp */}
          {lastReconciled && (
            <div class="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-300">
              <svg class="w-4 h-4 text-gray-400 dark:text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              {formatTimestamp(lastReconciled)}
            </div>
          )}

          {/* Message - truncated */}
          {message && (
            <div class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400" title={message}>
              <svg class="w-4 h-4 text-gray-400 dark:text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              <span class="truncate">{message.charAt(0).toLowerCase() + message.slice(1)}</span>
            </div>
          )}
        </div>
      )}
    </button>
  )
}
