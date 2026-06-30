// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { getWorkloadStatusBadgeClass, formatWorkloadStatus } from '../../utils/status'

const PILL_BASE = 'inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium'

/**
 * StatusPill - the workload-status badge shown across the resource dashboard tabs:
 * a colored pill with the formatted status, or a neutral pulsing "computing…"
 * placeholder while the live status is still being fetched. Shared by the
 * Inventory and Workloads tabs so the pill markup/palette stays in one place.
 *
 * @param {Object} props
 * @param {string} [props.status] - kstatus/workload status word; placeholder when falsy
 * @param {string} props.computingTestid - data-testid for the placeholder pill
 */
export function StatusPill({ status, computingTestid }) {
  if (!status) {
    return (
      <span class={`${PILL_BASE} bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400 animate-pulse`} data-testid={computingTestid}>
        computing…
      </span>
    )
  }
  return (
    <span class={`${PILL_BASE} ${getWorkloadStatusBadgeClass(status)}`}>
      {formatWorkloadStatus(status)}
    </span>
  )
}
