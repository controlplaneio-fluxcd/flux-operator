// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useMemo, useState, useEffect } from 'preact/hooks'
import { fetchWithMock } from '../../../utils/fetch'
import { isFluxInventoryItem, isWorkloadInventoryItem } from '../../../utils/constants'
import { compileSearch } from '../../../utils/inventorySearch'
import { ToggleGroup } from '../../common/ToggleGroup'
import { InventoryRow } from './InventoryRow'

// Category segmented control options. "Other" is everything that is neither a Flux
// nor a workload kind (Graph's "Resources" bucket), so the three specific
// categories are mutually exclusive.
const CATEGORIES = [
  { value: 'all', label: 'All', testid: 'inventory-cat-all' },
  { value: 'flux', label: 'Flux', testid: 'inventory-cat-flux' },
  { value: 'workloads', label: 'Workload', testid: 'inventory-cat-workloads' },
  { value: 'other', label: 'Other', testid: 'inventory-cat-other' }
]

// categoryMatch reports whether an item belongs to the selected category.
function categoryMatch(category, item) {
  switch (category) {
  case 'flux':
    return isFluxInventoryItem(item)
  case 'workloads':
    return isWorkloadInventoryItem(item)
  case 'other':
    return !isFluxInventoryItem(item) && !isWorkloadInventoryItem(item)
  default:
    return true
  }
}

// Stable empty reference so an undefined `inventory` prop does not produce a fresh
// array each render (which would churn the inventory-keyed effects below).
const EMPTY_INVENTORY = []

// statusKey is the stable identity used for both row keys and the status map,
// matching the key the batch status fetch builds per item.
function statusKey(item) {
  return `${item.apiVersion}/${item.kind}/${item.namespace || ''}/${item.name}`
}

// sortInventory orders items: cluster-scoped first, then by namespace, kind, name.
function sortInventory(items) {
  return [...items].sort((a, b) => {
    const aHasNs = !!a.namespace
    const bHasNs = !!b.namespace
    if (!aHasNs && bHasNs) return -1
    if (aHasNs && !bHasNs) return 1
    if (aHasNs && a.namespace !== b.namespace) return a.namespace.localeCompare(b.namespace)
    if (a.kind !== b.kind) return a.kind.localeCompare(b.kind)
    return a.name.localeCompare(b.name)
  })
}

/**
 * InventoryTabContent - Filterable list of the Kubernetes objects managed by a Flux
 * resource. Owns the category filter and search query (local state), the filtered
 * and sorted list, and the batch status fetch that feeds each row's live status
 * pill. Rows render immediately from the inventory prop; only the status pills are
 * async, mirroring the Workloads tab (no blocking loader).
 *
 * @param {array} inventory - The resource's status.inventory items (each carries
 *   its own namespace, empty for cluster-scoped objects)
 */
export function InventoryTabContent({ inventory }) {
  const items = inventory || EMPTY_INVENTORY
  const [category, setCategory] = useState('all')
  const [query, setQuery] = useState('')

  // Live status per object, keyed by statusKey. Fetched in one batch over the whole
  // inventory and refreshed whenever the inventory array changes (the parent poll
  // pushes a fresh array), so badges update without a dedicated polling loop.
  const [statuses, setStatuses] = useState({})
  useEffect(() => {
    if (items.length === 0) return
    let cancelled = false

    const fetchStatuses = async () => {
      try {
        const objects = items.map(item => ({
          apiVersion: item.apiVersion,
          kind: item.kind,
          namespace: item.namespace || '',
          name: item.name
        }))
        const data = await fetchWithMock({
          endpoint: '/api/v1/inventory/objects',
          mockPath: '../mock/inventory',
          mockExport: 'getMockInventoryObjects',
          method: 'POST',
          body: { objects }
        })
        const next = {}
        for (const o of (data?.objects || [])) {
          // Error items (Forbidden/NotFound) still get a map entry so their pill
          // shows the error instead of pulsing "computing…" forever — inventory can
          // legitimately list objects the impersonated user cannot read.
          next[statusKey(o)] = o.error
            ? { status: o.error }
            : { status: o.status, statusMessage: o.statusMessage }
        }
        if (!cancelled) setStatuses(next)
      } catch (err) {
        console.error('Failed to fetch inventory statuses:', err)
      }
    }

    fetchStatuses()
    return () => { cancelled = true }
  }, [items])

  // Apply the category filter, then the search matcher, then sort. Filter state is
  // independent of the inventory array, so polls never reset it.
  const visible = useMemo(() => {
    const matchesSearch = compileSearch(query)
    const filtered = items.filter(item => categoryMatch(category, item) && matchesSearch(item))
    return sortInventory(filtered)
  }, [items, category, query])

  const hasFilters = category !== 'all' || query !== ''
  const clearFilters = () => { setCategory('all'); setQuery('') }

  return (
    <div class="space-y-3">
      {/* Toolbar: category segmented control + search box + always-shown clear. */}
      <div class="flex flex-col sm:flex-row sm:items-center gap-2">
        <ToggleGroup
          ariaLabel="Filter by category"
          options={CATEGORIES}
          value={category}
          onChange={setCategory}
          testid="inventory-category"
        />
        <div class="flex-1 flex items-center gap-2">
          <div class="relative flex-1 min-w-0">
            <svg class="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              type="text"
              value={query}
              onInput={(e) => setQuery(e.currentTarget.value)}
              placeholder="Search by name, namespace, kind or API"
              aria-label="Search objects"
              data-testid="inventory-search"
              class="w-full pl-8 pr-2 py-1 text-xs border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-flux-blue"
            />
          </div>
          {/* Clear all filters: same icon button as the search pages' FilterForm. */}
          <button
            onClick={clearFilters}
            title="Clear"
            aria-label="Clear filters"
            data-testid="inventory-clear"
            class="inline-flex items-center p-1 rounded-md text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white focus:outline-none transition-colors"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      {/* Empty state when the filters exclude everything. */}
      {visible.length === 0 ? (
        <div class="py-10 text-center text-sm text-gray-500 dark:text-gray-400" data-testid="inventory-empty">
          {hasFilters ? 'No objects match the filters' : 'No managed objects'}
        </div>
      ) : (
        <div class="card overflow-hidden p-0 sm:p-2">
          {visible.map((item) => {
            // The inventory array reference changes once per parent poll, so it
            // doubles as the refetch signal for any open detail panel.
            const k = statusKey(item)
            return <InventoryRow key={k} item={item} status={statuses[k]} refreshKey={items} />
          })}
        </div>
      )}
    </div>
  )
}
