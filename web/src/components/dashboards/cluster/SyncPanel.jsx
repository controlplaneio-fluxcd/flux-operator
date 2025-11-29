// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'
import { useLocation } from 'preact-iso'

/**
 * SyncPanel component - Displays cluster sync information
 *
 * @param {Object} props
 * @param {Object} props.sync - Cluster sync information
 * @param {string} props.namespace - Report namespace
 */
export function SyncPanel({ sync, namespace }) {
  const location = useLocation()

  // Extract name from sync.id (e.g., "kustomization/flux-system" -> "flux-system")
  const syncName = sync.id ? sync.id.split('/').pop() : ''
  const isExpanded = useSignal(true)
  const isSuspended = sync.status && sync.status.startsWith('Suspended')

  const getStatusInfo = () => {
    if (isSuspended) {
      return {
        icon: (
          <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        ),
        badge: 'status-badge status-warning',
        label: 'Suspended'
      }
    } else if (sync.ready) {
      return {
        icon: (
          <svg class="w-5 h-5 text-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        ),
        badge: 'status-badge status-ready',
        label: 'Synced'
      }
    } else {
      return {
        icon: (
          <svg class="w-5 h-5 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        ),
        badge: 'status-badge status-not-ready',
        label: 'Not Synced'
      }
    }
  }

  const statusInfo = getStatusInfo()

  return (
    <div class="card">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class={`w-full text-left hover:opacity-80 transition-opacity ${isExpanded.value ? 'mb-4' : ''}`}
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white">Cluster Sync</h3>
            <div class="flex items-center space-x-4 mt-1">
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  location.route(`/resource/${encodeURIComponent('Kustomization')}/${encodeURIComponent(namespace)}/${encodeURIComponent(syncName)}`)
                }}
                class="text-sm text-flux-blue dark:text-blue-400 hover:underline break-words"
              >
                <span class="sm:hidden">{syncName}</span>
                <span class="hidden sm:inline">Kustomization/{namespace}/{syncName}</span>
              </button>
              {!sync.ready && !isSuspended && (
                <span class="status-badge status-not-ready text-xs sm:text-sm">
                  failing
                </span>
              )}
            </div>
          </div>
          <svg
            class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isExpanded.value ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </button>

      {isExpanded.value && (
        <div class="space-y-2">
          <div class="flex flex-col gap-2 text-sm text-gray-900 dark:text-white break-all">
            <div class="flex items-start gap-2">
              {(sync.source?.startsWith('http') || sync.source?.startsWith('ssh')) ? (
                <svg class="w-5 h-5 flex-shrink-0 mt-0.5 text-blue-600 dark:text-blue-400" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M21.62 11.108l-8.731-8.729a1.292 1.292 0 0 0-1.823 0L9.257 4.19l2.299 2.3a1.532 1.532 0 0 1 1.939 1.95l2.214 2.217a1.53 1.53 0 0 1 1.583 2.531c-.599.6-1.566.6-2.166 0a1.536 1.536 0 0 1-.337-1.662l-2.074-2.063V14.5a1.473 1.473 0 0 1 .404.29c.6.6.6 1.569 0 2.166a1.536 1.536 0 0 1-2.174 0 1.528 1.528 0 0 1 0-2.164c.152-.15.322-.264.504-.339v-5.11a1.391 1.391 0 0 1-.504-.339 1.527 1.527 0 0 1-.322-1.664L8.319 5.025l-5.939 5.94a1.292 1.292 0 0 0 0 1.821l8.732 8.732a1.29 1.29 0 0 0 1.823 0l8.69-8.59a1.29 1.29 0 0 0-.005-1.82z" />
                </svg>
              ) : sync.source?.startsWith('oci://') ? (
                <svg class="w-5 h-5 flex-shrink-0 mt-0.5 text-blue-600 dark:text-blue-400" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M13.983 11.078h2.119a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.119a.185.185 0 0 0-.185.185v1.888c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 0 0 .186-.186V3.574a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.888c0 .102.082.185.185.186m0 2.716h2.118a.187.187 0 0 0 .186-.186V6.29a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.887c0 .102.082.185.185.186m-2.93 0h2.12a.186.186 0 0 0 .184-.186V6.29a.185.185 0 0 0-.185-.185H8.1a.185.185 0 0 0-.185.185v1.887c0 .102.083.185.185.186m-2.964 0h2.119a.186.186 0 0 0 .185-.186V6.29a.185.185 0 0 0-.185-.185H5.136a.186.186 0 0 0-.186.185v1.887c0 .102.084.185.186.186m5.893 2.715h2.118a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.118a.185.185 0 0 0-.185.185v1.888c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.184.185v1.888c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 0 0 .185-.185V9.006a.185.185 0 0 0-.185-.186H5.136a.186.186 0 0 0-.186.185v1.888c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.184.185v1.888c0 .102.082.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 0 0-.75.748 11.376 11.376 0 0 0 .692 4.062c.545 1.428 1.355 2.48 2.41 3.124 1.18.723 3.1 1.137 5.275 1.137.983.003 1.963-.086 2.93-.266a12.248 12.248 0 0 0 3.823-1.389c.98-.567 1.86-1.288 2.61-2.136 1.252-1.418 1.998-2.997 2.553-4.4h.221c1.372 0 2.215-.549 2.68-1.009.309-.293.55-.65.707-1.046l.098-.288Z" />
                </svg>
              ) : (
                <svg class="w-5 h-5 flex-shrink-0 mt-0.5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-width="2" d="m20.25 7.5-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" />
                </svg>
              )}
              <span class="break-all">{sync.source}</span>
            </div>
            <div class="flex items-start gap-2">
              <svg class="w-5 h-5 flex-shrink-0 mt-0.5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
              </svg>
              <span class="break-all">{sync.path}</span>
            </div>
          </div>
          <div class="flex items-start gap-2 text-sm text-gray-900 dark:text-white">
            <span class="flex-shrink-0 mt-0.5">{statusInfo.icon}</span>
            <span class="whitespace-pre-wrap break-all max-w-full overflow-hidden">{sync.status}</span>
          </div>
        </div>
      )}
    </div>
  )
}
