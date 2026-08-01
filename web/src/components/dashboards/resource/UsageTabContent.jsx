// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/hooks'
import { UsageCharts } from '../workload/UsageCharts'
import { sumSeries, usageAnnotation } from '../../../utils/metrics'

/**
 * UsageTabContent - Resource Usage tab of the Managed Objects panel:
 * CPU and Memory charts aggregated across the workloads managed by the
 * resource, with one navigable bar row per workload underneath. The
 * workload statuses come from the panel-owned batch fetch; entries the
 * user cannot read (NotFound sentinel) are excluded. The last
 * reconciliation is drawn as a marker when it falls inside the sampled
 * window.
 *
 * @param {Object} props
 * @param {Object} props.resourceData - The Flux resource, used for the
 *   reconciled annotation from status.history
 * @param {Array|null} props.workloads - Workload statuses with samples
 *   from POST /api/v1/workloads; null until the first fetch resolves
 * @param {boolean} [props.error] - Whether the last fetch failed; shown
 *   instead of the loading state when no data has arrived yet
 */
export function UsageTabContent({ resourceData, workloads, error }) {
  // Workloads the user can read; the NotFound sentinel covers both
  // missing and forbidden workloads, neither of which can ever chart.
  const rows = useMemo(
    () => (workloads || []).filter(w => w.status !== 'NotFound'),
    [workloads],
  )

  const metrics = useMemo(
    () => ({ samples: sumSeries(rows.map(w => w.samples)) }),
    [rows],
  )

  // Bar rows carry the workload identity for keying and navigation and
  // the latest sample as the current usage; workloads without samples
  // yet render as N/A rows.
  const items = useMemo(() => rows.map(w => {
    const samples = Array.isArray(w.samples) ? w.samples : []
    const last = samples.length > 0 ? samples[samples.length - 1] : null
    return {
      kind: w.kind,
      namespace: w.namespace,
      name: w.name,
      metrics: last ? { cpu: last.cpu, memory: last.memory } : undefined,
    }
  }), [rows])

  // The resource-level analogue of the workload rollout marker: the last
  // apply from status.history (lastDeployed for HelmRelease).
  const entry = resourceData?.status?.history?.[0]
  const reconciledAt = resourceData?.kind === 'HelmRelease' ? entry?.lastDeployed : entry?.lastReconciled
  const reconciled = usageAnnotation(metrics, reconciledAt)
  const annotation = reconciled !== null ? { time: reconciled, label: 'reconciled' } : null

  if (workloads === null) {
    // A fetch failure with data from an earlier fetch keeps showing it;
    // without any data the error replaces the loading state, which would
    // otherwise sit there forever.
    if (error) {
      return (
        <div
          class="py-8 text-center text-sm text-gray-500 dark:text-gray-400"
          data-testid="usage-tab-error"
        >
          Failed to load usage data. Retrying on the next refresh.
        </div>
      )
    }
    return (
      <div
        role="status"
        class="py-8 text-center text-sm text-gray-500 dark:text-gray-400"
        data-testid="usage-tab-loading"
      >
        Loading usage data…
      </div>
    )
  }

  if (metrics.samples.length === 0) {
    return (
      <div
        class="py-8 text-center text-sm text-gray-500 dark:text-gray-400"
        data-testid="usage-tab-empty"
      >
        No usage data available. Workload metrics require the Kubernetes Metrics API.
      </div>
    )
  }

  return <UsageCharts metrics={metrics} items={items} annotation={annotation} />
}
