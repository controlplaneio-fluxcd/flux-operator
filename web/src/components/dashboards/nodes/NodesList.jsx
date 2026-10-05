// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState } from 'preact/hooks'
import { useSignal } from '@preact/signals'
import { formatBytes, percentOf } from '../../../utils/metrics'
import { formatTimestamp } from '../../../utils/time'
import { nodeBadge, nodeIssue, usageSeverity, severityRank, formatAge, formatCpu, plural } from '../../../utils/nodes'
import { ToggleGroup } from '../../common/ToggleGroup'
import { Chevron, Reveal } from '../../common/rowKit'
import { PILL_BASE } from '../../common/StatusPill'
import { TabbedPanel, Field, StatusBadge } from '../../search/detailPanel'
import { DashboardPanel } from '../common/panel'
import { severityTextClass } from '../common/ResourceMetric'
import { CHECK_CHIPS } from './FindingsList'

// Rows shown before the Show all button.
export const NODE_ROW_LIMIT = 50

// Node list filters, one per status pill color.
const STATUS_FILTERS = [
  { value: 'all', label: 'All' },
  { value: 'critical', label: 'Critical' },
  { value: 'warn', label: 'Warning' },
  { value: 'ok', label: 'Ready' }
]

const PILL_RANK = { critical: 0, warn: 1, unknown: 2, ok: 3 }
const STATUS_RANK = ['Unreachable', 'NotReady', 'Pressure', 'Cordoned', 'Unknown', 'Ready']
const statusRank = (s) => {
  const i = STATUS_RANK.indexOf(s)
  return i === -1 ? STATUS_RANK.length : i
}

const RESOURCES = [['cpu', 'CPU', formatCpu], ['memory', 'Memory', formatBytes]]

/**
 * Sort nodes worst first: the status pill color (red, yellow, grey, green),
 * then the Kubernetes status, then the node's worst finding (severity, then
 * the code order of the backend's grouped lines), then memory share.
 *
 * @param {Array<Object>} nodes - Nodes from the snapshot
 * @param {Array<Object>} lines - Grouped finding lines, in backend order
 */
export function sortNodes(nodes, lines = []) {
  const codeOrder = new Map(lines.map((l, i) => [l.code, i]))
  const worst = (n) => (n.findings || [])
    .map(f => severityRank(f.severity) * 1000 + (codeOrder.get(f.code) ?? 999))
    .reduce((a, b) => Math.min(a, b), 9999)
  const memShare = (n) => percentOf(n.usage?.memory ?? n.requests?.memory, n.allocatable?.memory) ?? 0
  return nodes
    .map(n => ({ n, pill: PILL_RANK[nodeBadge(n).severity], status: statusRank(n.status), worst: worst(n), mem: memShare(n) }))
    .sort((a, b) => a.pill - b.pill || a.status - b.status || a.worst - b.worst || b.mem - a.mem)
    .map(x => x.n)
}

/**
 * Absolute amounts of a node resource for tooltips and screen-reader text.
 */
function amounts(node, kind, format) {
  const used = node.usage?.[kind]
  return [used != null && `${format(used)} used`, `${format(node.requests?.[kind])} requested`, `${format(node.allocatable?.[kind])} allocatable`]
    .filter(Boolean).join(' · ')
}

/**
 * UsageText - share of allocatable for a node row's second line: usage, or
 * requests without metrics-server, colored at the usage thresholds.
 */
function UsageText({ node, kind, label, format, metricsAvailable }) {
  const share = metricsAvailable
    ? percentOf(node.usage?.[kind], node.allocatable?.[kind])
    : percentOf(node.requests?.[kind], node.allocatable?.[kind])
  const severity = metricsAvailable ? usageSeverity(kind, share) : (share >= 90 ? 'warn' : null)
  return (
    <span class="shrink-0 whitespace-nowrap" title={amounts(node, kind, format)}>
      {metricsAvailable ? label : `${label} requests`}{' '}
      <span class={`tabular-nums ${severityTextClass(severity)}`}>{share == null ? '-' : `${share}%`}</span>
      <span class="sr-only"> ({amounts(node, kind, format)})</span>
    </span>
  )
}

/**
 * NoMetrics - a node row's usage when metrics-server has no sample for the
 * node: one token, with the requested amounts in the tooltip.
 */
function NoMetrics({ node }) {
  const text = `CPU ${amounts(node, 'cpu', formatCpu)}; Memory ${amounts(node, 'memory', formatBytes)}`
  return (
    <span class="shrink-0 whitespace-nowrap" title={text} data-testid="node-no-metrics">
      no metrics<span class="sr-only"> ({text})</span>
    </span>
  )
}

/**
 * NodeDetail - expanded node row in the list detail style. Overview:
 * identity fields on the left, resource shares and the node's issues on the
 * right. Taints and System tabs hold the rest.
 */
function NodeDetail({ node, badge, metricsAvailable, controlPlaneVersion }) {
  const [tab, setTab] = useState('overview')
  const tabs = [
    { id: 'overview', label: 'Overview' },
    ...(node.taints?.length > 0 ? [{ id: 'taints', label: 'Taints' }] : []),
    { id: 'system', label: 'System' }
  ]
  const info = node.info || {}
  return (
    <TabbedPanel tabs={tabs} active={tab} onSelect={setTab}>
      {tab === 'overview' && (
        <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-4 text-xs">
          <div class="space-y-2.5 self-start">
            <Field label="Node">{node.name}</Field>
            <Field label="Status"><StatusBadge status={badge.label} colorClass={badge.class} /></Field>
            <Field label="Pool">{node.pool}</Field>
            <Field label="Zone">{node.zone}</Field>
            <Field label="Instance">{node.instanceType}</Field>
            <Field label="Kubelet">{info.kubeletVersion}</Field>
            <Field label="Created">{node.createdAt ? formatTimestamp(node.createdAt) : null}</Field>
          </div>
          <div class="space-y-2.5 self-start md:border-l md:border-gray-200 md:dark:border-gray-700 md:pl-8">
            {RESOURCES.map(([k, label, format]) => {
              const requested = percentOf(node.requests?.[k], node.allocatable?.[k])
              const used = metricsAvailable ? percentOf(node.usage?.[k], node.allocatable?.[k]) : null
              const limits = percentOf(node.limits?.[k], node.allocatable?.[k])
              const parts = [
                metricsAvailable && !node.usage && 'no metrics',
                used != null && <span class={severityTextClass(usageSeverity(k, used))}>{used}% used</span>,
                requested != null && `${requested}% requested`,
                k === 'memory' && limits > 100 && `limits ${limits}%`
              ].filter(Boolean)
              if (parts.length === 0) return null
              return (
                <Field key={k} label={label}>
                  <span title={amounts(node, k, format)}>
                    {parts.map((p, i) => <span key={i}>{i > 0 && ' · '}{p}</span>)}
                    <span class="sr-only"> ({amounts(node, k, format)})</span>
                  </span>
                </Field>
              )
            })}
            <Field label="Pods">{node.pods} / {node.allocatable?.pods}</Field>
            {(node.findings || []).map((f, i) => {
              const issue = nodeIssue(f, node, controlPlaneVersion)
              const chip = CHECK_CHIPS[f.severity] || CHECK_CHIPS.info
              return (
                <div key={i} class="text-gray-700 dark:text-gray-300 break-words" data-testid="node-issue">
                  <StatusBadge status={chip.label} colorClass={chip.class} />{' '}
                  {issue.title}{issue.detail && <span class="text-gray-500 dark:text-gray-400"> · {issue.detail}</span>}
                </div>
              )
            })}
          </div>
        </div>
      )}
      {tab === 'taints' && (
        <div class="space-y-1.5 text-xs">
          {node.taints.map((t, i) => (
            <div key={i} class="font-mono text-gray-900 dark:text-gray-100 break-all">
              {t.key}{t.value ? `=${t.value}` : ''}:{t.effect}
              {t.timeAdded && <span class="font-sans text-gray-500 dark:text-gray-400"> · {formatTimestamp(t.timeAdded)}</span>}
            </div>
          ))}
        </div>
      )}
      {tab === 'system' && (
        <div class="space-y-2.5 text-xs">
          <Field label="Runtime">{info.containerRuntimeVersion}</Field>
          <Field label="OS">{info.osImage}</Field>
          <Field label="Architecture">{info.architecture}</Field>
          <Field label="Kernel">{info.kernelVersion}</Field>
          <Field label="Heartbeat">{typeof node.heartbeatSeconds === 'number' ? `${node.heartbeatSeconds}s ago` : null}</Field>
        </div>
      )}
    </TabbedPanel>
  )
}

/**
 * Tooltip of the status pill: the full label and how long the node has
 * been in that state.
 */
function pillTitle(node, label) {
  const since = node.status === 'Cordoned'
    ? (node.taints || []).find(t => t.key === 'node.kubernetes.io/unschedulable')?.timeAdded
    : (node.status === 'Unreachable' || node.status === 'NotReady')
      ? (node.conditions || []).find(c => c.type === 'Ready')?.lastTransitionTime
      : null
  const age = formatAge(since)
  return age ? `${label} for ${age}` : label
}

/**
 * NodeRow - compact two-line row in the Inventory tab style: the node name,
 * its status pill and the expand chevron, then pool · zone · instance type
 * and the CPU and Memory shares. The whole row toggles the node detail. The
 * button's accessible name is its visible text: name, status and usage.
 */
function NodeRow({ node, shortName, metricsAvailable, controlPlaneVersion }) {
  const [open, setOpen] = useState(false)
  const badge = nodeBadge(node)
  const labels = [node.pool, node.zone, node.instanceType].filter(Boolean)
  return (
    <div class="border-b border-gray-100 dark:border-gray-700/60 last:border-0" data-testid="node-row">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        class="group block w-full text-left px-3 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/30"
      >
        {/* Line 1: name (fills) + status pill + chevron. */}
        <div class="flex items-center gap-2.5">
          <span class="block flex-1 min-w-0 truncate text-sm font-semibold text-gray-900 dark:text-gray-100" title={node.name}>{shortName}</span>
          <span class={`${PILL_BASE} ${badge.class} shrink-0 max-w-[50%]`} title={pillTitle(node, badge.label)} data-testid="node-pill">
            <span class="min-w-0 truncate">{badge.label}</span>
          </span>
          <span class="shrink-0 p-0.5 text-gray-400 group-hover:text-flux-blue" aria-hidden="true"><Chevron open={open} /></span>
        </div>
        {/* Line 2: labels · CPU · Memory (labels hidden on phones). */}
        <div class="mt-0.5 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          {labels.length > 0 && (
            <>
              <span class="hidden sm:block min-w-0 truncate">{labels.join(' · ')}</span>
              <span class="hidden sm:block shrink-0 text-gray-300 dark:text-gray-600" aria-hidden="true">·</span>
            </>
          )}
          {metricsAvailable && !node.usage ? (
            // Metrics-server has no sample for this node (down or not scraped).
            <NoMetrics node={node} />
          ) : (
            <>
              <UsageText node={node} kind="cpu" label="CPU" format={formatCpu} metricsAvailable={metricsAvailable} />
              <span class="shrink-0 text-gray-300 dark:text-gray-600" aria-hidden="true">·</span>
              <UsageText node={node} kind="memory" label="Memory" format={formatBytes} metricsAvailable={metricsAvailable} />
            </>
          )}
        </div>
      </button>
      <Reveal open={open}>
        <div class="px-3 pt-1 pb-4">
          <NodeDetail node={node} badge={badge} metricsAvailable={metricsAvailable} controlPlaneVersion={controlPlaneVersion} />
        </div>
      </Reveal>
    </div>
  )
}

/**
 * NodesList - the Nodes panel: node rows sorted worst first, with a status
 * filter and a search box in the Inventory tab toolbar style. Shows the
 * NODE_ROW_LIMIT worst nodes, then a Show all button.
 *
 * @param {Object} props
 * @param {Object} props.data - Snapshot from GET /api/v1/nodes
 * @param {Function} props.shortName - Strips the shared DNS suffix from node names
 */
export function NodesList({ data, shortName }) {
  const statusFilter = useSignal('all')
  const query = useSignal('')
  const [showAll, setShowAll] = useState(false)

  const nodes = data.nodes || []
  const q = query.value.trim().toLowerCase()
  const filtered = nodes.filter(n => {
    if (statusFilter.value !== 'all' && nodeBadge(n).severity !== statusFilter.value) return false
    if (q && ![n.name, n.pool, n.zone, n.instanceType].some(v => v?.toLowerCase().includes(q))) return false
    return true
  })
  const sorted = sortNodes(filtered, data.findings)
  const visible = showAll ? sorted : sorted.slice(0, NODE_ROW_LIMIT)
  const clearFilters = () => { statusFilter.value = 'all'; query.value = '' }

  return (
    <DashboardPanel
      title="Nodes"
      subtitle={<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{plural(nodes.length, 'node')}</p>}
    >
      <div class="space-y-3">
        {/* Toolbar: status segmented control + search box + clear, as in the Inventory tab. */}
        <div class="flex flex-col sm:flex-row sm:items-center gap-2">
          <ToggleGroup ariaLabel="Filter nodes by status" options={STATUS_FILTERS} value={statusFilter.value} onChange={(v) => statusFilter.value = v} />
          <div class="flex-1 flex items-center gap-2">
            <div class="relative flex-1 min-w-0">
              <svg class="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                type="text"
                value={query.value}
                onInput={(e) => query.value = e.currentTarget.value}
                placeholder="Search by name, pool, zone or instance type"
                aria-label="Search nodes"
                class="w-full pl-8 pr-2 py-1 text-xs border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-flux-blue"
              />
            </div>
            <button
              onClick={clearFilters}
              title="Clear"
              aria-label="Clear filters"
              class="inline-flex items-center p-1 rounded-md text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white focus:outline-none focus-visible:ring-2 focus-visible:ring-flux-blue transition-colors"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>
        {sorted.length === 0 ? (
          <div class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">No nodes match the filters</div>
        ) : (
          <div class="card overflow-hidden p-0 sm:p-2">
            {visible.map(n => (
              <NodeRow
                key={n.name}
                node={n}
                shortName={shortName(n.name)}
                metricsAvailable={!!data.metricsAvailable}
                controlPlaneVersion={data.controlPlaneVersion}
              />
            ))}
          </div>
        )}
        {sorted.length > visible.length && (
          <div class="flex justify-center">
            <button
              type="button"
              onClick={() => setShowAll(true)}
              class="text-sm text-flux-blue dark:text-blue-400 hover:underline focus:outline-none focus:ring-2 focus:ring-flux-blue rounded"
            >
              Show all {plural(sorted.length, 'node')}
            </button>
          </div>
        )}
      </div>
    </DashboardPanel>
  )
}
