// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useSignal } from '@preact/signals'
import { formatBytes, percentOf } from '../../../utils/metrics'
import { summaryTitle, formatCpu, plural } from '../../../utils/nodes'
import { ResourceMetric } from '../common/ResourceMetric'

// Status icons shared with the Cluster Sync panel.
const STATUS_ICONS = {
  critical: (
    <svg class="w-5 h-5 text-danger" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  warn: (
    <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
    </svg>
  ),
  ok: (
    <svg class="w-5 h-5 text-success" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  )
}

/**
 * StatusLine - the worst finding and the count of the others, or all
 * healthy. Users who can list nodes get the line as a link to the Nodes
 * dashboard; everyone else gets the same line as plain text.
 */
function StatusLine({ findings, canViewNodes }) {
  const issues = (findings?.critical || 0) + (findings?.warn || 0)
  const worst = findings?.top?.[0]
  const more = issues - 1
  const text = worst
    ? `${summaryTitle(worst)}${more > 0 ? ` · ${more} more ${more === 1 ? 'issue' : 'issues'}` : ''}`
    : 'All nodes healthy'
  const icon = <span class="flex-shrink-0">{STATUS_ICONS[worst?.severity === 'critical' ? 'critical' : worst ? 'warn' : 'ok']}</span>
  if (canViewNodes) {
    return (
      <a href="/nodes" class="flex items-center gap-2 text-sm text-flux-blue dark:text-blue-400 hover:underline" data-testid="nodes-summary-status">
        {icon}<span>{text}</span>
      </a>
    )
  }
  return (
    <div class="flex items-center gap-2 text-sm text-gray-900 dark:text-white" data-testid="nodes-summary-status">
      {icon}<span>{text}</span>
    </div>
  )
}

/**
 * Build the ResourceMetric props of a resource: usage and requests as shares
 * of allocatable, or requests only without metrics-server. The absolute
 * amounts go in the tooltip and in screen-reader text. Returns null when the shares are unknown (no
 * eligible capacity), so the row is hidden.
 */
export function summaryResourceRow(r, format) {
  const requested = percentOf(r?.requested, r?.allocatable)
  if (requested == null) return null
  const title = [r.used != null && `${format(r.used)} used`, `${format(r.requested)} requested`, `${format(r.allocatable)} allocatable`]
    .filter(Boolean).join(' · ')
  const used = percentOf(r.used, r.allocatable)
  if (used == null) {
    return { title, value: `${requested}% requested`, barPercent: requested }
  }
  return { title, value: `${used}% used`, percentLabel: `${requested}% requested`, barPercent: used }
}

/**
 * Fact - one label/value pair in the Cluster Info `dl` style.
 */
function Fact({ label, children }) {
  return (
    <div class="flex items-baseline space-x-2">
      <dt class="text-xs sm:text-sm text-gray-500 dark:text-gray-400">{label}:</dt>
      <dd class="text-xs sm:text-sm font-semibold text-gray-900 dark:text-white">{children}</dd>
    </div>
  )
}

/**
 * NodesSummaryPanel - node health summary on the cluster dashboard, visible
 * to all users. It is rendered from the name-less summary in the report
 * (spec.nodes), which carries only codes, counts and totals.
 *
 * @param {Object} props
 * @param {Object} props.summary - Nodes summary from the report spec
 * @param {number} [props.nodeCount] - Number of nodes (spec.cluster.nodes)
 * @param {boolean} props.canViewNodes - Whether the user can open the Nodes dashboard
 */
export function NodesSummaryPanel({ summary, nodeCount, canViewNodes }) {
  const isExpanded = useSignal(true)
  if (!summary) return null

  const { findings = {}, pods = {} } = summary
  const cpu = summaryResourceRow(summary.cpu, formatCpu)
  const memory = summaryResourceRow(summary.memory, formatBytes)
  const badges = [
    summary.down > 0 && { text: `${summary.down} down`, cls: 'status-not-ready' },
    summary.pressure > 0 && { text: `${summary.pressure} under pressure`, cls: 'status-not-ready' },
    summary.cordoned > 0 && { text: `${summary.cordoned} cordoned`, cls: 'status-warning' }
  ].filter(Boolean)

  return (
    <div class="card p-0" data-testid="nodes-summary-panel">
      <button
        onClick={() => isExpanded.value = !isExpanded.value}
        class="w-full px-6 py-4 border-b border-gray-200 dark:border-gray-700 text-left hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
        aria-expanded={isExpanded.value}
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-white">Cluster Nodes</h3>
            <div class="flex flex-wrap items-center gap-x-4 gap-y-1 mt-1">
              {typeof nodeCount === 'number' && (
                <p class="text-sm text-gray-600 dark:text-gray-400">{plural(nodeCount, 'node')}</p>
              )}
              {badges.map(b => (
                <span key={b.text} class={`status-badge whitespace-nowrap ${b.cls} text-xs sm:text-sm`}>{b.text}</span>
              ))}
            </div>
          </div>
          <svg
            class={`w-5 h-5 text-gray-400 dark:text-gray-500 transition-transform ${isExpanded.value ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
          </svg>
        </div>
      </button>
      {isExpanded.value && (
        <div class="px-6 py-4">
          <div class="flex flex-col lg:flex-row lg:gap-8">
            {/* Left side: counts and the worst finding */}
            <div class="space-y-3 lg:flex-1">
              <dl class="grid grid-cols-2 gap-x-6 gap-y-2">
                <Fact label="Pods">{pods.running ?? 0} / {pods.allocatable ?? '-'}</Fact>
                <Fact label="Unschedulable">{summary.unschedulablePods ?? 0}</Fact>
                <Fact label="Critical">{findings.critical ?? 0}</Fact>
                <Fact label="Warnings">{findings.warn ?? 0}</Fact>
              </dl>
              <StatusLine findings={findings} canViewNodes={canViewNodes} />
            </div>

            {/* Right side: cluster usage, or requests without metrics-server */}
            {(cpu || memory) && (
              <div class="space-y-3 mt-4 pt-4 border-t border-gray-200 dark:border-gray-700 lg:flex-1 lg:mt-0 lg:pt-0 lg:border-t-0 lg:border-l lg:pl-8">
                {cpu && <div title={cpu.title} data-testid="nodes-summary-cpu"><ResourceMetric label="CPU" {...cpu} /><span class="sr-only">{cpu.title}</span></div>}
                {memory && <div title={memory.title} data-testid="nodes-summary-memory"><ResourceMetric label="Memory" {...memory} /><span class="sr-only">{memory.title}</span></div>}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
