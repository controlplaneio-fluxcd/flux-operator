// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { formatScheduleMessage } from '../../../utils/cron'
import { isFluxInventoryItem, isWorkloadInventoryItem } from '../../../utils/constants'
import { getDashboardUrl } from '../../../utils/routing'
import { NameSpans, Chevron, Spinner, useDisclosure, Reveal } from '../../common/rowKit'
import { StatusPill } from '../../common/StatusPill'
import { ObjectDetailsView } from './ObjectDetailsView'

/**
 * InventoryRow - Compact, expandable row for one managed object. Mirrors the
 * search WorkloadRow layout minus the favorite star, kind chip, "managed by" line,
 * and timestamp.
 *
 * Every row is a two-line card at all widths: the first line carries the
 * namespace/name (left, the object's identity — given the most room and truncated
 * only when it runs out), the status pill, and a trailing affordance (right); the
 * second line carries the kind and the status message (the message is hidden on
 * phones to keep the row compact). This keeps the name from ever competing with
 * fixed kind/pill columns, so it reads the same on a phone and a wide monitor.
 *
 * The trailing affordance depends on whether the kind has a dashboard: Flux and
 * workload kinds show a link icon that navigates to their dashboard and never
 * expand; every other kind shows an expand chevron that lazily mounts
 * {@link ObjectDetailsView} to reveal the object's manifest inline.
 *
 * @param {Object} props
 * @param {Object} props.item - Inventory item with apiVersion, kind, name, namespace
 * @param {{status: string, statusMessage?: string}} [props.status] - Live status
 *   from the tab-owned status map; undefined until the first status fetch resolves
 * @param {*} [props.refreshKey] - Changes once per parent poll (the inventory array
 *   reference); forwarded to the detail view so an open panel refetches in the
 *   background
 */
export function InventoryRow({ item, status, refreshKey }) {
  // Disclosure state: the chevron spins while the lazily mounted detail panel
  // fetches, then the panel animates open via Reveal.
  const d = useDisclosure()

  // An empty namespace means a cluster-scoped object: render the name only, no link
  // prefix. Namespaced items always carry their namespace explicitly in inventory.
  const namespace = item.namespace || ''
  const linkable = isFluxInventoryItem(item) || isWorkloadInventoryItem(item)
  const href = linkable ? getDashboardUrl(item.kind, namespace, item.name) : undefined

  // The pill is driven solely by the tab's batch status map (the `status` prop),
  // which refreshes every poll for every item — including a sentinel for objects
  // the user cannot read — so it never freezes or goes stale after a collapse.
  const statusWord = status?.status
  const statusMessage = status?.statusMessage

  // Two-line row body, shared by the link and button wrappers below. `trailing` is
  // the right-hand affordance (a link icon or the expand chevron). The name fills
  // the first line and clips when too long; it tints on row hover via the wrapper's
  // `group` class only when the row is a link.
  const body = (trailing) => (
    <>
      {/* Line 1: name (fills) + status pill + trailing affordance. */}
      <div class="flex items-center gap-2.5">
        <span class="block flex-1 min-w-0 truncate text-sm">
          <NameSpans namespace={namespace} name={item.name} linked={linkable} />
        </span>
        <div class="shrink-0">
          <StatusPill status={statusWord} computingTestid="inventory-status-computing" />
        </div>
        {trailing}
      </div>
      {/* Line 2: kind · status message (message + separator hidden on phones). */}
      <div class="mt-0.5 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span class="shrink-0 truncate" title={item.kind}>{item.kind}</span>
        {statusMessage && <span class="hidden sm:block shrink-0 text-gray-300 dark:text-gray-600" aria-hidden="true">·</span>}
        <span class="hidden sm:block flex-1 min-w-0 truncate" title={statusMessage || ''}>
          {statusMessage ? formatScheduleMessage(statusMessage) : ''}
        </span>
      </div>
    </>
  )

  // Trailing affordance, tinting on row hover via the wrapper's `group` class.
  const iconCls = 'shrink-0 p-0.5 text-gray-400 group-hover:text-flux-blue'

  // Dashboard-backed kinds (Flux/workload): the whole row is a link with a link
  // icon; clicking anywhere navigates to the dashboard and the row never expands.
  if (linkable) {
    return (
      <div class="border-b border-gray-100 dark:border-gray-700/60 last:border-0">
        <a href={href} class="group block px-3 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/30">
          {body(
            <span class={iconCls} aria-hidden="true">
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5h5m0 0v5m0-5L10 14M9 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-3" />
              </svg>
            </span>
          )}
        </a>
      </div>
    )
  }

  // Other kinds (no dashboard): the whole row is a button with an expand chevron;
  // clicking anywhere toggles the inline manifest, lazily mounting the detail view.
  return (
    <div class="border-b border-gray-100 dark:border-gray-700/60 last:border-0">
      <button type="button" onClick={d.toggle} aria-expanded={d.open} class="group block w-full text-left px-3 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/30">
        {body(
          <span class={iconCls} aria-hidden="true">
            {d.loading ? <Spinner /> : <Chevron open={d.open} />}
          </span>
        )}
      </button>
      <Reveal open={d.open}>
        <div class="px-3 pt-1 pb-4">
          {/* Lazily mounted on expand; unmounted on collapse so each expand
              re-fetches. onReady flips the disclosure once the fetch settles. */}
          {d.mounted && (
            <ObjectDetailsView
              apiVersion={item.apiVersion}
              kind={item.kind}
              namespace={namespace}
              name={item.name}
              isExpanded
              refreshKey={refreshKey}
              onReady={d.onReady}
            />
          )}
        </div>
      </Reveal>
    </div>
  )
}
