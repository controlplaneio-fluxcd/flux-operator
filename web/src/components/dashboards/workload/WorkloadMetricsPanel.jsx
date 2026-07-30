// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/hooks'
import { DashboardPanel } from '../common/panel'
import { UsageChart } from './UsageChart'
import { severityTextClass } from '../common/ResourceMetric'
import {
  formatCores, formatBytes, percentOf, limitSeverity,
  buildChartData, latestSample, hasChartableMetrics,
  podUsageSeries, trimPodUsage, usageAnnotation,
  BINARY_TICK_INCRS,
} from '../../../utils/metrics'

/**
 * HeaderPercent - Secondary percentage text for a chart header, with the
 * limit fragment colored by proximity to the limit. Renders nothing when
 * neither requests nor limits are set.
 */
function HeaderPercent({ value, requests, limits }) {
  const reqPct = percentOf(value, requests)
  const limPct = percentOf(value, limits)
  if (reqPct === null && limPct === null) return null
  return (
    <span class="text-xs text-gray-500 dark:text-gray-400">
      {reqPct !== null && `${reqPct}% of request`}
      {reqPct !== null && limPct !== null && ' · '}
      {limPct !== null && (
        <span class={severityTextClass(limitSeverity(value, limits))}>{limPct}% of limit</span>
      )}
    </span>
  )
}

/**
 * ChartHeader - Headline for one usage chart: metric name, current
 * absolute value and, when requests/limits are set, the usage percentage.
 */
function ChartHeader({ label, value, percent, testId }) {
  return (
    <div class="flex items-baseline justify-between flex-wrap gap-x-2" data-testid={testId}>
      <div class="flex items-baseline gap-2 min-w-0">
        <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">{label}</span>
        <span class="text-sm sm:text-base font-medium text-gray-900 dark:text-white">{value}</span>
      </div>
      {percent}
    </div>
  )
}

// Class names are spelled out per metric: Tailwind drops classes that
// never appear literally in the sources.
const BAR_CLASSES = {
  cpu: {
    track: 'usage-bar-track usage-bar-track-cpu w-20 sm:w-24',
    fill: 'usage-bar-fill usage-bar-fill-cpu',
  },
  memory: {
    track: 'usage-bar-track usage-bar-track-memory w-20 sm:w-24',
    fill: 'usage-bar-fill usage-bar-fill-memory',
  },
}

/**
 * PodUsageBars - Current usage of each pod as a horizontal bar, scaled
 * to the busiest pod. Pods without a usage sample yet show a gray bar
 * with an N/A value. Large workloads are trimmed to a fixed row budget
 * (trimPodUsage): the collapsed middle renders as a "+N pods" row with
 * a muted bar at the average and the value range in its tooltip.
 *
 * @param {Object} props
 * @param {Array<{name: string, value: number|null}>} props.items -
 *   Per-pod usage sorted descending (podUsageSeries output)
 * @param {string} props.colorKey - "cpu" or "memory", matches the chart hue
 * @param {Function} props.formatValue - Value formatter
 * @param {string} props.testId - data-testid for the bars container
 */
function PodUsageBars({ items, colorKey, formatValue, testId }) {
  const max = items[0]?.value || 0
  const barClasses = BAR_CLASSES[colorKey] || BAR_CLASSES.cpu
  return (
    <div class="space-y-1" data-testid={testId}>
      {trimPodUsage(items).map(row => {
        if (row.type === 'elision') {
          const rangeText = formatValue(row.min) === formatValue(row.max)
            ? `${formatValue(row.min)} each`
            : `${formatValue(row.min)} – ${formatValue(row.max)}`
          return (
            <div
              key="elision"
              class="flex items-center gap-2"
              data-testid={`${testId}-elision`}
              title={`${row.count} pods, ${rangeText}`}
            >
              <span class="flex-1 min-w-0 truncate text-xs text-gray-500 dark:text-gray-400">
                +{row.count} pods
              </span>
              <div class="usage-bar-track usage-bar-track-na w-20 sm:w-24">
                <div
                  class="usage-bar-fill usage-bar-fill-na"
                  style={{ width: `${max > 0 && row.avg > 0 ? Math.max((row.avg / max) * 100, 2) : 0}%` }}
                />
              </div>
              <span class="w-14 shrink-0 text-right text-xs tabular-nums text-gray-500 dark:text-gray-400">
                {formatValue(row.avg)}
              </span>
            </div>
          )
        }
        if (row.type === 'collecting') {
          return (
            <div key="collecting" class="flex items-center gap-2" data-testid={`${testId}-collecting`}>
              <span class="flex-1 min-w-0 truncate text-xs text-gray-500 dark:text-gray-400">
                collecting metrics for {row.count} pods
              </span>
              <div class="usage-bar-track usage-bar-track-na w-20 sm:w-24" />
              <span class="w-14 shrink-0 text-right text-xs text-gray-500 dark:text-gray-400">N/A</span>
            </div>
          )
        }
        const limitPct = percentOf(row.value, row.limit)
        const severity = limitSeverity(row.value, row.limit)
        const fillClass = severity === 'critical'
          ? 'usage-bar-fill usage-bar-fill-critical'
          : severity === 'warn'
            ? 'usage-bar-fill usage-bar-fill-warn'
            : barClasses.fill
        // Name the severity in text so it isn't conveyed by color alone.
        const limitText = limitPct === null
          ? null
          : `${limitPct}% of limit${severity === 'critical' ? ' (critical)' : severity === 'warn' ? ' (high)' : ''}`
        return (
          <div key={row.name} class="flex items-center gap-2" data-testid={`${testId}-row`}>
            <span
              class="flex-1 min-w-0 truncate text-xs text-gray-600 dark:text-gray-400"
              title={row.name}
            >
              {row.name}
            </span>
            {row.value === null ? (
              <div class="usage-bar-track usage-bar-track-na w-20 sm:w-24" />
            ) : (
              <div
                class={barClasses.track}
                title={limitText || undefined}
              >
                <div
                  class={fillClass}
                  style={{ width: `${max > 0 && row.value > 0 ? Math.max((row.value / max) * 100, 2) : 0}%` }}
                />
              </div>
            )}
            <span class="w-14 shrink-0 text-right text-xs tabular-nums text-gray-900 dark:text-white">
              {row.value === null
                ? <span class="text-gray-500 dark:text-gray-400">N/A</span>
                : formatValue(row.value)}
              {/* The limit percentage lives in the bar tooltip, which is
                  unavailable to screen readers; repeat it as hidden text. */}
              {limitText !== null && <span class="sr-only"> ({limitText})</span>}
            </span>
          </div>
        )
      })}
    </div>
  )
}

/**
 * ChartPlaceholder - Stand-in for a usage chart while only one sample
 * exists, holding the chart's footprint so the panel doesn't jump when
 * the chart appears.
 */
function ChartPlaceholder({ testId }) {
  return (
    <div
      role="status"
      class="h-[180px] flex items-center justify-center rounded border border-dashed border-gray-200 dark:border-gray-700 text-xs text-gray-500 dark:text-gray-400"
      data-testid={testId}
    >
      Collecting usage data…
    </div>
  )
}

/**
 * WorkloadMetricsPanel - CPU and Memory usage charts for a workload,
 * with a rollout marker and per-pod usage bars. Renders nothing until
 * the first sample exists; the charts show a collecting placeholder
 * until a second sample is buffered.
 *
 * @param {Object} props
 * @param {Object} props.metrics - workloadInfo.metrics object with samples
 *   and the requests/limits summed across the running pods (0 = unset)
 * @param {Array} [props.pods] - workloadInfo.pods entries carrying the
 *   current per-pod usage in pod.metrics
 * @param {string} [props.rolledOutAt] - workloadInfo.rolledOutAt timestamp
 */
export function WorkloadMetricsPanel({ metrics, pods, rolledOutAt }) {
  const chartable = hasChartableMetrics(metrics)

  const cpu = useMemo(
    () => (chartable ? buildChartData(metrics.samples, 'cpu', metrics.cpuLimits) : null),
    [metrics, chartable],
  )
  const memory = useMemo(
    () => (chartable ? buildChartData(metrics.samples, 'memory', metrics.memoryLimits) : null),
    [metrics, chartable],
  )

  const current = latestSample(metrics)
  if (!current) {
    return null
  }
  const cpuPercent = <HeaderPercent value={current.cpu} requests={metrics.cpuRequests} limits={metrics.cpuLimits} />
  const memoryPercent = <HeaderPercent value={current.memory} requests={metrics.memoryRequests} limits={metrics.memoryLimits} />

  const rolledOut = usageAnnotation(metrics, rolledOutAt)
  const annotation = rolledOut !== null ? { time: rolledOut, label: 'rolled out' } : null

  const cpuPods = podUsageSeries(pods, 'cpu')
  const memoryPods = podUsageSeries(pods, 'memory')

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
          {chartable ? (
            <UsageChart
              data={cpu.data}
              hasLimit={cpu.hasLimit}
              colorKey="cpu"
              formatValue={formatCores}
              annotation={annotation}
              testId="cpu-usage-chart"
            />
          ) : (
            <ChartPlaceholder testId="cpu-usage-chart-placeholder" />
          )}
          {cpuPods.length > 0 && (
            <PodUsageBars
              items={cpuPods}
              colorKey="cpu"
              formatValue={formatCores}
              testId="cpu-pod-bars"
            />
          )}
        </div>
        <div class="space-y-2 min-w-0">
          <ChartHeader
            label="Memory"
            value={formatBytes(current.memory)}
            percent={memoryPercent}
            testId="memory-usage-header"
          />
          {chartable ? (
            <UsageChart
              data={memory.data}
              hasLimit={memory.hasLimit}
              colorKey="memory"
              formatValue={formatBytes}
              tickIncrs={BINARY_TICK_INCRS}
              annotation={annotation}
              testId="memory-usage-chart"
            />
          ) : (
            <ChartPlaceholder testId="memory-usage-chart-placeholder" />
          )}
          {memoryPods.length > 0 && (
            <PodUsageBars
              items={memoryPods}
              colorKey="memory"
              formatValue={formatBytes}
              testId="memory-pod-bars"
            />
          )}
        </div>
      </div>
    </DashboardPanel>
  )
}
