// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { useState } from 'preact/hooks'
import { fluxCRDs, eventSeverities, resourceStatuses } from '../../utils/constants'

// Derive unique groups from fluxCRDs array (preserving order of first occurrence)
const crdGroups = [...new Set(fluxCRDs.map(crd => crd.group))]

// Compact field styling shared by the name input and the dropdowns. Selects get
// their chevron + right padding from the global `select` rule in index.css.
const FIELD = 'w-full px-2 py-1 text-xs border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-flux-blue'

// Compact icon button styling for the clear/refresh actions.
const ICON_BTN = 'inline-flex items-center p-1 rounded-md text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white focus:outline-none transition-colors'

/**
 * FilterForm component - Reusable filter form for Events and Resources
 *
 * @param {Object} props
 * @param {Signal} props.kindSignal - Signal for selected kind
 * @param {Signal} props.nameSignal - Signal for selected name
 * @param {Signal} props.namespaceSignal - Signal for selected namespace
 * @param {Array<string>} props.namespaces - Array of namespace names from report
 * @param {Signal} [props.severitySignal] - Optional signal for event severity filter (Normal, Warning)
 * @param {Signal} [props.statusSignal] - Optional signal for resource status filter (Ready, Failed, etc.)
 * @param {Array<string>} [props.kinds] - Optional flat list of kinds. When provided, the kind
 *   dropdown renders a flat list instead of the Flux group-based optgroup layout.
 * @param {Function} props.onClear - Callback function to clear filters
 * @param {Function} [props.onRefresh] - Optional async callback to re-fetch the list. When provided,
 *   a refresh button is rendered after clear and spins while the refresh is in flight.
 */
export function FilterForm({ kindSignal, nameSignal, namespaceSignal, namespaces, severitySignal, statusSignal, kinds, onClear, onRefresh }) {
  const [refreshing, setRefreshing] = useState(false)

  const handleRefresh = async () => {
    if (!onRefresh || refreshing) return
    setRefreshing(true)
    try {
      await onRefresh()
    } finally {
      setRefreshing(false)
    }
  }

  return (
    // Mobile: a two-row stack (name + status/severity + clear, then namespace +
    // kind + refresh). Each row wrapper collapses on desktop via `sm:contents`,
    // so the field divs become direct children of the wrapping flex line again
    // and `sm:order` restores the original single-line layout.
    <div class="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-2">
      {/* Mobile row 1: name + status/severity + clear */}
      <div class="flex items-center gap-2 sm:contents">
        {/* Name Filter — each field is wrapped in a flex-1 div (the control fills
            it via w-full), so all fields distribute equally on both mobile rows;
            the name keeps a slightly larger desktop min-width. */}
        <div class="flex-1 min-w-0 sm:min-w-[200px] sm:order-1">
          <input
            id="filter-name"
            name="name"
            type="text"
            value={nameSignal.value}
            onChange={(e) => nameSignal.value = e.target.value}
            placeholder="Resource name (* wildcard, ! exclude)"
            class={FIELD}
          />
        </div>

        {/* Severity Filter (Events only) */}
        {severitySignal && (
          <div class="flex-1 min-w-0 sm:min-w-[170px] sm:order-4">
            <select
              id="filter-severity"
              name="severity"
              value={severitySignal.value}
              onChange={(e) => severitySignal.value = e.target.value}
              class={FIELD}
            >
              <option value="">All severities</option>
              {eventSeverities.map(severity => (
                <option key={severity} value={severity}>{severity}</option>
              ))}
            </select>
          </div>
        )}

        {/* Status Filter (Resources only) */}
        {statusSignal && (
          <div class="flex-1 min-w-0 sm:min-w-[170px] sm:order-4">
            <select
              id="filter-status"
              name="status"
              value={statusSignal.value}
              onChange={(e) => statusSignal.value = e.target.value}
              class={FIELD}
            >
              <option value="">All statuses</option>
              {resourceStatuses.map(status => (
                <option key={status} value={status}>{status}</option>
              ))}
            </select>
          </div>
        )}

        {/* Clear Filters Button */}
        <div class="sm:order-5">
          <button
            onClick={onClear}
            title="Clear"
            aria-label="Clear filters"
            class={ICON_BTN}
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      {/* Mobile row 2: namespace + kind + refresh */}
      <div class="flex items-center gap-2 sm:contents">
        {/* Namespace Filter */}
        <div class="flex-1 min-w-0 sm:min-w-[170px] sm:order-2">
          <select
            id="filter-namespace"
            name="namespace"
            value={namespaceSignal.value}
            onChange={(e) => namespaceSignal.value = e.target.value}
            class={FIELD}
          >
            <option value="">All namespaces</option>
            {(namespaces || []).map(ns => (
              <option key={ns} value={ns}>{ns}</option>
            ))}
          </select>
        </div>

        {/* Kind Filter */}
        <div class="flex-1 min-w-0 sm:min-w-[170px] sm:order-3">
          <select
            id="filter-kind"
            name="kind"
            value={kindSignal.value}
            onChange={(e) => kindSignal.value = e.target.value}
            class={FIELD}
          >
            <option value="">All kinds</option>
            {kinds
              ? kinds.map(kind => (
                <option key={kind} value={kind}>{kind}</option>
              ))
              : crdGroups.map(group => (
                <optgroup key={group} label={group}>
                  {fluxCRDs.filter(crd => crd.group === group).map(crd => (
                    <option key={crd.kind} value={crd.kind}>{crd.kind}</option>
                  ))}
                </optgroup>
              ))}
          </select>
        </div>

        {/* Refresh Button (re-fetches the list) */}
        {onRefresh && (
          <div class="sm:order-6">
            <button
              onClick={handleRefresh}
              disabled={refreshing}
              title="Refresh"
              aria-label="Refresh"
              class={`${ICON_BTN} disabled:opacity-60`}
            >
              <svg class={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
