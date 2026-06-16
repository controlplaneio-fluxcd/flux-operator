// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useEffect, useRef, useMemo } from 'preact/hooks'
import { WorkloadLogsViewer } from './WorkloadLogsViewer'

/**
 * WorkloadLogsAction - "View logs" dropdown for the workload dashboard action bar.
 *
 * Lists the pods of the workload and opens the logs viewer for the selected one.
 * Renders nothing unless the user is authorized to read logs ('logs' user action)
 * and there is at least one pod with container status to view.
 *
 * @param {Object} props
 * @param {string} props.namespace - Workload namespace
 * @param {Array} props.pods - Pods of the workload (each with a podStatus)
 * @param {Array} props.userActions - Allowed user actions for the workload
 */
export function WorkloadLogsAction({ namespace, pods = [], userActions = [] }) {
  const [open, setOpen] = useState(false)
  const [logsPodName, setLogsPodName] = useState(null)
  const dropdownRef = useRef(null)

  // Pods that can be inspected: the user is authorized to read logs and the pod
  // carries container status (needed to populate the viewer's container list).
  const logsPods = useMemo(
    () => (userActions.includes('logs') ? pods.filter(p => p.podStatus) : []),
    [pods, userActions]
  )

  // Live pod whose logs are displayed; looked up from the polled pods by name
  // so its restart counts (and the viewer's "(previous)" options) stay current
  // while the viewer is open. Resolves to null if the pod is gone, closing it.
  const logsPod = useMemo(
    () => (logsPodName ? logsPods.find(p => p.name === logsPodName) || null : null),
    [logsPodName, logsPods]
  )

  // Close the dropdown on click outside or Escape.
  useEffect(() => {
    if (!open) return
    const onClick = (e) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target)) setOpen(false)
    }
    const onKey = (e) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  if (logsPods.length === 0) {
    return null
  }

  // Build the container list for a pod (init containers first). The restart
  // count lets the viewer offer a "(previous)" entry for restarted containers.
  const containersOf = (pod) => [
    ...(pod.podStatus.initContainerStatuses || []).map(cs => ({ name: cs.name, isInit: true, restartCount: cs.restartCount || 0 })),
    ...(pod.podStatus.containerStatuses || []).map(cs => ({ name: cs.name, isInit: false, restartCount: cs.restartCount || 0 }))
  ]

  const handleSelect = (pod) => {
    setOpen(false)
    setLogsPodName(pod.name)
  }

  const baseButtonClass = 'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded border transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 dark:focus:ring-offset-gray-900'

  return (
    <div class="relative" ref={dropdownRef} data-testid="workload-logs-action">
      <button
        onClick={() => setOpen(v => !v)}
        class={`${baseButtonClass} ${
          open
            ? 'border-teal-500 text-teal-600 bg-teal-50 dark:border-teal-400 dark:text-teal-400 dark:bg-teal-900/30'
            : 'border-teal-500 text-teal-600 hover:bg-teal-50 dark:border-teal-400 dark:text-teal-400 dark:hover:bg-teal-900/30 focus:ring-teal-500'
        }`}
        data-testid="view-logs-dropdown-button"
        title="View pod logs"
        aria-expanded={open}
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        View logs
        <svg class="w-3 h-3 ml-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {open && (
        <div
          class="absolute left-0 mt-1 w-72 max-h-80 overflow-auto bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50"
          data-testid="view-logs-dropdown-menu"
        >
          {logsPods.map((pod) => (
            <button
              key={pod.name}
              onClick={() => handleSelect(pod)}
              class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              data-testid={`view-logs-pod-${pod.name}`}
            >
              <div class="font-medium truncate text-gray-900 dark:text-gray-100">{pod.name}</div>
              {pod.status && (
                <div class="text-xs text-gray-500 dark:text-gray-400 truncate">{pod.status}</div>
              )}
            </button>
          ))}
        </div>
      )}

      {logsPod && (
        <WorkloadLogsViewer
          namespace={namespace}
          name={logsPod.name}
          containers={containersOf(logsPod)}
          onClose={() => setLogsPodName(null)}
        />
      )}
    </div>
  )
}
