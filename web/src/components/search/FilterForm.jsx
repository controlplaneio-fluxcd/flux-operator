// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { fluxCRDs, eventSeverities, resourceStatuses } from '../../utils/constants'

// Derive unique groups from fluxCRDs array (preserving order of first occurrence)
const crdGroups = [...new Set(fluxCRDs.map(crd => crd.group))]

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
 * @param {Function} props.onClear - Callback function to clear filters
 */
export function FilterForm({ kindSignal, nameSignal, namespaceSignal, namespaces, severitySignal, statusSignal, onClear }) {
  return (
    <div class="flex flex-wrap gap-4 items-center">
      {/* Name Filter */}
      <div class="flex-1 min-w-[200px]">
        <input
          id="filter-name"
          name="name"
          type="text"
          value={nameSignal.value}
          onChange={(e) => nameSignal.value = e.target.value}
          placeholder="Resource name (* for wildcard)"
          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-flux-blue"
        />
      </div>

      {/* Namespace Filter */}
      <div class="flex-1 min-w-[200px]">
        <select
          id="filter-namespace"
          name="namespace"
          value={namespaceSignal.value}
          onChange={(e) => namespaceSignal.value = e.target.value}
          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-flux-blue"
        >
          <option value="">All namespaces</option>
          {(namespaces || []).map(ns => (
            <option key={ns} value={ns}>{ns}</option>
          ))}
        </select>
      </div>

      {/* Kind Filter */}
      <div class="flex-1 min-w-[200px]">
        <select
          id="filter-kind"
          name="kind"
          value={kindSignal.value}
          onChange={(e) => kindSignal.value = e.target.value}
          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-flux-blue"
        >
          <option value="">All kinds</option>
          {crdGroups.map(group => (
            <optgroup key={group} label={group}>
              {fluxCRDs.filter(crd => crd.group === group).map(crd => (
                <option key={crd.kind} value={crd.kind}>{crd.kind}</option>
              ))}
            </optgroup>
          ))}
        </select>
      </div>

      {/* Severity Filter (Events only) */}
      {severitySignal && (
        <div class="flex-1 min-w-[200px]">
          <select
            id="filter-severity"
            name="severity"
            value={severitySignal.value}
            onChange={(e) => severitySignal.value = e.target.value}
            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-flux-blue"
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
        <div class="flex-1 min-w-[200px]">
          <select
            id="filter-status"
            name="status"
            value={statusSignal.value}
            onChange={(e) => statusSignal.value = e.target.value}
            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-flux-blue"
          >
            <option value="">All statuses</option>
            {resourceStatuses.map(status => (
              <option key={status} value={status}>{status}</option>
            ))}
          </select>
        </div>
      )}

      {/* Clear Filters Button */}
      <div>
        <button
          onClick={onClear}
          title="Clear"
          aria-label="Clear filters"
          class="p-2 text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white focus:outline-none transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  )
}
