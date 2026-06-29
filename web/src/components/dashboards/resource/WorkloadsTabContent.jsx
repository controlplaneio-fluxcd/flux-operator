// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { formatScheduleMessage } from '../../../utils/cron'
import { getDashboardUrl } from '../../../utils/routing'
import { StatusPill } from '../../common/StatusPill'

/**
 * WorkloadsTabContent - Launchpad list of the Kubernetes workloads managed by a
 * Flux resource. Each row shows the workload's aggregate status and links to the
 * dedicated workload dashboard, where pods, container images, logs, events and
 * actions live.
 *
 * Status (status + statusMessage only — not pods, images, or actions) is fetched
 * and owned by ManagedObjectsPanel and passed in via workloadStatuses, so it is shared
 * with the Graph tab and survives tab switches without refetching.
 *
 * @param {array} workloadItems - Inventory items of workload kinds
 * @param {string} namespace - Default namespace for items without explicit namespace
 * @param {object} workloadStatuses - Map of workload key to {status, statusMessage}
 */
export function WorkloadsTabContent({ workloadItems, namespace, workloadStatuses = {} }) {
  // Render the workload list. Rows render immediately from props; the status
  // badge and message fill in once statuses arrive (no blocking loader).
  return (
    <div class="space-y-4">
      {workloadItems.map((item) => {
        const resolvedNamespace = item.namespace || namespace
        const key = `${item.kind}/${resolvedNamespace}/${item.name}`
        const status = workloadStatuses[key]

        return (
          <a
            key={key}
            href={getDashboardUrl(item.kind, resolvedNamespace, item.name)}
            class="group flex items-center gap-2 border border-gray-200 dark:border-gray-700 rounded-md px-4 py-3 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
          >
            <div class="flex-grow min-w-0">
              {/* Line 1: KIND (uppercase) with status badge */}
              <div class="flex items-center gap-3 mb-1">
                <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase">
                  {item.kind}
                </span>
                {/* Colored badge once live status arrives, else a pulsing
                    placeholder so the row keeps its shape from first paint. */}
                <StatusPill status={status?.status} computingTestid="workload-status-computing" />
              </div>
              {/* Line 2: Namespace/Name */}
              <div class="text-sm truncate">
                <span class="text-gray-500 dark:text-gray-400">{resolvedNamespace}/</span>
                <span class="font-semibold text-gray-900 dark:text-gray-100">{item.name}</span>
              </div>
              {/* Line 3: StatusMessage */}
              {status && status.statusMessage && (
                <div class="text-sm text-gray-700 dark:text-gray-300 mt-1 break-all">
                  {formatScheduleMessage(status.statusMessage)}
                </div>
              )}
            </div>
            {/* Navigation chevron on the right */}
            <svg class="w-4 h-4 text-gray-400 group-hover:text-flux-blue dark:group-hover:text-blue-400 transition-colors flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
          </a>
        )
      })}
    </div>
  )
}
