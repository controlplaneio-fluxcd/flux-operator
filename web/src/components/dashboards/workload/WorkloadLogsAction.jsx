// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState, useRef, useMemo, useEffect, useCallback } from 'preact/hooks'
import { WorkloadLogsViewer } from './WorkloadLogsViewer'
import { urlWithParam } from '../../../utils/routing'

// LOGS_QUERY_PARAM carries the open viewer in the URL so a session is shareable:
// a pod name, or ALL_PODS_VALUE for "All pods". Shared with WorkloadDetailPanel,
// which writes the same param. ALL_PODS_VALUE is `*`, not a valid pod name, so it
// can never collide with a real pod.
export const LOGS_QUERY_PARAM = 'logs'
export const ALL_PODS_VALUE = '*'

// containersOf builds the viewer's container list for a pod (init containers
// first). The restart count lets the viewer offer a "(previous)" entry for
// restarted containers.
const containersOf = (pod) => [
  ...(pod.podStatus.initContainerStatuses || []).map(cs => ({ name: cs.name, isInit: true, restartCount: cs.restartCount || 0 })),
  ...(pod.podStatus.containerStatuses || []).map(cs => ({ name: cs.name, isInit: false, restartCount: cs.restartCount || 0 }))
]

/**
 * WorkloadLogsAction - "View logs" button for the workload dashboard action bar.
 *
 * Opens the viewer on "All pods"; its pod selector then narrows to one pod. Renders
 * nothing without the 'logs' user action. With the action but no inspectable pods
 * (e.g. scaled to zero), the button is shown disabled rather than hidden, so the
 * capability stays visible.
 *
 * @param {Object} props
 * @param {string} props.kind - Workload kind (shown in the viewer title)
 * @param {string} props.namespace - Workload namespace
 * @param {string} props.name - Workload name (shown in the viewer title)
 * @param {Array} props.pods - Pods of the workload (each with a podStatus)
 * @param {Array} props.userActions - Allowed user actions for the workload
 */
export function WorkloadLogsAction({ kind, namespace, name, pods = [], userActions = [] }) {
  // Open session: { key, pod } where pod is a pod name or null for "All pods". key
  // increments per open so the viewer remounts and re-inits its pod selection. null
  // when no viewer is open.
  const [session, setSession] = useState(null)
  const sessionKeyRef = useRef(0)

  // RBAC gate: only users authorized to read logs see the button at all.
  const canViewLogs = userActions.includes('logs')

  // Pods that can be inspected: those carrying container status (needed to populate
  // the viewer's container list). Empty for a scaled-to-zero workload.
  const logsPods = useMemo(
    () => (canViewLogs ? pods.filter(p => p.podStatus) : []),
    [pods, canViewLogs]
  )

  // Live pods passed to the viewer, each with its containers, so it can build the
  // "All pods" request and resolve a pod's containers (and restart counts) from the
  // polled data while open.
  const viewerPods = useMemo(
    () => logsPods.map(p => ({ name: p.name, status: p.status, containers: containersOf(p) })),
    [logsPods]
  )

  // Keep the URL pointed at the pod shown (on open and every in-viewer switch via
  // onPodChange) so the address bar is a shareable link. A null pod is "All pods".
  const syncLogFilterToUrl = useCallback((pod) => {
    window.history.replaceState(null, '', urlWithParam(LOGS_QUERY_PARAM, pod || ALL_PODS_VALUE))
  }, [])

  // Deep link: a `?logs=<pod|*>` param opens the viewer on that pod (or "All pods")
  // so a shared link lands in the logs. Runs once on mount. An unknown pod still
  // opens the viewer, which falls back to "All pods" when the pod is gone.
  const deepLinked = useRef(false)
  useEffect(() => {
    if (deepLinked.current) return
    deepLinked.current = true
    // Nothing to show for a workload with no inspectable pods, even with a ?logs link.
    if (logsPods.length === 0) return
    const logs = new URLSearchParams(window.location.search).get(LOGS_QUERY_PARAM)
    if (!logs) return
    sessionKeyRef.current += 1
    setSession({ key: sessionKeyRef.current, pod: logs === ALL_PODS_VALUE ? null : logs })
  }, [])

  if (!canViewLogs) {
    return null
  }

  // No pods to inspect (e.g. scaled to zero): the button is shown but disabled.
  const hasPods = logsPods.length > 0

  // Open the viewer on "All pods"; its pod selector narrows from there.
  const openAllPods = () => {
    sessionKeyRef.current += 1
    setSession({ key: sessionKeyRef.current, pod: null })
    syncLogFilterToUrl(null)
  }

  const closeSession = () => {
    setSession(null)
    window.history.replaceState(null, '', urlWithParam(LOGS_QUERY_PARAM, null))
  }

  const baseButtonClass = 'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded border transition-colors focus:outline-none'
  const buttonClass = hasPods
    ? `${baseButtonClass} border-teal-500 text-teal-600 hover:bg-teal-50 dark:border-teal-400 dark:text-teal-400 dark:hover:bg-teal-900/30 focus:ring-2 focus:ring-offset-1 dark:focus:ring-offset-gray-900 focus:ring-teal-500`
    : `${baseButtonClass} border-gray-300 text-gray-400 cursor-not-allowed dark:border-gray-600 dark:text-gray-500`

  return (
    <div class="relative" data-testid="workload-logs-action">
      <button
        onClick={hasPods ? openAllPods : undefined}
        disabled={!hasPods}
        class={buttonClass}
        data-testid="view-logs-button"
        title={hasPods ? 'View pod logs' : 'No running pods to view logs'}
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        View logs
      </button>

      {/* Stays mounted even with no pods, showing an inline notice instead of
          vanishing; the button above still disables when there are no pods. */}
      {session && (
        <WorkloadLogsViewer
          key={session.key}
          kind={kind}
          namespace={namespace}
          workloadName={name}
          pods={viewerPods}
          initialPodName={session.pod || undefined}
          onClose={closeSession}
          onPodChange={syncLogFilterToUrl}
        />
      )}
    </div>
  )
}
