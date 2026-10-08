// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { formatCores, formatBytes, buildChartData, percentOf, percentSeverity, BINARY_TICK_INCRS } from '../../../utils/metrics'
import { formatCpu } from '../../../utils/nodes'
import { DashboardPanel } from '../common/panel'
import { ResourceMetric, severityTextClass } from '../common/ResourceMetric'
import { UsageChart } from '../workload/UsageChart'
import { ChartHeader } from '../workload/UsageCharts'

const RESOURCES = [
  { kind: 'cpu', label: 'CPU', format: formatCores, formatTitle: formatCpu },
  { kind: 'memory', label: 'Memory', format: formatBytes, formatTitle: formatBytes }
]

/**
 * Absolute amounts of a resource for the tooltip of its headline and its
 * screen-reader text.
 */
function capacityTitle(r, format) {
  return [r.used != null && `${format(r.used)} used`, `${format(r.requested)} requested`, `${format(r.allocatable)} allocatable`]
    .filter(Boolean).join(' · ')
}

/**
 * CapacityUsage - one resource with metrics: the latest usage and the
 * requests as shares of allocatable, and the 30m cluster usage chart with
 * allocatable as the threshold line.
 */
function CapacityUsage({ resource, r, samples }) {
  const used = percentOf(r.used, r.allocatable)
  const requested = percentOf(r.requested, r.allocatable)
  const chart = buildChartData(samples, resource.kind, r.allocatable)
  return (
    <div class="space-y-2 min-w-0">
      <div title={capacityTitle(r, resource.formatTitle)}>
        <ChartHeader
          label={resource.label}
          value={<span class={severityTextClass(percentSeverity(used))}>{used == null ? '-' : `${used}% used`}</span>}
          percent={requested != null && <span class="text-xs text-gray-500 dark:text-gray-400">{requested}% requested</span>}
          testId={`nodes-${resource.kind}-header`}
        />
        <span class="sr-only">{capacityTitle(r, resource.formatTitle)}</span>
      </div>
      <UsageChart
        data={chart.data}
        hasLimit={chart.hasLimit}
        limitLabel="allocatable"
        colorKey={resource.kind}
        formatValue={resource.format}
        tickIncrs={resource.kind === 'memory' ? BINARY_TICK_INCRS : undefined}
        testId={`nodes-${resource.kind}-chart`}
      />
    </div>
  )
}

/**
 * CapacityPanel - cluster capacity of the nodes that take workloads: CPU
 * and memory usage charts side by side, or request bars without
 * metrics-server. The subtitle is the pod slots in use.
 *
 * @param {Object} props
 * @param {Object} props.data - Snapshot from GET /api/v1/nodes
 */
export function CapacityPanel({ data }) {
  const capacity = data.capacity || {}
  const pods = capacity.pods || {}
  const samples = data.usage?.samples || []
  const charts = data.metricsAvailable && samples.length > 0
  // Resources with a known allocatable; with no eligible node there is nothing to show.
  const resources = RESOURCES.filter(res => percentOf(capacity[res.kind]?.requested, capacity[res.kind]?.allocatable) != null)

  return (
    <DashboardPanel
      title="Capacity"
      subtitle={<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{pods.running ?? 0} / {pods.allocatable ?? '-'} pods</p>}
    >
      {resources.length === 0 ? (
        <p class="text-sm text-gray-500 dark:text-gray-400" data-testid="nodes-capacity-empty">No allocatable capacity</p>
      ) : charts ? (
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          {resources.map(res => <CapacityUsage key={res.kind} resource={res} r={capacity[res.kind]} samples={samples} />)}
        </div>
      ) : (
        <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-3">
          {resources.map(res => {
            const r = capacity[res.kind]
            const requested = percentOf(r.requested, r.allocatable)
            return (
              <div key={res.kind} title={capacityTitle(r, res.formatTitle)} data-testid={`nodes-${res.kind}-requests`}>
                <ResourceMetric label={res.label} value={`${requested}% requested`} barPercent={requested} />
                <span class="sr-only">{capacityTitle(r, res.formatTitle)}</span>
              </div>
            )
          })}
        </div>
      )}
    </DashboardPanel>
  )
}
