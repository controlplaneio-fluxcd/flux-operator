// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/hooks'
import { DashboardPanel } from '../common/panel'
import { UsageChart } from './UsageChart'
import {
  formatCores, formatBytes, percentText,
  buildChartData, latestSample, hasChartableMetrics,
  BINARY_TICK_INCRS,
} from '../../../utils/metrics'

/**
 * ChartHeader - Headline for one usage chart: metric name, current
 * absolute value and, when requests/limits are set, the usage percentage.
 * The percentage is omitted entirely when no denominator exists.
 */
function ChartHeader({ label, value, percent, testId }) {
  return (
    <div class="flex items-baseline justify-between flex-wrap gap-x-2" data-testid={testId}>
      <div class="flex items-baseline gap-2 min-w-0">
        <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">{label}</span>
        <span class="text-sm sm:text-base font-medium text-gray-900 dark:text-white">{value}</span>
      </div>
      {percent && (
        <span class="text-xs text-gray-500 dark:text-gray-400">{percent}</span>
      )}
    </div>
  )
}

/**
 * WorkloadMetricsPanel - CPU and Memory usage charts for a workload,
 * fed by the ~30 minute usage window aggregated across the workload pods
 * (workloadInfo.metrics). Renders nothing when the Metrics API is
 * unavailable or fewer than two samples exist.
 *
 * @param {Object} props
 * @param {Object} props.metrics - workloadInfo.metrics object with samples
 *   and the requests/limits summed across the running pods (0 = unset)
 */
export function WorkloadMetricsPanel({ metrics }) {
  const chartable = hasChartableMetrics(metrics)

  const cpu = useMemo(
    () => (chartable ? buildChartData(metrics.samples, 'cpu', metrics.cpuLimits) : null),
    [metrics, chartable],
  )
  const memory = useMemo(
    () => (chartable ? buildChartData(metrics.samples, 'memory', metrics.memoryLimits) : null),
    [metrics, chartable],
  )

  if (!chartable) {
    return null
  }

  const current = latestSample(metrics)
  const cpuPercent = percentText(current.cpu, metrics.cpuRequests, metrics.cpuLimits)
  const memoryPercent = percentText(current.memory, metrics.memoryRequests, metrics.memoryLimits)

  return (
    <DashboardPanel title="Resource Usage" id="workload-metrics-panel">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6" data-testid="workload-metrics">
        <div class="space-y-2 min-w-0">
          <ChartHeader
            label="CPU"
            value={formatCores(current.cpu)}
            percent={cpuPercent}
            testId="cpu-usage-header"
          />
          <UsageChart
            data={cpu.data}
            hasLimit={cpu.hasLimit}
            colorKey="cpu"
            formatValue={formatCores}
            testId="cpu-usage-chart"
          />
        </div>
        <div class="space-y-2 min-w-0">
          <ChartHeader
            label="Memory"
            value={formatBytes(current.memory)}
            percent={memoryPercent}
            testId="memory-usage-header"
          />
          <UsageChart
            data={memory.data}
            hasLimit={memory.hasLimit}
            colorKey="memory"
            formatValue={formatBytes}
            tickIncrs={BINARY_TICK_INCRS}
            testId="memory-usage-chart"
          />
        </div>
      </div>
    </DashboardPanel>
  )
}
