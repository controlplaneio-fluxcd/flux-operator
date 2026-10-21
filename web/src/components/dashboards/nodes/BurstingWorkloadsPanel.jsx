// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { formatBytes } from '../../../utils/metrics'
import { plural } from '../../../utils/nodes'
import { getDashboardUrl } from '../../../utils/routing'
import { KindChip, NameLink, NEUTRAL_CHIP } from '../../common/rowKit'
import { DashboardPanel } from '../common/panel'

// Pill sized to the text-xs bar rows, so rows with and without it are the
// same height.
const ROW_PILL = 'inline-flex items-center px-2 rounded-full text-xs leading-4 font-medium'

/**
 * Used, requests, limits, pods and nodes of a workload, for its row tooltip and
 * accessible name.
 */
export function workloadDetails(w) {
  return [
    `used ${formatBytes(w.usage)}`,
    `requests ${formatBytes(w.requests)}`,
    w.limits != null ? `limits ${formatBytes(w.limits)}` : 'no limit',
    typeof w.pods === 'number' && `${plural(w.pods, 'pod')}${typeof w.nodes === 'number' ? ` on ${plural(w.nodes, 'node')}` : ''}`
  ].filter(Boolean).join(' · ')
}

/**
 * WorkloadRow - one workload in the Workloads list row style: kind chip,
 * namespace/name linking to the workload dashboard and a `no limit` pill,
 * then the memory above requests as a bar (scaled to the largest row) and
 * its value in fixed-width columns so the bars line up across rows. Used,
 * requests, limits and pods are in the row tooltip and screen-reader text.
 * On phones the kind chip moves to a second line.
 */
function WorkloadRow({ w, max }) {
  const value = w.aboveRequests ?? 0
  const details = workloadDetails(w)
  const width = max > 0 && value > 0 ? Math.max((value / max) * 100, 2) : 0
  return (
    <div class="border-b border-gray-100 dark:border-gray-700/60 last:border-0" data-testid="bursting-workload-row">
      <div class="px-3 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/30" title={`${w.kind} ${w.namespace}/${w.name} · ${details}`}>
        <div class="flex items-center gap-2.5">
          <KindChip kind={w.kind} colorClass={NEUTRAL_CHIP} title={w.kind} cls="hidden sm:inline-block" />
          <NameLink href={getDashboardUrl(w.kind, w.namespace, w.name)} namespace={w.namespace} name={w.name} />
          {w.noLimit && <span class={`${ROW_PILL} status-warning shrink-0`}>no limit</span>}
          <span class="hidden sm:block flex-1" />
          <div class="usage-bar-track usage-bar-track-memory w-20 sm:w-24 shrink-0">
            <div class="usage-bar-fill usage-bar-fill-memory" style={{ width: `${width}%` }} />
          </div>
          <span class="w-14 shrink-0 text-right text-xs tabular-nums text-gray-900 dark:text-white">
            {formatBytes(value)}
            <span class="sr-only"> above requests · {details}</span>
          </span>
        </div>
        {/* Mobile-only second line: the kind chip. */}
        <div class="sm:hidden mt-1 flex items-center gap-2">
          <KindChip kind={w.kind} colorClass={NEUTRAL_CHIP} title={w.kind} />
        </div>
      </div>
    </div>
  )
}

/**
 * BurstingWorkloadsPanel - the workloads using the most memory above their
 * requests, from GET /api/v1/nodes/workloads, filtered by the backend to
 * the namespaces the user can see, one Workloads-list row each. Without
 * pod metrics or bursting workloads, the panel states that fact; a failed
 * fetch shows as failed, never as stale rows.
 *
 * @param {Object} props
 * @param {{metricsAvailable: boolean, workloads: Array<Object>}} [props.data] - Workloads response
 * @param {string} [props.error] - Error of the last fetch
 */
export function BurstingWorkloadsPanel({ data, error }) {
  const workloads = [...(data?.workloads || [])].sort((a, b) => (b.aboveRequests || 0) - (a.aboveRequests || 0))
  const metrics = !!data?.metricsAvailable

  let body
  if (error) {
    body = <p class="text-sm text-red-700 dark:text-red-300">Failed to load workloads</p>
  } else if (!metrics) {
    body = <p class="text-sm text-gray-500 dark:text-gray-400">No pod metrics</p>
  } else if (workloads.length === 0) {
    body = <p class="text-sm text-gray-500 dark:text-gray-400">No workloads above memory requests</p>
  } else {
    const max = workloads[0].aboveRequests || 0
    body = (
      <div class="card overflow-hidden p-0 sm:p-2">
        {workloads.map(w => <WorkloadRow key={`${w.kind}/${w.namespace}/${w.name}`} w={w} max={max} />)}
      </div>
    )
  }

  return (
    <DashboardPanel
      title="Bursting Workloads"
      subtitle={!error && metrics && (
        <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{plural(workloads.length, 'workload')}</p>
      )}
    >
      <div data-testid="bursting-workloads">{body}</div>
    </DashboardPanel>
  )
}
