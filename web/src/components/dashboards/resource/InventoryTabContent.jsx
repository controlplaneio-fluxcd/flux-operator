// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo } from 'preact/compat'
import { isFluxInventoryItem, isWorkloadInventoryItem } from '../../../utils/constants'
import { getDashboardUrl } from '../../../utils/routing'

/**
 * InventoryTabContent - Flat list of all Kubernetes objects managed by a Flux
 * resource. Owns its own sorting and link routing: Flux resources and workloads
 * link to their dashboards, other kinds render as plain text.
 *
 * @param {array} inventory - The resource's status.inventory items
 * @param {string} namespace - Default namespace for items without explicit namespace
 */
export function InventoryTabContent({ inventory = [], namespace }) {
  // Sort inventory items
  const sortedInventory = useMemo(() => {
    return [...inventory].sort((a, b) => {
      // Non-namespaced items first
      const aHasNamespace = !!a.namespace
      const bHasNamespace = !!b.namespace

      if (!aHasNamespace && bHasNamespace) return -1
      if (aHasNamespace && !bHasNamespace) return 1

      // Both non-namespaced: sort by kind, then name
      if (!aHasNamespace && !bHasNamespace) {
        if (a.kind !== b.kind) {
          return a.kind.localeCompare(b.kind)
        }
        return a.name.localeCompare(b.name)
      }

      // Both namespaced: sort by namespace, then kind, then name
      if (a.namespace !== b.namespace) {
        return a.namespace.localeCompare(b.namespace)
      }
      if (a.kind !== b.kind) {
        return a.kind.localeCompare(b.kind)
      }
      return a.name.localeCompare(b.name)
    })
  }, [inventory])

  // Build the dashboard URL for an inventory item, routing workloads to the
  // workload dashboard and Flux resources to the resource dashboard.
  const getItemUrl = (item) =>
    getDashboardUrl(item.kind, item.namespace || namespace, item.name)

  return (
    <div class="overflow-x-auto">
      <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
        <thead>
          <tr>
            <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Name</th>
            <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Namespace</th>
            <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Kind</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          {sortedInventory.map((item, idx) => {
            const isFluxResource = isFluxInventoryItem(item)
            const isWorkload = !isFluxResource && isWorkloadInventoryItem(item)
            return (
              <tr key={idx} class="hover:bg-gray-50 dark:hover:bg-gray-800">
                <td class="px-3 py-2 text-sm">
                  {(isFluxResource || isWorkload) ? (
                    <a
                      href={getItemUrl(item)}
                      class="text-flux-blue dark:text-blue-400 hover:underline"
                    >
                      {item.name}
                    </a>
                  ) : (
                    <span class="text-gray-900 dark:text-gray-100">{item.name}</span>
                  )}
                </td>
                <td class="px-3 py-2 text-sm text-gray-900 dark:text-gray-100">{item.namespace || '-'}</td>
                <td class="px-3 py-2 text-sm text-gray-900 dark:text-gray-100">{item.kind}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
