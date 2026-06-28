// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0
//
// Toolbar wrapper for the compact list view: a result count, a mobile-only
// "Filters" toggle that collapses the form, the FilterForm in a collapsible
// container, and an optional status chart shown only on desktop.

import { useState } from 'preact/hooks'
import { Spinner } from './compactRow'

/**
 * FilterBar - presentational toolbar that wraps a FilterForm. On mobile it
 * shows the result count beside a funnel toggle and hides the form until the
 * toggle is pressed; on desktop the form (and the status chart) is always
 * visible. The filter signals stay in the FilterForm passed as `children`.
 *
 * @param {Object} props
 * @param {number} props.count - Number of results, shown before `label`
 * @param {string} props.label - Plural noun for the count (e.g. "resources")
 * @param {boolean} [props.loading] - When true, shows a spinner + "Loading…"
 *   instead of the (still-zero) result count
 * @param {*} props.children - The FilterForm element
 * @param {*} [props.statusChart] - Status chart node; FilterBar hides it on
 *   mobile (it stays in the desktop toolbar only)
 */
export function FilterBar({ count, label, loading, children, statusChart }) {
  // Mobile: the filter form is collapsed behind a toggle to save vertical space.
  const [showFilters, setShowFilters] = useState(false)

  return (
    <div class="card px-3 py-2.5">
      {/* Mobile: a single row with the count (or loader) + a filter toggle. The
          full filter form (and status chart) opens on demand. */}
      <div class="sm:hidden flex items-center justify-between">
        <span class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
          {loading ? (
            <>
              <Spinner cls="w-4 h-4" />
              Loading…
            </>
          ) : (
            <>{count} {label}</>
          )}
        </span>
        <button
          onClick={() => setShowFilters(v => !v)}
          aria-label="Toggle filters"
          aria-expanded={showFilters}
          aria-controls="filter-panel"
          class={`inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-sm hover:text-flux-blue ${showFilters ? 'text-flux-blue' : 'text-gray-500 dark:text-gray-400'}`}
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4h18M6 9h12M10 14h4M12 19h0" /></svg>
          Filters
        </button>
      </div>
      {/* Filters + status chart: always shown on desktop, toggled on mobile. The
          top margin only spaces this below the mobile bar; on desktop the bar is
          hidden, so the margin collapses and the card padding stays symmetric. */}
      <div id="filter-panel" class={`${showFilters ? 'block' : 'hidden'} sm:block mt-2.5 sm:mt-0 space-y-2.5`}>
        {children}
        {/* Status chart is desktop-only — kept out of the mobile filter panel even
            when it is expanded. */}
        {statusChart && <div class="hidden sm:block">{statusChart}</div>}
      </div>
    </div>
  )
}
