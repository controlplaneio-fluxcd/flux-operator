// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { DashboardPanel } from '../common/panel'
import { UsageCharts } from './UsageCharts'
import { latestSample, usageAnnotation } from '../../../utils/metrics'

/**
 * WorkloadMetricsPanel - CPU and Memory usage charts for a workload,
 * with a rollout marker and per-pod usage bars trimmed to the fixed row
 * budget. Renders nothing until the first sample exists; the charts
 * show a collecting placeholder until a second sample is buffered.
 *
 * @param {Object} props
 * @param {Object} props.metrics - workloadInfo.metrics object with samples
 *   and the requests/limits summed across the running pods (0 = unset)
 * @param {Array} [props.pods] - workloadInfo.pods entries carrying the
 *   current per-pod usage in pod.metrics
 * @param {string} [props.rolledOutAt] - workloadInfo.rolledOutAt timestamp
 */
export function WorkloadMetricsPanel({ metrics, pods, rolledOutAt }) {
  if (!latestSample(metrics)) {
    return null
  }

  const rolledOut = usageAnnotation(metrics, rolledOutAt)
  const annotation = rolledOut !== null ? { time: rolledOut, label: 'rolled out' } : null

  return (
    <DashboardPanel title="Resource Usage" id="workload-metrics-panel">
      <UsageCharts metrics={metrics} items={pods} annotation={annotation} trim />
    </DashboardPanel>
  )
}
